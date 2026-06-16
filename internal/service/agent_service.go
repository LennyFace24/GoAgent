package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"


	cfg "github.com/LennyFace24/MiniAgent/internal/config"
	ctxmgr "github.com/LennyFace24/MiniAgent/internal/context"
	"github.com/LennyFace24/MiniAgent/internal/permission"
	"github.com/LennyFace24/MiniAgent/internal/skills"
	"github.com/LennyFace24/MiniAgent/internal/store"
	"github.com/LennyFace24/MiniAgent/internal/tools"
	contexttool "github.com/LennyFace24/MiniAgent/internal/tools/context_tool"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)



// ToolEvent 通过 SSE 推送给前端的工具事件
type ToolEvent struct {
	Type      string `json:"type"`       // "tool_call" | "tool_result" | "permission_request" | "thinking"
	Name      string `json:"name"`       // 工具名称
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
	baseModel      model.BaseChatModel
	toolModel      model.ToolCallingChatModel
	tools          []tool.BaseTool
	compactTrigger *contexttool.CompactTrigger
	perms          *permission.PermissionManager
	budget         int64 // 上下文 token 预算（ContextBudgetTokens）
	skillReg       *skills.SkillRegistry
}



func NewAgentService(toolHandler *tools.ToolHandler, convStore *store.ConversationStore) *AgentService {
	compactTrigger := toolHandler.CompactTrigger
	maxTokens := cfg.GetConfig().Llm.MaxTokens
	chatModel, err := openai.NewChatModel(context.Background(), &openai.ChatModelConfig{
		Model:               cfg.GetConfig().Llm.Model,
		APIKey:              cfg.GetConfig().Llm.ApiKey,
		BaseURL:             cfg.GetConfig().Llm.BaseUrl,
		MaxCompletionTokens: &maxTokens,
	})
	if err != nil {
		log.Printf("AgentService: ChatModel创建失败 %v", err)
		return nil
	}

	tools_ := toolHandler.Tools()
	toolInfos := make([]*schema.ToolInfo, len(tools_))
	for i, t := range tools_ {
		info, infoErr := t.Info(context.Background())
		if infoErr != nil {
			log.Printf("AgentService: 获取工具信息失败 %v", infoErr)
			return nil
		}
		toolInfos[i] = info
	}

	toolModel, err := chatModel.WithTools(toolInfos)
	if err != nil {
		log.Printf("AgentService: 绑定工具失败 %v", err)
		return nil
	}

	return &AgentService{
		store:          convStore,
		baseModel:      chatModel,
		toolModel:      toolModel,
		tools:          tools_,
		compactTrigger: compactTrigger,
		perms:          permission.NewPermissionManager(permission.ModeDefault),
		budget:         cfg.GetConfig().ContextBudgetTokens(),
		skillReg:       skills.NewSkillRegistry(),
	}

}

// Stream 启动流式 Agent 对话，mode 支持 "chat" 和 "aiops"
func (s *AgentService) Stream(ctx context.Context,
	mode string, sessionID string, conversationID string, userMsg string) (*adk.AsyncIterator[*adk.AgentEvent], chan ToolEvent, error) {

	history, err := s.store.LoadHistory(sessionID, conversationID)
	if err != nil {
		log.Printf("AgentService: 加载历史失败 %v", err)
		history = nil
	}

	// 构建上下文
	cm := ctxmgr.New(s.budget, s.skillReg)
	cm.SetMode(mode)
	cm.LoadSystemInfo()
	cm.LoadSoul()
	cm.LoadProjectContext()
	cm.LoadSkills()
	cm.LoadMemory()


	history = cm.BuildHistory(ctx, history, s.baseModel, sessionID, conversationID)
	messages := cm.BuildMessages(history, userMsg)

	// 每次请求创建独立的 ContextState，避免跨对话污染
	ctxState := contexttool.NewContextState(s.budget)

	// 同步全局单例（供 /context HTTP 端点展示）
	globalState := contexttool.GetContextState()
	globalState.ConfigureMaxTokens(s.budget)

	iterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	toolEvents := make(chan ToolEvent, 64)
	go s.runLoop(ctx, mode, messages, generator, toolEvents, sessionID, conversationID, ctxState)
	return iterator, toolEvents, nil
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
) {

	defer close(toolEvents)
	defer gen.Close()

	const maxTurns = 20
	turn := 0
	todoUncalledCount := 0


	for {
		// 更新上下文状态（使用本次请求的独立实例）
		ctxState.EstimateAndEstimateMessages(messages)

		// compact 检查
		level := contexttool.ShouldCompact(messages, ctxState)

		if s.compactTrigger.IsTriggered() {
			level = contexttool.CompactFull
		}
		switch level {
		case contexttool.CompactFull:
			messages = contexttool.CompactFunc(ctx, messages, s.baseModel, sessionID, conversationID)
		case contexttool.CompactMicro:
			messages = contexttool.MicroCompactFunc(messages)
		}

		stream, err := s.toolModel.Stream(ctx, messages)
		if err != nil {
			gen.Send(&adk.AgentEvent{Err: err})
			return
		}

		// 读取 stream，统一处理 thinking 和 reply 内容
		var toolCalls []schema.ToolCall
		var thinkingBuilder strings.Builder
		var replyBuilder strings.Builder

		for {
			chunk, err := stream.Recv()
			if err != nil {
				break
			}
			// thinking 内容
			if chunk.ReasoningContent != "" {
				thinkingBuilder.WriteString(chunk.ReasoningContent)
				toolEvents <- ToolEvent{
					Type:    "thinking_delta",
					Content: chunk.ReasoningContent,
				}
			}
			// reply 内容
			if chunk.Content != "" {
				replyBuilder.WriteString(chunk.Content)
				toolEvents <- ToolEvent{
					Type:    "delta",
					Content: chunk.Content,
				}
			}
			// 收集 tool calls
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
		thinkingContent := thinkingBuilder.String()
		replyContent := replyBuilder.String()

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
			if !useTodo {
				todoUncalledCount++
			}
			if !useTodo && todoUncalledCount >= 5 {
				todoUncalledCount = 0
				// 将在工具结果中注入提醒
			}
		}

		// 构造完整消息用于追加到历史
		fullMsg := &schema.Message{
			Role:      schema.Assistant,
			ToolCalls: toolCalls,
		}
		messages = append(messages, fullMsg)

		needReminder := mode != "aiops" && todoUncalledCount == 0

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
				toolEvents <- ToolEvent{
					Type:      "permission_request",
					Name:      tc.Function.Name,
					Args:      tc.Function.Arguments,
					RequestID: reqID,
					Reason:    decision.Reason,
					CallID:    tc.ID,
				}

				select {
				case approved := <-permission.WaitForDecision(reqID):
					if approved {
						result, _ = s.executeTool(ctx, tc)
						s.perms.ResetDenials()
					} else {
						result = "Permission denied by user"
						s.perms.RecordDenial()
					}
				case <-time.After(60 * time.Second):
					result = "Permission request timed out"
					permission.CancelDecision(reqID)
				case <-ctx.Done():
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
	}
	stream, err := s.toolModel.Stream(ctx, messages)
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
