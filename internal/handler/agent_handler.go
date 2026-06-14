package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/LennyFace24/MiniAgent/internal/permission"
	"github.com/LennyFace24/MiniAgent/internal/service"
	"github.com/cloudwego/eino/schema"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)




type AgentHandler struct {
	service *service.AgentService
}

func NewAgentHandler(s *service.AgentService) *AgentHandler {
	return &AgentHandler{service: s}
}

// Stream 处理统一的流式 Agent 请求，根据 mode 路由到不同 prompt
func (h *AgentHandler) Stream(c *gin.Context) {
	mode := c.GetString("agent_mode")

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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 300*time.Second)
	defer cancel()

	sessionID := sessions.Default(c).Get("session_id").(string)
	conversationID := req.ConversationID
	ctx = context.WithValue(ctx, "session_id", sessionID)
	ctx = context.WithValue(ctx, "conversation_id", conversationID)

	if conversationID == "" {
		conversationID = "default"
	}

	iter, toolEvents, err := h.service.Stream(ctx, mode, sessionID, conversationID, req.Message)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")

	// 写 SSE 事件的辅助函数
	writeSSE := func(event string, data string) {
		fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, data)
		c.Writer.Flush()
	}

	var fullReply strings.Builder
	var toolMsgs []*schema.Message

	go func() {
		for ev := range toolEvents {
			log.Printf("[SSE] tool event: type=%s name=%s requestID=%s", ev.Type, ev.Name, ev.RequestID)
			switch ev.Type {
			case "thinking_delta":
				writeSSE("thinking_delta", fmt.Sprintf(`{"content":%q}`, ev.Content))
			case "thinking_done":
				writeSSE("thinking_done", "{}")
			case "tool_call":
				data, _ := json.Marshal(ev)
				writeSSE("tool", string(data))
				toolMsgs = append(toolMsgs, &schema.Message{
					Role: schema.Assistant,
					ToolCalls: []schema.ToolCall{{
						ID:   ev.CallID,
						Type: "function",
						Function: schema.FunctionCall{
							Name:      ev.Name,
							Arguments: ev.Args,
						},
					}},
				})
			case "tool_result":
				data, _ := json.Marshal(ev)
				writeSSE("tool", string(data))
				toolMsgs = append(toolMsgs, schema.ToolMessage(ev.Result, ev.CallID, schema.WithToolName(ev.Name)))
			default:
				data, _ := json.Marshal(ev)
				writeSSE("tool", string(data))
			}
		}
	}()

	defer func() {
		writeSSE("done", `"done"`)
	}()

	for {
		select {
		case <-ctx.Done():
			log.Printf("AgentStream(mode=%s): 客户端断开或超时", mode)
			return
		default:
		}

		event, ok := iter.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			writeSSE("error", fmt.Sprintf(`{"error":%q}`, event.Err.Error()))
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
				writeSSE("delta", fmt.Sprintf(`{"content":%q}`, delta))
			}
		} else if mv.Message != nil && mv.Message.Content != "" {
			fullReply.WriteString(mv.Message.Content)
			writeSSE("message", fmt.Sprintf(`{"content":%q}`, mv.Message.Content))
		}
	}



	// 持久化对话
	var convMessages []*schema.Message
	convMessages = append(convMessages, schema.UserMessage(req.Message))
	convMessages = append(convMessages, toolMsgs...)
	convMessages = append(convMessages, schema.AssistantMessage(fullReply.String(), nil))
	h.service.SaveReply(ctx, sessionID, conversationID, convMessages)
}

// PermissionResponse 处理前端的权限确认响应
func (h *AgentHandler) PermissionResponse(c *gin.Context) {
	var req struct {
		ID       string `json:"id"`
		Approved bool   `json:"approved"`
		Always   bool   `json:"always"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	permission.HandleResponse(req.ID, req.Approved)

	if req.Always && req.Approved {
		log.Printf("PermissionResponse: 用户选择始终允许 request=%s", req.ID)
	}

	c.JSON(200, gin.H{"ok": true})
}
