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

const aiopsInstruction = `你是专业的运维诊断专家。

# 工作流程
收到故障描述后，严格按以下步骤进行：

## 阶段 1: 计划 (Plan)
分析问题，输出排查计划。格式：
"## 排查计划
1. [步骤1]
2. [步骤2]
..."

## 阶段 2: 执行 (Execute)
使用工具逐步执行计划：
- 优先使用 health_check 检查服务健康状态
- 使用 log_analyzer 分析错误日志
- 使用 knowledge_search 查询相关运维文档
- 每步执行后评估是否需要调整计划

## 阶段 3: 重规划 (Replan)
如果执行结果指向新问题方向，在此说明并调整后续步骤。

## 阶段 4: 报告 (Report)
汇总所有发现，输出最终诊断报告。格式：
"## 诊断报告

### 根因分析
...
### 时间线
...
### 影响范围
...
### 修复建议
1. ...
2. ..."

# 行为准则
- 先计划再执行，不要跳过计划阶段
- 每轮至少使用一种工具获取信息
- 所有判断基于工具返回的实际数据
- 最终报告前必须完成所有排查步骤`

type AIOpsService struct {
	store          *store.ConversationStore
	baseModel      model.BaseChatModel
	toolModel      model.ToolCallingChatModel
	tools          []tool.BaseTool
	skills         *skills_registry.SkillRegistry
	compactTrigger *contexttool.CompactTrigger
	perms          *permission.PermissionManager
}

func NewAIOpsService(toolHandler *tools.ToolHandler, convStore *store.ConversationStore, skills *skills_registry.SkillRegistry) *AIOpsService {
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
		skills:         skills,
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

	systemPrompt := aiopsInstruction
	if desc := s.skills.DescribeAvailable(); desc != "" {
		systemPrompt += "\n\n# 已安装的 Agent Skills（技能模块）\n" + desc + "\n以上是系统预装的技能模块，不是你的通用能力。当用户提到某个技能相关的需求时，调用 skill 工具（传入技能名称）来加载该技能的完整规则，然后按规则执行。"
	}
	messages := []*schema.Message{schema.SystemMessage(systemPrompt)}
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
