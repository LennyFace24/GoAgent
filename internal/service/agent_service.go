package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	cfg "github.com/LennyFace24/MiniAgent/internal/config"
	"github.com/LennyFace24/MiniAgent/internal/permission"
	"github.com/LennyFace24/MiniAgent/internal/prompt"
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
	Type      string `json:"type"`       // "tool_call" | "tool_result" | "permission_request"
	Name      string `json:"name"`       // 工具名称
	Args      string `json:"args,omitempty"`
	Result    string `json:"result,omitempty"`
	CallID    string `json:"call_id,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

// AgentService 统一的 Agent 服务，根据 mode 选择不同的 prompt 和行为
type AgentService struct {
	store          *store.ConversationStore
	baseModel      model.BaseChatModel
	toolModel      model.ToolCallingChatModel
	tools          []tool.BaseTool
	compactTrigger *contexttool.CompactTrigger
	perms          *permission.PermissionManager
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

	// 根据 mode 选择 prompt block
	b := prompt.NewBuilder()
	switch mode {
	case "aiops":
		b.Add(prompt.AIOpsCoreBlock())
	default:
		b.Add(prompt.CoreBlock())
	}
	b.Add(prompt.ToolsRuleBlock())

	messages := []*schema.Message{schema.SystemMessage(b.Build())}
	messages = append(messages, history...)
	messages = append(messages, schema.UserMessage(userMsg))

	iterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	toolEvents := make(chan ToolEvent, 64)
	go s.runLoop(ctx, mode, messages, generator, toolEvents, sessionID, conversationID)
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
) {
	defer close(toolEvents)
	defer gen.Close()

	const maxTurns = 20
	turn := 0
	todoUncalledCount := 0

	for {
		turn++

		// compact 检查
		level := contexttool.ShouldCompact(messages)
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

		streams := stream.Copy(2)
		gen.Send(adk.EventFromMessage(nil, streams[0], schema.Assistant, ""))

		fullMsg, err := schema.ConcatMessageStream(streams[1])
		if err != nil {
			gen.Send(&adk.AgentEvent{Err: err})
			return
		}

		log.Printf("[Turn %d] mode=%s ToolCalls 数量: %d", turn, mode, len(fullMsg.ToolCalls))
		for i, tc := range fullMsg.ToolCalls {
			log.Printf("[Turn %d] ToolCall[%d]: name=%s args=%s", turn, i, tc.Function.Name, tc.Function.Arguments)
		}

		if len(fullMsg.ToolCalls) == 0 {
			log.Printf("[Turn %d] 无 ToolCall，返回最终回答", turn)
			return
		}

		// todo 提醒（仅 chat 模式）
		if mode != "aiops" {
			useTodo := false
			for _, tc := range fullMsg.ToolCalls {
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

		messages = append(messages, fullMsg)

		needReminder := mode != "aiops" && todoUncalledCount == 0

		for _, tc := range fullMsg.ToolCalls {
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
	}

	// 达到最大轮次，强制最后一轮直接回答
	messages = append(messages, schema.UserMessage("你已调用足够多次工具，现在必须直接回答用户的问题。不要再调用任何工具。"))
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
