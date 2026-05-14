package service

import (
	"context"
	"fmt"
	"log"

	"github.com/LennyFace24/MiniAgent/internal/config"
	skills_registry "github.com/LennyFace24/MiniAgent/internal/skills/registry"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino/components/tool"
	"github.com/LennyFace24/MiniAgent/internal/tools"
	"github.com/LennyFace24/MiniAgent/internal/store"
)

type ChatService struct {
	toolModel model.ToolCallingChatModel
	store     *store.ConversationStore
	tools     []tool.BaseTool
	skills    *skills_registry.SkillRegistry
}

func NewChatService(toolHandler *tools.ToolHandler, convStore *store.ConversationStore, skills *skills_registry.SkillRegistry) *ChatService {
	ctx := context.Background()
	cfg := config.GetConfig()
	maxTokens := cfg.Llm.MaxTokens
	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		Model:               cfg.Llm.Model,
		APIKey:              cfg.Llm.ApiKey,
		BaseURL:             cfg.Llm.BaseUrl,
		MaxCompletionTokens: &maxTokens,
	})
	if err != nil {
		log.Printf("ChatService: ChatModel创建失败 %v", err)
		return nil
	}

	tools_ := toolHandler.Tools()
	toolInfos := make([]*schema.ToolInfo, len(tools_))
	for i, t := range tools_ {
		info, infoErr := t.Info(context.Background())
		if infoErr != nil {
			log.Printf("ChatService: 获取工具信息失败 %v", infoErr)
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
		store:     convStore,
		tools:     tools_,
		skills:    skills,
	}
}

func (s *ChatService) Chat(ctx context.Context, sessionID string, conversationID string, message string) string {
	history,err := s.store.LoadHistory(sessionID, conversationID)
	if err != nil {
		log.Printf("ChatService: 加载历史记录失败 %v", err)
	}

	systemPrompt := instruction
	if desc := s.skills.DescribeAvailable(); desc != "" {
		systemPrompt += "\n\n# 已安装的 Agent Skills（技能模块）\n" + desc + "\n以上是系统预装的技能模块，不是你的通用能力。当用户提到某个技能相关的需求时，调用 skill 工具（传入技能名称）来加载该技能的完整规则，然后按规则执行。"
	}
	messages := []*schema.Message{schema.SystemMessage(systemPrompt)}
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

func (s *ChatService) SaveReply(ctx context.Context, sessionID string, conversationID string, userMsg string, aiMsg string) {
	if err := s.store.SaveMessages(sessionID, conversationID,
		schema.UserMessage(userMsg),
		schema.AssistantMessage(aiMsg, nil),
	); err != nil {
		log.Printf("ChatStreamService: 保存对话失败 %v", err)
	}
}
