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

type AIOpsService struct {
	store          *store.ConversationStore
	baseModel      model.BaseChatModel
	toolModel      model.ToolCallingChatModel
	tools          []tool.BaseTool
	compactTrigger *contexttool.CompactTrigger
	perms          *permission.PermissionManager
}

func NewAIOpsService(toolHandler *tools.ToolHandler, convStore *store.ConversationStore) *AIOpsService {
	compactTrigger := toolHandler.CompactTrigger
	maxTokens := cfg.GetConfig().Llm.MaxTokens
	chatModel, err := openai.NewChatModel(context.Background(), &openai.ChatModelConfig{
		Model:               cfg.GetConfig().Llm.Model,
		APIKey:              cfg.GetConfig().Llm.ApiKey,
		BaseURL:             cfg.GetConfig().Llm.BaseUrl,
		MaxCompletionTokens: &maxTokens,
	})
	if err != nil {
		log.Printf("AIOpsService: ChatModel创建失败 %v", err)
		return nil
	}

	tools_ := toolHandler.Tools()
	toolInfos := make([]*schema.ToolInfo, len(tools_))
	for i, t := range tools_ {
		info, infoErr := t.Info(context.Background())
		if infoErr != nil {
			log.Printf("AIOpsService: 获取工具信息失败 %v", infoErr)
			return nil
		}
		toolInfos[i] = info
	}

	toolModel, err := chatModel.WithTools(toolInfos)
	if err != nil {
		log.Printf("AIOpsService: 绑定工具失败 %v", err)
		return nil
	}

	return &AIOpsService{
		store:          convStore,
		baseModel:      chatModel,
		toolModel:      toolModel,
		tools:          tools_,
		compactTrigger: compactTrigger,
		perms:          permission.NewPermissionManager(permission.ModeDefault),
	}
}

func (s *AIOpsService) Diagnose(ctx context.Context,
	sessionID string, conversationID string, message string) (*adk.AsyncIterator[*adk.AgentEvent], chan ToolEvent, error) {

	history, err := s.store.LoadHistory(sessionID, conversationID)
	if err != nil {
		log.Printf("AIOpsService: 加载历史失败 %v", err)
		history = nil
	}

	b := prompt.NewBuilder()
	b.Add(prompt.AIOpsCoreBlock())
	b.Add(prompt.ToolsRuleBlock())
	messages := []*schema.Message{schema.SystemMessage(b.Build())}
	messages = append(messages, history...)
	messages = append(messages, schema.UserMessage(message))

	iterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	toolEvents := make(chan ToolEvent, 64)
	go s.runDiagnosis(ctx, messages, generator, toolEvents, sessionID, conversationID)
	return iterator, toolEvents, nil
}

func (s *AIOpsService) runDiagnosis(
	ctx context.Context,
	messages []*schema.Message,
	gen *adk.AsyncGenerator[*adk.AgentEvent],
	toolEvents chan ToolEvent,
	sessionID string,
	conversationID string,
) {
	defer gen.Close()
	defer close(toolEvents)

	const safetyLimit = 20
	for turn := 0; turn < safetyLimit; turn++ {
		log.Printf("[AIOps Turn %d] 调用 LLM...", turn)

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

		log.Printf("[AIOps Turn %d] ToolCalls 数量: %d", turn, len(fullMsg.ToolCalls))
		for i, tc := range fullMsg.ToolCalls {
			log.Printf("[AIOps Turn %d] ToolCall[%d]: name=%s args=%s", turn, i, tc.Function.Name, tc.Function.Arguments)
		}

		if len(fullMsg.ToolCalls) == 0 {
			log.Printf("[AIOps Turn %d] 无 ToolCall，诊断结束", turn)
			return
		}

		messages = append(messages, fullMsg)
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

			// 熔断器
			if s.perms.ShouldSuggestPlanMode() {
				result += "\n\n[系统提醒] 工具调用已连续多次被拒绝，建议切换到 plan 模式。"
				s.perms.ResetDenials()
			}

			log.Printf("[AIOps Turn %d] 工具 %s 返回 %d 字符", turn, tc.Function.Name, len(result))

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
	}
}

func (s *AIOpsService) executeTool(ctx context.Context, tc schema.ToolCall) (string, error) {
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
