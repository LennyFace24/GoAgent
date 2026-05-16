package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	cfg "github.com/LennyFace24/MiniAgent/internal/config"
	"github.com/LennyFace24/MiniAgent/internal/permission"
	skills_registry "github.com/LennyFace24/MiniAgent/internal/skills/registry"
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

const instruction = `你是专业的智能问答助手。

# 行为准则
- 用户的请求主要涉及知识检索和问题回答。
- 优先使用工具获取信息，获取到信息后立即给出答案。
- 回答时基于工具返回的文档内容，标注信息来源。

# 工具使用规范
- 每轮对话中每种工具最多调用一次。
- 工具返回空结果时，直接回复"根据现有资料无法回答该问题"。

# 输出规范
- 中文回答，简洁专业，适当分段。`

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

type ChatStreamService struct {
	store          *store.ConversationStore
	baseModel      model.BaseChatModel
	toolModel      model.ToolCallingChatModel
	tools          []tool.BaseTool
	skills         *skills_registry.SkillRegistry
	compactTrigger *contexttool.CompactTrigger
	perms          *permission.PermissionManager
}

func NewChatStreamService(toolHandler *tools.ToolHandler, convStore *store.ConversationStore, skills *skills_registry.SkillRegistry) *ChatStreamService {
	compactTrigger := toolHandler.CompactTrigger
	maxTokens := cfg.GetConfig().Llm.MaxTokens
	chatModel, err := openai.NewChatModel(context.Background(), &openai.ChatModelConfig{
		Model:               cfg.GetConfig().Llm.Model,
		APIKey:              cfg.GetConfig().Llm.ApiKey,
		BaseURL:             cfg.GetConfig().Llm.BaseUrl,
		MaxCompletionTokens: &maxTokens,
	})
	if err != nil {
		log.Printf("ChatStreamService: ChatModel创建失败 %v", err)
		return nil
	}

	tools_ := toolHandler.Tools()
	toolInfos := make([]*schema.ToolInfo, len(tools_))
	for i, t := range tools_ {
		info, infoErr := t.Info(context.Background())
		if infoErr != nil {
			log.Printf("ChatStreamService: 获取工具信息失败 %v", infoErr)
			return nil
		}
		toolInfos[i] = info
	}

	toolModel, err := chatModel.WithTools(toolInfos)
	if err != nil {
		log.Printf("ChatStreamService: 绑定工具失败 %v", err)
		return nil
	}

	return &ChatStreamService{
		store:          convStore,
		baseModel:      chatModel,
		toolModel:      toolModel,
		tools:          tools_,
		skills:         skills,
		compactTrigger: compactTrigger,
		perms:          permission.NewPermissionManager(permission.ModeDefault),
	}
}

func (s *ChatStreamService) ChatStream(ctx context.Context,
	sessionID string, conversationID string, userMsg string) (*adk.AsyncIterator[*adk.AgentEvent], chan ToolEvent, error) {

	history, err := s.store.LoadHistory(sessionID, conversationID)
	if err != nil {
		log.Printf("ChatStreamService: 加载历史失败 %v", err)
		history = nil
	}

	systemPrompt := instruction
	if desc := s.skills.DescribeAvailable(); desc != "" {
		systemPrompt += "\n\n# 已安装的 Agent Skills（技能模块）\n" + desc + "\n以上是系统预装的技能模块，不是你的通用能力。当用户提到某个技能相关的需求时，调用 skill 工具（传入技能名称）来加载该技能的完整规则，然后按规则执行。"
	}
	messages := []*schema.Message{schema.SystemMessage(systemPrompt)}
	messages = append(messages, history...)
	messages = append(messages, schema.UserMessage(userMsg))

	iterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	toolEvents := make(chan ToolEvent, 64)
	go s.runConversation(ctx, messages, generator, toolEvents, sessionID, conversationID)
	return iterator, toolEvents, nil
}

func (s *ChatStreamService) runConversation(
	ctx context.Context,
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
	todo_uncalled_count := 0

	for {
		// 回合数+1
		turn++
		// todo工具调用提醒
		use_todo := false

		// compact 检查：token 超阈值 或 LLM 主动触发
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

		log.Printf("[Turn %d] ToolCalls 数量: %d", turn, len(fullMsg.ToolCalls))
		for i, tc := range fullMsg.ToolCalls {
			log.Printf("[Turn %d] ToolCall[%d]: name=%s args=%s", turn, i, tc.Function.Name, tc.Function.Arguments)
		}

		// 检查是否有toolcall，没有则直接返回
		if len(fullMsg.ToolCalls) == 0 {
			log.Printf("[Turn %d] 无 ToolCall，返回最终回答", turn)
			return
		}

		for _, tc := range fullMsg.ToolCalls {
			if tc.Function.Name == "write_todo" || tc.Function.Name == "read_todo" {
				use_todo = true
				break
			}
		}
		if !use_todo {
			todo_uncalled_count++
		}
		messages = append(messages, fullMsg)

		needReminder := !use_todo && todo_uncalled_count >= 5
		if needReminder {
			todo_uncalled_count = 0
		}

		for _, tc := range fullMsg.ToolCalls {
			// 发送 tool_call 事件
			toolEvents <- ToolEvent{
				Type:   "tool_call",
				Name:   tc.Function.Name,
				Args:   tc.Function.Arguments,
				CallID: tc.ID,
			}

			// 解析工具参数
			var toolInput map[string]any
			json.Unmarshal([]byte(tc.Function.Arguments), &toolInput)
			if toolInput == nil {
				toolInput = map[string]any{}
			}

			// 权限检查
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

			default: // allow
				var toolErr error
				result, toolErr = s.executeTool(ctx, tc)
				if toolErr != nil {
					result = fmt.Sprintf("工具执行失败: %v", toolErr)
				}
				s.perms.ResetDenials()
			}

			// 熔断器：连续拒绝 3 次，提示切换模式
			if s.perms.ShouldSuggestPlanMode() {
				result += "\n\n[系统提醒] 工具调用已连续多次被拒绝，建议切换到 plan 模式。"
				s.perms.ResetDenials()
			}

			if needReminder {
				result = result + "\n\n[系统提醒] 你已经连续多轮未更新待办事项，请立即调用 read_todo 和 write_todo 更新当前任务进度。"
				needReminder = false
			}

			// 发送 tool_result 事件
			toolEvents <- ToolEvent{
				Type:   "tool_result",
				Name:   tc.Function.Name,
				Result: result,
				CallID: tc.ID,
			}

			msg := schema.ToolMessage(result, tc.ID, schema.WithToolName(tc.Function.Name))
			messages = append(messages, msg)
		}

		// 达到最大轮次仍未回答，强制最后一轮
		if turn >= maxTurns {
			break
		}
	}
	messages = append(messages, schema.UserMessage("你已调用足够多次工具，现在必须直接回答用户的问题。不要再调用任何工具。"))
	stream, err := s.toolModel.Stream(ctx, messages)
	if err != nil {
		gen.Send(&adk.AgentEvent{Err: err})
		return
	}
	gen.Send(adk.EventFromMessage(nil, stream, schema.Assistant, ""))

}

func (s *ChatStreamService) executeTool(ctx context.Context, tc schema.ToolCall) (string, error) {
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

func (s *ChatStreamService) SaveReply(ctx context.Context, sessionID, conversationID, userMsg, aiMsg string) {
	if err := s.store.SaveMessages(sessionID, conversationID,
		schema.UserMessage(userMsg),
		schema.AssistantMessage(aiMsg, nil),
	); err != nil {
		log.Printf("ChatStreamService: 保存对话失败 %v", err)
	}
}
