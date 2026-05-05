package service

import (
	"context"
	"fmt"
	"log"

	cfg "github.com/LennyFace24/MiniAgent/internal/config"
	"github.com/LennyFace24/MiniAgent/internal/store"
	"github.com/LennyFace24/MiniAgent/internal/tools"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
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
	store       *store.ConversationStore
	fileService *FileService
	toolModel   model.ToolCallingChatModel
	tools       []tool.BaseTool
}

func NewAIOpsService(fileService *FileService, convStore *store.ConversationStore) *AIOpsService {
	healthCheckTool, err := tools.NewHealthCheckTool(cfg.GetConfig().Prometheus.URL)
	if err != nil {
		log.Printf("AIOpsService: 健康检查工具创建失败 %v", err)
		return nil
	}

	logAnalyzerTool, err := tools.NewLogAnalyzerTool()
	if err != nil {
		log.Printf("AIOpsService: 日志分析工具创建失败 %v", err)
		return nil
	}

	knowledgeTool, err := tools.NewKnowledgeSearchTool(fileService)
	if err != nil {
		log.Printf("AIOpsService: 知识检索工具创建失败 %v", err)
		return nil
	}

	maxTokens := cfg.GetConfig().Llm.MaxTokens
	chatModel, err := openai.NewChatModel(context.Background(), &openai.ChatModelConfig{
		Model:              cfg.GetConfig().Llm.Model,
		APIKey:             cfg.GetConfig().Llm.ApiKey,
		BaseURL:            cfg.GetConfig().Llm.BaseUrl,
		MaxCompletionTokens: &maxTokens,
	})
	if err != nil {
		log.Printf("AIOpsService: ChatModel创建失败 %v", err)
		return nil
	}

	tools_ := []tool.BaseTool{healthCheckTool, logAnalyzerTool, knowledgeTool}
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
		store:       convStore,
		fileService: fileService,
		toolModel:   toolModel,
		tools:       tools_,
	}
}

func (s *AIOpsService) Diagnose(ctx context.Context,
	sessionID string, message string) (*adk.AsyncIterator[*adk.AgentEvent], error) {

	history, err := s.store.LoadHistory(sessionID)
	if err != nil {
		log.Printf("AIOpsService: 加载历史失败 %v", err)
		history = nil
	}

	messages := []*schema.Message{schema.SystemMessage(aiopsInstruction)}
	messages = append(messages, history...)
	messages = append(messages, schema.UserMessage(message))

	iterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go s.runDiagnosis(ctx, messages, generator)
	return iterator, nil
}

func (s *AIOpsService) runDiagnosis(
	ctx context.Context,
	messages []*schema.Message,
	gen *adk.AsyncGenerator[*adk.AgentEvent],
) {
	defer gen.Close()

	const maxTurns = 5
	for turn := 0; turn < maxTurns; turn++ {
		log.Printf("[AIOps Turn %d] 调用 LLM...", turn)

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
			result, toolErr := s.executeTool(ctx, tc)
			if toolErr != nil {
				gen.Send(&adk.AgentEvent{Err: toolErr})
				return
			}
			log.Printf("[AIOps Turn %d] 工具 %s 返回 %d 字符", turn, tc.Function.Name, len(result))
			msg := schema.ToolMessage(result, tc.ID, schema.WithToolName(tc.Function.Name))
			messages = append(messages, msg)
		}
	}

	// 达到最大轮次仍未结束，强制生成诊断报告
	log.Printf("[AIOps Fallback] 模型 %d 轮均调用工具，强制要求生成报告", maxTurns)
	messages = append(messages, schema.UserMessage("你已收集到足够的信息。现在请直接输出最终诊断报告，不要再调用任何工具。"))
	stream, err := s.toolModel.Stream(ctx, messages)
	if err != nil {
		gen.Send(&adk.AgentEvent{Err: err})
		return
	}
	gen.Send(adk.EventFromMessage(nil, stream, schema.Assistant, ""))
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
