package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	ctxmgr "github.com/LennyFace24/MiniAgent/internal/context"
	"github.com/LennyFace24/MiniAgent/internal/modelmgr"
	"github.com/LennyFace24/MiniAgent/internal/permission"
	"github.com/LennyFace24/MiniAgent/internal/pipeline"
	"github.com/LennyFace24/MiniAgent/internal/skills"
	"github.com/LennyFace24/MiniAgent/internal/store"
	"github.com/LennyFace24/MiniAgent/internal/tools"
	contexttool "github.com/LennyFace24/MiniAgent/internal/tools/context_tool"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

// ToolEvent 通过 SSE 推送给前端的工具事件
type ToolEvent struct {
	Type      string `json:"type"` // "tool_call" | "tool_result" | "permission_request" | "thinking"
	Name      string `json:"name"` // 工具名称
	Args      string `json:"args,omitempty"`
	Result    string `json:"result,omitempty"`
	CallID    string `json:"call_id,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Content   string `json:"content,omitempty"` // 思考内容
}

// AgentService 统一的 Agent 服务，根据 mode 选择不同的 prompt 和行为
type AgentService struct {
	store          *store.ConversationStore
	models         *modelmgr.ModelSet // 本服务特有的模型集合（每服务一个实例，非全局）
	tools          []tool.BaseTool
	compactTrigger *contexttool.CompactTrigger
	perms          *permission.PermissionManager
	skillReg       *skills.SkillRegistry
}

// NewAgentService 创建 AgentService，注入服务级 ModelSet（registry 由 main 从 model.json 加载）。
func NewAgentService(toolHandler *tools.ToolHandler, convStore *store.ConversationStore, registry *modelmgr.Registry) *AgentService {
	tools_ := toolHandler.Tools()
	models := modelmgr.NewModelSet(registry, tools_)

	return &AgentService{
		store:          convStore,
		models:         models,
		tools:          tools_,
		compactTrigger: toolHandler.CompactTrigger,
		perms:          permission.NewPermissionManager(),
		skillReg:       skills.NewSkillRegistry(),
	}
}

// ModelSet 暴露服务级模型集合（供 handler 层切换/查询）
func (s *AgentService) ModelSet() *modelmgr.ModelSet {
	return s.models
}

// SwitchModel 切换当前模型（供切换端点调用）
func (s *AgentService) SwitchModel(ctx context.Context, id string) error {
	if err := s.models.Switch(ctx, id); err != nil {
		return err
	}
	contexttool.GetContextRegistry().SetMaxTokens(s.models.Current().ContextBudgetTokens())
	return nil
}

// Stream 准备请求级模型快照、组装上下文，并在启动工具循环前压缩完整消息列表。
func (s *AgentService) Stream(ctx context.Context,
	mode string, sessionID string, conversationID string, userMsg string) (*adk.AsyncIterator[*adk.AgentEvent], chan ToolEvent, error) {

	history, err := s.store.LoadHistory(sessionID, conversationID)
	if err != nil {
		return nil, nil, fmt.Errorf("加载对话历史失败: %w", err)
	}

	scope := contexttool.ScopeKey(sessionID, conversationID)
	state := contexttool.GetContextRegistry().State(scope)

	// 预算检查在槽位组装之后执行，因此会把 system 和当前用户输入计入。
	pl := pipeline.New(
		pipeline.NewSkillExpandStage(func(text string) (*skills.SkillDocument, bool) {
			doc := s.skillReg.Match(text)
			if doc == nil {
				return nil, false
			}
			return doc, true
		}),
		pipeline.NewModelPrepStage(s.models.Snapshot),
		pipeline.NewContextBuildStage(func(mc *pipeline.MessageContext) *ctxmgr.ContextManager {
			cm := ctxmgr.New(s.skillReg)
			return cm
		}),
		pipeline.NewCompactCheckStage(contexttool.CompactFunc, s.compactTrigger.IsTriggered),
	)

	mc := &pipeline.MessageContext{
		Raw:            userMsg,
		Mode:           mode,
		SessionID:      sessionID,
		ConversationID: conversationID,
		History:        history,
		CtxState:       state,
		PersistCompaction: func(result *contexttool.CompactionResult) error {
			return s.store.SaveCompaction(sessionID, conversationID, result.Summary, result.FirstKeptID)
		},
	}
	if err := pl.Run(ctx, mc); err != nil {
		return nil, nil, err
	}

	iterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	toolEvents := make(chan ToolEvent, 64)
	go s.runLoop(ctx, mc, generator, toolEvents)
	return iterator, toolEvents, nil
}

type collectedTurn struct {
	toolCalls    []schema.ToolCall
	thinking     string
	reply        string
	finishReason string
	err          error
}

func collectTurnWithMeta(stream *schema.StreamReader[*schema.Message], toolEvents chan<- ToolEvent) collectedTurn {
	var result collectedTurn
	var thinkingBuilder, replyBuilder strings.Builder
	for {
		chunk, err := stream.Recv()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				result.err = err
			}
			break
		}
		if chunk == nil {
			continue
		}
		if chunk.ResponseMeta != nil && chunk.ResponseMeta.FinishReason != "" {
			result.finishReason = chunk.ResponseMeta.FinishReason
		}
		if chunk.ReasoningContent != "" {
			thinkingBuilder.WriteString(chunk.ReasoningContent)
			toolEvents <- ToolEvent{Type: "thinking_delta", Content: chunk.ReasoningContent}
		}
		if chunk.Content != "" {
			replyBuilder.WriteString(chunk.Content)
			toolEvents <- ToolEvent{Type: "delta", Content: chunk.Content}
		}
		for i, tc := range chunk.ToolCalls {
			if i < len(result.toolCalls) {
				result.toolCalls[i].Function.Arguments += tc.Function.Arguments
				if tc.Function.Name != "" {
					result.toolCalls[i].Function.Name = tc.Function.Name
				}
				if tc.ID != "" {
					result.toolCalls[i].ID = tc.ID
				}
			} else {
				result.toolCalls = append(result.toolCalls, tc)
			}
		}
	}
	result.thinking = thinkingBuilder.String()
	result.reply = replyBuilder.String()
	return result
}

func outputLimitFinishReason(reason string) bool {
	switch strings.ToLower(reason) {
	case "length", "max_tokens", "max_output_tokens":
		return true
	default:
		return false
	}
}

const outputLimitMarker = "\n\n[回答因达到模型输出上限而截断]"

func (s *AgentService) runLoop(
	ctx context.Context,
	mc *pipeline.MessageContext,
	gen *adk.AsyncGenerator[*adk.AgentEvent],
	toolEvents chan ToolEvent,
) {

	defer close(toolEvents)
	defer gen.Close()
	messages := mc.Messages
	ctxState := mc.CtxState
	toolModel := mc.ToolModel
	scope := permission.ScopeKey(mc.SessionID, mc.ConversationID)
	defer s.perms.ResetDenials(scope)
	const maxToolRounds = 20
	todoUncalledCount := 0
	todoTools := 0

	// 判断有没有read_todo和write_todo工具，如果都有则提醒用户使用
	for _, registered := range s.tools {
		info, err := registered.Info(ctx)
		if err != nil {
			continue
		}
		switch info.Name {
		case "read_todo":
			todoTools |= 1
		case "write_todo":
			todoTools |= 2
		}
		if todoTools == 3 {
			break
		}
	}
	remindTodo := todoTools == 3

	for turn := range maxToolRounds + 1 {
		if err := ctx.Err(); err != nil {
			gen.Send(&adk.AgentEvent{Err: err})
			return
		}
		// 管线已检查过初始消息。这里只在工具结果扩展消息后再检查，
		// 避免紧接着重复触发一次摘要。
		if turn > 0 {
			level := contexttool.ShouldCompact(messages, ctxState)
			if s.compactTrigger.IsTriggered() {
				level = contexttool.CompactFull
			}
			switch level {
			case contexttool.CompactFull:
				// 工具循环中的压缩只影响本次请求；落盘由下一轮请求的管线统一完成，
				// 避免把尚未保存的当前轮消息重复写入历史。
				messages, _ = contexttool.CompactFunc(ctx, messages, mc.ChatModel, mc.SessionID, mc.ConversationID)
			case contexttool.CompactMicro:
				messages = contexttool.MicroCompactFunc(messages)
			}
		}

		// 最后一轮强制禁止工具调用，促使模型用文本收尾；其余轮次允许返回 tool_calls。
		var stream *schema.StreamReader[*schema.Message]
		var err error
		if turn == maxToolRounds {
			stream, err = toolModel.Stream(ctx, messages, model.WithToolChoice(schema.ToolChoiceForbidden))
		} else {
			stream, err = toolModel.Stream(ctx, messages)
		}
		if err != nil {
			gen.Send(&adk.AgentEvent{Err: err})
			return
		}

		// 读取整轮流：增量推送给前端，同时累积文本、思考内容和工具调用分片。
		turnResult := collectTurnWithMeta(stream, toolEvents)
		stream.Close()

		if turnResult.err != nil {
			gen.Send(&adk.AgentEvent{Err: turnResult.err})
			return
		}
		thinkingContent := turnResult.thinking
		replyContent := turnResult.reply
		toolCalls := turnResult.toolCalls

		// 服务端因输出上限截断时，补一条可见标记并终止本轮；
		// 此时不执行可能只收到一半的 tool_calls。
		if outputLimitFinishReason(turnResult.finishReason) {
			if replyContent != "" {
				toolEvents <- ToolEvent{Type: "delta", Content: outputLimitMarker}
				replyContent += outputLimitMarker
			}
			if thinkingContent != "" {
				toolEvents <- ToolEvent{Type: "thinking_done"}
			}
			gen.Send(&adk.AgentEvent{Err: fmt.Errorf("模型达到最大输出限制")})
			return
		}

		// 发送 thinking 结束信号
		if thinkingContent != "" {
			toolEvents <- ToolEvent{Type: "thinking_done"}
		}

		// 交付本轮回复给迭代器消费者；HTTP 正文通过 delta 收集，不再重复输出。
		replyMsg := schema.AssistantMessage(replyContent, nil)
		gen.Send(adk.EventFromMessage(replyMsg, nil, schema.Assistant, ""))

		log.Printf("[Turn %d] mode=%s ToolCalls 数量: %d", turn, mc.Mode, len(toolCalls))
		for i, tc := range toolCalls {
			log.Printf("[Turn %d] ToolCall[%d]: name=%s args=%s", turn, i, tc.Function.Name, tc.Function.Arguments)
		}

		// 没有工具调用说明模型已给出最终回答，结束循环。
		if len(toolCalls) == 0 {
			log.Printf("[Turn %d] 无 ToolCall，返回最终回答", turn)
			return
		}
		// 最后一轮已被禁止使用工具，仍然返回 tool_calls 属于异常。
		if turn == maxToolRounds {
			gen.Send(&adk.AgentEvent{Err: fmt.Errorf("模型在工具轮数上限后仍返回工具调用")})
			return
		}

		// todo 提醒（chat 与 aiops 均启用）
		if remindTodo {
			useTodo := false
			for _, tc := range toolCalls {
				if tc.Function.Name == "write_todo" || tc.Function.Name == "read_todo" {
					useTodo = true
					break
				}
			}
			if useTodo {
				// 调用过 todo 工具，重置未调用计数，避免误提醒
				todoUncalledCount = 0
			} else {
				todoUncalledCount++
			}
		}

		// 构造完整消息用于追加到历史
		fullMsg := &schema.Message{
			Role:      schema.Assistant,
			ToolCalls: toolCalls,
			Content:   replyContent,
		}
		messages = append(messages, fullMsg)

		needReminder := remindTodo && todoUncalledCount >= 5

		for _, tc := range toolCalls {
			toolEvents <- ToolEvent{
				Type:   "tool_call",
				Name:   tc.Function.Name,
				Args:   tc.Function.Arguments,
				CallID: tc.ID,
			}

			var toolInput map[string]any
			json.Unmarshal([]byte(tc.Function.Arguments), &toolInput)
			if toolInput == nil {
				toolInput = map[string]any{}
			}

			decision := s.perms.Check(scope, tc.Function.Name, toolInput)
			var result string
			execute := false

			switch decision.Behavior {
			case permission.BehaviorDeny:
				result = fmt.Sprintf("Permission denied: %s", decision.Reason)
				s.perms.RecordDenial(scope)
				log.Printf("[Turn %d] 工具 %s 被拒绝: %s", turn, tc.Function.Name, decision.Reason)

			case permission.BehaviorAsk:
				reqID := uuid.New().String()
				decisionCh := permission.WaitForDecision(reqID, mc.SessionID, mc.ConversationID, tc.Function.Name, toolInput)
				toolEvents <- ToolEvent{
					Type:      "permission_request",
					Name:      tc.Function.Name,
					Args:      tc.Function.Arguments,
					RequestID: reqID,
					Reason:    decision.Reason,
					CallID:    tc.ID,
				}

				select {
				case response := <-decisionCh:
					execute = response.Approved
					if !response.Approved {
						result = "Permission denied by user"
						s.perms.RecordDenial(scope)
					} else if response.Always {
						// 先安装规则再继续执行，后续同类调用无需再次确认。
						ruleScope := scope
						if response.Global {
							ruleScope = ""
						}
						s.perms.AddRule(ruleScope, permission.RuleForInvocation(tc.Function.Name, toolInput))
					}
				case <-time.After(60 * time.Second):
					result = "Permission request timed out"
					permission.CancelDecision(reqID)
				case <-ctx.Done():
					permission.CancelDecision(reqID)
					return
				}

			case permission.BehaviorAllow:
				execute = true
			default:
				result = "Permission denied: unsupported permission decision"
			}
			if execute {
				var toolErr error
				result, toolErr = s.executeTool(ctx, tc)
				if toolErr != nil {
					result = fmt.Sprintf("工具执行失败: %v", toolErr)
				} else {
					s.perms.ResetDenials(scope)
				}
			}

			if s.perms.ShouldConfirmPermissions(scope) {
				result += "\n\n[系统提醒] 工具调用已连续多次被拒绝。请停止重复尝试，向用户确认允许执行的范围，或仅基于已有信息回答。"
				s.perms.ResetDenials(scope)
			}

			if needReminder {
				result = result + "\n\n[系统提醒] 你已经连续多轮未更新待办事项，请立即调用 read_todo 和 write_todo 更新当前任务进度。"
				todoUncalledCount = 0
				needReminder = false
			}

			toolEvents <- ToolEvent{
				Type:   "tool_result",
				Name:   tc.Function.Name,
				Result: result,
				CallID: tc.ID,
			}

			msg := schema.ToolMessage(result, tc.ID, schema.WithToolName(tc.Function.Name))
			messages = append(messages, msg)
		}

		// 更新本次对话的上下文状态
		ctxState.EstimateAndEstimateMessages(messages)
	}
}

func (s *AgentService) executeTool(ctx context.Context, tc schema.ToolCall) (string, error) {
	for _, t := range s.tools {
		info, err := t.Info(ctx)
		if err != nil {
			continue
		}
		if info.Name == tc.Function.Name {
			if inv, ok := t.(tool.InvokableTool); ok {
				return inv.InvokableRun(ctx, tc.Function.Arguments)
			}
		}
	}
	return "", fmt.Errorf("tool not found: %s", tc.Function.Name)
}

func (s *AgentService) SaveReply(ctx context.Context, sessionID, conversationID string, msgs []*schema.Message) {
	if len(msgs) == 0 {
		return
	}
	if err := s.store.SaveMessages(sessionID, conversationID, msgs...); err != nil {
		log.Printf("AgentService: 保存对话失败 %v", err)
	}
}

// PermissionManager 暴露权限管理器，供 handler 层写入"始终允许"规则。
func (s *AgentService) PermissionManager() *permission.PermissionManager {
	return s.perms
}
