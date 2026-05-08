package handler

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/LennyFace24/MiniAgent/internal/service"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type AIOpsHandler struct {
	service *service.AIOpsService
}

func NewAIOpsHandler(s *service.AIOpsService) *AIOpsHandler {
	return &AIOpsHandler{service: s}
}

func (h *AIOpsHandler) Diagnose(c *gin.Context) {
	var req struct {
		Message        string `json:"message"`
		ConversationID string `json:"conversation_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if req.Message == "" {
		c.JSON(400, gin.H{"error": "message 字段不能为空"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 180*time.Second)
	defer cancel()

	sessionID := sessions.Default(c).Get("session_id").(string)
	conversationID := req.ConversationID
	if conversationID == "" {
		conversationID = "default"
	}
	iter, err := h.service.Diagnose(ctx, sessionID, conversationID, req.Message)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")

	var fullReply strings.Builder

	defer func() {
		c.SSEvent("done", "")
		c.Writer.Flush()
	}()

	for {
		select {
		case <-ctx.Done():
			log.Println("AIOps: 客户端断开或超时")
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
}