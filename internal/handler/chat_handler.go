package handler

import (
	"context"

	"time"
	"github.com/LennyFace24/MiniAgent/internal/service"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type ChatHandler struct {
	chatService *service.ChatService
}

func NewChatHandler(chatService *service.ChatService) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
	}
}

func (h *ChatHandler) Chat(c *gin.Context) {
	// 这里直接调用ChatService的Chat方法，返回结果
	var req struct {
		Message string `json:"message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{
			"error": err.Error(),
		})
		return
	}
	sessionId := sessions.Default(c).Get("session_id").(string)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()
	reply := h.chatService.Chat(ctx, sessionId, req.Message)

	h.chatService.SaveReply(ctx, sessionId, req.Message, reply)
	
	c.JSON(200, gin.H{
		"reply": reply,
	})
}