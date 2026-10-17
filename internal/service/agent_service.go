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
	budget         int64 // 上下文 token 预算（当前激活模型的 ContextBudgetTokens）
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
		budget:         models.Current().ContextBudgetTokens(),
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
	contexttool.GetContextState().ConfigureMaxTokens(s.budget)
	return nil
}

// Stream 启动流式 Agent 对话，mode 支持 "chat" 和 "aiops"。
// 消息经预处理管线（skill 匹配 → 模型准备 → 压缩预检 → 上下文组装）后进入 runLoop。
func (s *AgentService) Stream(ctx context.Context,
	mode string, sessionID string, conversationID string, userMsg string) (*adk.AsyncIterator[*adk.AgentEvent], chan ToolEvent, error) {

	history, err := s.store.LoadHistory(sessionID, conversationID)
	if err != nil {
		log.Printf("AgentService: 加载历史失败 %v", err)
		history = nil
	}

	// 每次请求创建独立的 ContextState，避免跨对话污染
	ctxState := contexttool.NewContextState(s.budget)

	// 同步全局单例（供 /context HTTP 端点展示）
	globalState := contexttool.GetContextState()
	globalState.ConfigureMaxTokens(s.budget)

	// 组装预处理管线：skill 触发词匹配 → 模型准备 → 压缩预检 → 上下文组装
	pl := pipeline.New(
		pipeline.NewSkillExpandStage(func(text string) (*skills.SkillDocument, bool) {
			doc := s.skillReg.Match(text)
			if doc == nil {
				return nil, false
			}
			return doc, true
		}),
		pipeline.NewModelPrepStage(nil, s.buildToolModel),
		pipeline.NewCompactCheckStage(s.baseModel()),
		pipeline.NewContextBuildStage(func(mc *pipeline.MessageContext) *ctxmgr.ContextManager {
			cm := ctxmgr.New(mc.CtxState.MaxTokens(), s.skillReg)
			return cm
		}),
		pipeline.NewSendStage(),
	)

	mc := &pipeline.MessageContext{
		Raw:            userMsg,
		Mode:           mode,
		SessionID:      sessionID,
		ConversationID: conversationID,
		History:        history,
		CtxState:       ctxState,
	}
	if err := pl.Run(ctx, mc); err != nil {
		return nil, nil, err
	}

	iterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	toolEvents := make(chan ToolEvent, 64)
	go s.runLoop(ctx, mode, mc.Messages, generator, toolEvents, sessionID, conversationID, ctxState, globalState)
	return iterator, toolEvents, nil
}

// baseModel 返回当前激活模型的 ChatModel（惰性构建）
func (s *AgentService) baseModel() model.BaseChatModel {
	cm, err := s.models.ChatModel(context.Background())
	if err != nil {
		log.Printf("AgentService: ChatModel 构建失败 %v", err)
		return nil
	}
	return cm
}

// toolModel 返回当前激活模型的带工具模型（惰性构建）
func (s *AgentService) toolModel() model.ToolCallingChatModel {
	tm, err := s.models.ToolModel(context.Background())
	if err != nil {
		log.Printf("AgentService: ToolModel 构建失败 %v", err)
		return nil
	}
	return tm
}

// buildToolModel 构建带工具调用的模型（供管线 ModelPrep 阶段使用）
func (s *AgentService) buildToolModel(ctx context.Context) (model.ToolCallingChatModel, error) {
	return s.models.ToolModel(ctx)
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
	mode string,
	messages []*schema.Message,
	gen *adk.AsyncGenerator[*adk.AgentEvent],
	toolEvents chan ToolEvent,
	sessionID string,
	conversationID string,
	ctxState *contexttool.ContextState,
	globalState *contexttool.ContextState,
) {

	defer close(toolEvents)
	defer gen.Close()

	const maxTurns = 20
	turn := 0
	todoUncalledCount := 0

	for {

		// compact 检查
		level := contexttool.ShouldCompact(messages, ctxState)

		if s.compactTrigger.IsTriggered() {
			level = contexttool.CompactFull
		}
		switch level {
		case contexttool.CompactFull:
			messages = contexttool.CompactFunc(ctx, messages, s.baseModel(), sessionID, conversationID)
		case contexttool.CompactMicro:
			messages = contexttool.MicroCompactFunc(messages)
		}

		stream, err := s.toolModel().Stream(ctx, messages)
		if err != nil {
			gen.Send(&adk.AgentEvent{Err: err})
			return
		}

		// 读取 stream，统一处理 thinking 和 reply 内容
		toolCalls, thinkingContent, replyContent := collectTurn(stream, toolEvents)

		// 发送 thinking 结束信号
		if thinkingContent != "" {
			toolEvents <- ToolEvent{Type: "thinking_done"}
		}

		// 发送 reply 到 handler 用于持久化
		replyMsg := schema.AssistantMessage(replyContent, nil)
		gen.Send(adk.EventFromMessage(replyMsg, nil, schema.Assistant, ""))

		log.Printf("[Turn %d] mode=%s ToolCalls 数量: %d", turn, mode, len(toolCalls))
		for i, tc := range toolCalls {
			log.Printf("[Turn %d] ToolCall[%d]: name=%s args=%s", turn, i, tc.Function.Name, tc.Function.Arguments)
		}

		if len(toolCalls) == 0 {
			log.Printf("[Turn %d] 无 ToolCall，返回最终回答", turn)
			return
		}

		// todo 提醒（仅 chat 模式）
		if mode != "aiops" {
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
		}
		messages = append(messages, fullMsg)

		needReminder := mode != "aiops" && todoUncalledCount >= 5

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

			switch decision.Behavior {
			case permission.BehaviorDeny:
				result = fmt.Sprintf("Permission denied: %s", decision.Reason)
				s.perms.RecordDenial()
				log.Printf("[Turn %d] 工具 %s 被拒绝: %s", turn, tc.Function.Name, decision.Reason)

			case "ask":
				reqID := uuid.New().String()
				decisionCh := permission.WaitForDecision(reqID, sessionID, conversationID, tc.Function.Name)
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
					if approved {
						var toolErr error
						result, toolErr = s.executeTool(ctx, tc)
						if toolErr != nil {
							result = fmt.Sprintf("工具执行失败: %v", toolErr)
						}
						s.perms.ResetDenials()
					} else {
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

			default:
				var toolErr error
				result, toolErr = s.executeTool(ctx, tc)
				if toolErr != nil {
					result = fmt.Sprintf("工具执行失败: %v", toolErr)
				}
				s.perms.ResetDenials()
			}

			if s.perms.ShouldSuggestPlanMode() {
				result += "\n\n[系统提醒] 工具调用已连续多次被拒绝，建议切换到 plan 模式。"
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

		if turn >= maxTurns {
			break
		}
		turn++
		// 更新上下文状态（使用本次请求的独立实例）
		ctxState.EstimateAndEstimateMessages(messages)
		globalState.SetTokens(ctxState.GetUsage().CurrentTokens)
	}
	stream, err := s.toolModel().Stream(ctx, messages)
	if err != nil {
		gen.Send(&adk.AgentEvent{Err: err})
		return
	}
	gen.Send(adk.EventFromMessage(nil, stream, schema.Assistant, ""))
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
