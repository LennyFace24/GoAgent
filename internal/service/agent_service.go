package service

import (
	"context"
	"encoding/json"
	"fmt"
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
		perms:          permission.NewPermissionManager(permission.ModeDefault),
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
	contexttool.GetContextState().ConfigureMaxTokens(s.models.Current().ContextBudgetTokens())
	return nil
}

// Stream prepares a request-level model snapshot, assembles its context, and
// compacts the complete message list before starting the tool loop.
func (s *AgentService) Stream(ctx context.Context,
	mode string, sessionID string, conversationID string, userMsg string) (*adk.AsyncIterator[*adk.AgentEvent], chan ToolEvent, error) {

	history, err := s.store.LoadHistory(sessionID, conversationID)
	if err != nil {
		log.Printf("AgentService: 加载历史失败 %v", err)
		history = nil
	}

	globalState := contexttool.GetContextState()

	// Budget checking runs after slot assembly so it includes system and user input.
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
	}
	if err := pl.Run(ctx, mc); err != nil {
		return nil, nil, err
	}

	globalState.ConfigureMaxTokens(mc.CtxState.MaxTokens())
	globalState.SetTokens(mc.CtxState.GetUsage().CurrentTokens)
	iterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	toolEvents := make(chan ToolEvent, 64)
	go s.runLoop(ctx, mc, generator, toolEvents, globalState)
	return iterator, toolEvents, nil
}

// collectTurn 读取一轮模型流，推送 thinking_delta/delta 事件到 toolEvents，
// 并累积跨 chunk 分片的 tool_calls，返回本轮的 toolCalls、思考内容与回复内容。
func collectTurn(stream *schema.StreamReader[*schema.Message], toolEvents chan<- ToolEvent) (toolCalls []schema.ToolCall, thinking, reply string) {
	var thinkingBuilder, replyBuilder strings.Builder
	for {
		chunk, err := stream.Recv()
		if err != nil {
			break
		}
		if chunk.ReasoningContent != "" {
			thinkingBuilder.WriteString(chunk.ReasoningContent)
			toolEvents <- ToolEvent{
				Type:    "thinking_delta",
				Content: chunk.ReasoningContent,
			}
		}
		if chunk.Content != "" {
			replyBuilder.WriteString(chunk.Content)
			toolEvents <- ToolEvent{
				Type:    "delta",
				Content: chunk.Content,
			}
		}
		for i, tc := range chunk.ToolCalls {
			if i < len(toolCalls) {
				toolCalls[i].Function.Arguments += tc.Function.Arguments
				if tc.Function.Name != "" {
					toolCalls[i].Function.Name = tc.Function.Name
				}
				if tc.ID != "" {
					toolCalls[i].ID = tc.ID
				}
			} else {
				toolCalls = append(toolCalls, tc)
			}
		}
	}
	return toolCalls, thinkingBuilder.String(), replyBuilder.String()
}

func (s *AgentService) runLoop(
	ctx context.Context,
	mc *pipeline.MessageContext,
	gen *adk.AsyncGenerator[*adk.AgentEvent],
	toolEvents chan ToolEvent,
	globalState *contexttool.ContextState,
) {

	defer close(toolEvents)
	defer gen.Close()
	messages := mc.Messages
	ctxState := mc.CtxState
	toolModel := mc.ToolModel

	const maxToolRounds = 20
	todoUncalledCount := 0
	todoTools := 0
	if mc.Mode != "aiops" {
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
	}
	remindTodo := todoTools == 3

	for turn := range maxToolRounds + 1 {
		if err := ctx.Err(); err != nil {
			gen.Send(&adk.AgentEvent{Err: err})
			return
		}
		// Pipeline already checked the initial messages. Recheck only after
		// tool results have extended them, avoiding an immediate second summary.
		if turn > 0 {
			level := contexttool.ShouldCompact(messages, ctxState)
			if s.compactTrigger.IsTriggered() {
				level = contexttool.CompactFull
			}
			switch level {
			case contexttool.CompactFull:
				messages = contexttool.CompactFunc(ctx, messages, mc.ChatModel, mc.SessionID, mc.ConversationID)
			case contexttool.CompactMicro:
				messages = contexttool.MicroCompactFunc(messages)
			}
		}

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

		// 读取 stream，统一处理 thinking 和 reply 内容
		toolCalls, thinkingContent, replyContent := collectTurn(stream, toolEvents)
		stream.Close()

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

		if len(toolCalls) == 0 {
			log.Printf("[Turn %d] 无 ToolCall，返回最终回答", turn)
			return
		}
		if turn == maxToolRounds {
			gen.Send(&adk.AgentEvent{Err: fmt.Errorf("模型在工具轮数上限后仍返回工具调用")})
			return
		}

		// todo 提醒（仅 chat 模式）
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

			decision := s.perms.Check(tc.Function.Name, toolInput)
			var result string
			execute := false

			switch decision.Behavior {
			case permission.BehaviorDeny:
				result = fmt.Sprintf("Permission denied: %s", decision.Reason)
				s.perms.RecordDenial()
				log.Printf("[Turn %d] 工具 %s 被拒绝: %s", turn, tc.Function.Name, decision.Reason)

			case permission.BehaviorAsk:
				reqID := uuid.New().String()
				decisionCh := permission.WaitForDecision(reqID, mc.SessionID, mc.ConversationID, tc.Function.Name)
				toolEvents <- ToolEvent{
					Type:      "permission_request",
					Name:      tc.Function.Name,
					Args:      tc.Function.Arguments,
					RequestID: reqID,
					Reason:    decision.Reason,
					CallID:    tc.ID,
				}

				select {
				case approved := <-decisionCh:
					execute = approved
					if !approved {
						result = "Permission denied by user"
						s.perms.RecordDenial()
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
				}
				s.perms.ResetDenials()
			}

			if s.perms.ShouldConfirmPermissions() {
				result += "\n\n[系统提醒] 工具调用已连续多次被拒绝。请停止重复尝试，向用户确认允许执行的范围，或仅基于已有信息回答。"
				s.perms.ResetDenials()
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

		// 更新上下文状态（使用本次请求的独立实例）
		ctxState.EstimateAndEstimateMessages(messages)
		globalState.SetTokens(ctxState.GetUsage().CurrentTokens)
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
