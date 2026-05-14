package handler

import (
	"context"
	"github.com/LennyFace24/MiniAgent/internal/service"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"log"
	"strings"
	"time"
)

type ChatStreamHandler struct {
	service *service.ChatStreamService
}

func NewChatStreamHandler(s *service.ChatStreamService) *ChatStreamHandler {
	return &ChatStreamHandler{
		service: s,
	}
}

func (h *ChatStreamHandler) ChatStream(c *gin.Context) {
	var req struct {
		Message        string `json:"message"`
		ConversationID string `json:"conversation_id"`
	}
	// 解析json
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{
			"error": err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 300*time.Second)
	defer cancel()

	sessionID := sessions.Default(c).Get("session_id").(string)
	// 将session_id conversationId注入到上下文中
	conversationID := req.ConversationID
	ctx = context.WithValue(ctx, "session_id", sessionID)
	ctx = context.WithValue(ctx, "conversation_id", conversationID)
	
	if conversationID == "" {
		conversationID = "default"
	}

	iter, err := h.service.ChatStream(ctx,
		sessionID, conversationID, req.Message)
	if err != nil {
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}
	// 改为SSE连接
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")

	// 完整的回复内容，用于后续保存到历史记录
	var fullReply strings.Builder

	defer func() {
		c.SSEvent("done", "")
		c.Writer.Flush()
	}()

	for {
		select {
			case <-ctx.Done():
				log.Println("ChatStream: 客户端断开或超时")
				return
			default:
		}

		event, ok := iter.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			c.SSEvent("error", gin.H{"error": event.Err.Error()})
			c.Writer.Flush()
			return
		}

		mv := event.Output.MessageOutput
		if mv == nil {
			continue
		}

		if mv.IsStreaming && mv.MessageStream != nil {
			for {
				chunk, err := mv.MessageStream.Recv()
				if err != nil {
					break
				}
				delta := chunk.Content
				if delta == "" {
					continue
				}
				fullReply.WriteString(delta)
				c.SSEvent("delta", gin.H{"content": delta})
				c.Writer.Flush()
			}
		} else if mv.Message != nil && mv.Message.Content != "" {
			fullReply.WriteString(mv.Message.Content)
			c.SSEvent("message", gin.H{"content": mv.Message.Content})
			c.Writer.Flush()
		}
	}

	if fullReply.Len() > 0 {
		h.service.SaveReply(ctx, sessionID, conversationID, req.Message, fullReply.String())
	} else {
		log.Println("ChatStream: received empty reply")
	}
}
