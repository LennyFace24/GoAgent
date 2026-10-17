package handler

import (
	"strings"

	"github.com/LennyFace24/MiniAgent/internal/commands"
	"github.com/LennyFace24/MiniAgent/internal/store"
	"github.com/cloudwego/eino/schema"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// CommandHandler 处理斜杠命令执行
type CommandHandler struct {
	store *store.ConversationStore
}

func NewCommandHandler(convStore *store.ConversationStore) *CommandHandler {
	return &CommandHandler{store: convStore}
}

type executeRequest struct {
	Name           string   `json:"name"`
	Args           []string `json:"args"`
	ConversationID string   `json:"conversation_id"`
}

// Execute 执行斜杠命令：
// - 命令原文（/name args...）落库为 user 消息（需求：命令消息本身进上下文）
// - builtin 命令：执行 handler，结果落库为 assistant 消息，返回 {type:"builtin", result}
// - file-based 命令：渲染模板，返回 {type:"file", rendered}，前端将渲染文本作为普通消息发出
func (h *CommandHandler) Execute(c *gin.Context) {
	var req executeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.Name == "" {
		c.JSON(400, gin.H{"error": "name is required"})
		return
	}

	sessionID, ok := sessions.Default(c).Get("session_id").(string)
	if !ok || sessionID == "" {
		c.JSON(401, gin.H{"error": "session is required"})
		return
	}
	conversationID := req.ConversationID
	if conversationID == "" {
		conversationID = "default"
	}

	cmd, found := commands.GetRegistry().Find(req.Name)
	if !found {
		c.JSON(404, gin.H{"error": "command not found: " + req.Name})
		return
	}

	// required 命令缺参：返回用法提示
	if cmd.ArgsPolicy == commands.ArgsRequired && len(req.Args) == 0 {
		hint := cmd.ArgsHint
		if hint == "" {
			hint = "该命令需要参数"
		}
		c.JSON(400, gin.H{"error": hint})
		return
	}

	// 落库 user 原文（命令消息进上下文）
	raw := "/" + req.Name
	if len(req.Args) > 0 {
		raw += " " + strings.Join(req.Args, " ")
	}
	_ = h.store.SaveMessages(sessionID, conversationID, schema.UserMessage(raw))

	if cmd.Handler == nil {
		// client 命令不应到达后端
		c.JSON(400, gin.H{"error": "client command cannot be executed server-side"})
		return
	}

	result, err := cmd.Handler(c.Request.Context(), req.Args)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	if cmd.Source == "builtin" {
		// builtin 命令结果落库为 assistant 消息
		_ = h.store.SaveMessages(sessionID, conversationID, schema.AssistantMessage(result, nil))
		c.JSON(200, gin.H{"type": "builtin", "result": result})
		return
	}

	// file-based 命令：渲染文本由前端作为普通消息发出（AI 回复走流式落库）
	c.JSON(200, gin.H{"type": "file", "rendered": result})
}
