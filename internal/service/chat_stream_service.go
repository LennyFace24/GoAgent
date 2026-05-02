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

type ChatStreamService struct {
	store       *store.ConversationStore
	fileService *FileService
	toolModel   model.ToolCallingChatModel
	tools       []tool.BaseTool
}

func NewChatStreamService(fileService *FileService, convStore *store.ConversationStore) *ChatStreamService {
	knowledgeTool, err := tools.NewKnowledgeSearchTool(fileService)
	if err != nil {
		log.Printf("ChatStreamService: 知识检索工具创建失败 %v", err)
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
		log.Printf("ChatStreamService: ChatModel创建失败 %v", err)
		return nil
	}

	tools_ := []tool.BaseTool{knowledgeTool}
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
		store:       convStore,
		fileService: fileService,
		toolModel:   toolModel,
		tools:       tools_,
	}
}

func (s *ChatStreamService) ChatStream(ctx context.Context,
	sessionID string, userMsg string) (*adk.AsyncIterator[*adk.AgentEvent], error) {

	history, err := s.store.LoadHistory(sessionID)
	if err != nil {
		log.Printf("ChatStreamService: 加载历史失败 %v", err)
		history = nil
	}

	messages := []*schema.Message{schema.SystemMessage(instruction)}
	messages = append(messages, history...)
	messages = append(messages, schema.UserMessage(userMsg))

	iterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go s.runConversation(ctx, messages, generator)
	return iterator, nil
}

func (s *ChatStreamService) runConversation(
	ctx context.Context,
	messages []*schema.Message,
	gen *adk.AsyncGenerator[*adk.AgentEvent],
) {
	defer gen.Close()

	const maxTurns = 3
	for turn := 0; turn < maxTurns; turn++ {
		log.Printf("[Turn %d] 调用 LLM...", turn)

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

		if len(fullMsg.ToolCalls) == 0 {
			log.Printf("[Turn %d] 无 ToolCall，返回最终回答", turn)
			return
		}

		messages = append(messages, fullMsg)
		for _, tc := range fullMsg.ToolCalls {
			result, toolErr := s.executeTool(ctx, tc)
			if toolErr != nil {
				gen.Send(&adk.AgentEvent{Err: toolErr})
				return
			}
			log.Printf("[Turn %d] 工具 %s 返回 %d 字符", turn, tc.Function.Name, len(result))
			msg := schema.ToolMessage(result, tc.ID, schema.WithToolName(tc.Function.Name))
			messages = append(messages, msg)
		}

	}

	// 达到最大轮次仍未回答，强制最后一轮
	log.Printf("[Fallback] 模型 %d 轮均调用工具，强制要求回答", maxTurns)
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

func (s *ChatStreamService) SaveReply(ctx context.Context, sessionID, userMsg, aiMsg string) {
	if err := s.store.SaveMessages(sessionID,
		schema.UserMessage(userMsg),
		schema.AssistantMessage(aiMsg, nil),
	); err != nil {
		log.Printf("ChatStreamService: 保存对话失败 %v", err)
	}
}
