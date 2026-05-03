package service

import (
	"context"
	"fmt"
	"log"

	"github.com/LennyFace24/MiniAgent/internal/config"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino/components/tool"
	"github.com/LennyFace24/MiniAgent/internal/tools"
	"github.com/LennyFace24/MiniAgent/internal/store"
)

type ChatService struct {
	toolModel model.ToolCallingChatModel
	store *store.ConversationStore
	tools []tool.BaseTool
}

func NewChatService(fileService *FileService, convStore *store.ConversationStore) *ChatService {

	ctx := context.Background()
	cfg := config.GetConfig()
	maxTokens := cfg.Llm.MaxTokens
	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		Model:              cfg.Llm.Model,
		APIKey:             cfg.Llm.ApiKey,
		BaseURL:            cfg.Llm.BaseUrl,
		MaxCompletionTokens: &maxTokens,
	})
	if err != nil {
		log.Printf("ChatService: ChatModel创建失败 %v", err)
		return nil
	}

	knowledgeTool, err := tools.NewKnowledgeSearchTool(fileService)
	if err != nil {
		log.Printf("ChatStreamService: 知识检索工具创建失败 %v", err)
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
		log.Printf("ChatService: 绑定工具失败 %v", err)
		return nil
	}


	return &ChatService{
		toolModel: toolModel,
		store: convStore,
		tools: tools_,
	}
}

func (s *ChatService) Chat(ctx context.Context, sessionID string, message string) string {
	history,err := s.store.LoadHistory(sessionID)
	if err != nil {
		log.Printf("ChatService: 加载历史记录失败 %v", err)
	}

	messages := []*schema.Message{schema.SystemMessage(instruction)}
	messages = append(messages,history...)
	messages = append(messages, schema.UserMessage(message))

	resp,err := s.runConversation(ctx,messages)
	if err != nil {
		log.Printf("ChatService: 生成回复失败 %v", err)
		return "生成回复时出错了。"
	}
	return resp
}


func (s *ChatService) runConversation(ctx context.Context, messages []*schema.Message) (string, error) {
	for i := 0;i < 3;i++{
		resp, err := s.toolModel.Generate(ctx, messages)
		if err != nil {
			log.Printf("ChatServiceRunConversation: 生成回复失败 %v", err)
			return "", err
		}
		if resp.ToolCalls == nil || len(resp.ToolCalls) == 0 {
			log.Printf("ChatServiceRunConversation: 没有工具调用，直接回复用户")
			return resp.Content, nil
		}

		messages = append(messages, resp)
		for _, tc := range resp.ToolCalls {
			res,err := s.executeTool(ctx, tc)
			if err != nil {
				log.Printf("ChatServiceRunConversation: 执行工具失败 %v", err)
				return "", err
			}
			messages = append(messages, schema.ToolMessage(res,tc.ID,schema.WithToolName(tc.Function.Name)))
		}
	}
	messages = append(messages, schema.UserMessage("你已调用足够多次工具，现在必须直接回答用户的问题。不要再调用任何工具。"))
	resp, err := s.toolModel.Generate(ctx, messages)
	if err != nil {
		log.Printf("ChatServiceRunConversation: 最终生成回复失败 %v", err)
		return "", err
	}
	return resp.Content, nil
}

func (s *ChatService) executeTool(ctx context.Context, tc schema.ToolCall) (string, error) {
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

func (s *ChatService) SaveReply(ctx context.Context, sessionID string, userMsg string, aiMsg string) {
	if err := s.store.SaveMessages(sessionID,
		schema.UserMessage(userMsg),
		schema.AssistantMessage(aiMsg, nil),
	); err != nil {
		log.Printf("ChatStreamService: 保存对话失败 %v", err)
	}
}
