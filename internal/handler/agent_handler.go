package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
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
	var writeMu sync.Mutex
	writeSSE := func(event string, data string) {
		writeMu.Lock()
		defer writeMu.Unlock()
		fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, data)
		c.Writer.Flush()
	}

	var fullReply strings.Builder
	var toolMsgs []*schema.Message

	toolDone := make(chan struct{})
	go func() {
		defer close(toolDone)
		for ev := range toolEvents {
			log.Printf("[SSE] tool event: type=%s name=%s requestID=%s", ev.Type, ev.Name, ev.RequestID)
			switch ev.Type {
			case "thinking_delta":
				writeSSE("thinking_delta", fmt.Sprintf(`{"content":%q}`, ev.Content))
			case "thinking_done":
				writeSSE("thinking_done", "{}")
			case "delta":
				writeSSE("delta", fmt.Sprintf(`{"content":%q}`, ev.Content))
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

	// persist 落库已收集的对话；terminated=true 时在末尾追加终止标记。
	// 用 context.Background() 是因为 ctx.Done 分支下原 ctx 已取消，落库不应被取消。
	persist := func(terminated bool) {
		<-toolDone // 等待 toolEvents goroutine 结束，确保 toolMsgs 完整
		if fullReply.Len() == 0 && len(toolMsgs) == 0 {
			return
		}
		reply := fullReply.String()
		if terminated && reply != "" {
			reply += "\n\n该回答此处被用户终止"
		}
		var convMessages []*schema.Message
		convMessages = append(convMessages, schema.UserMessage(req.Message))
		convMessages = append(convMessages, toolMsgs...)
		if reply != "" {
			convMessages = append(convMessages, schema.AssistantMessage(reply, nil))
		}
		h.service.SaveReply(context.Background(), sessionID, conversationID, convMessages)
	}

	defer func() {
		writeSSE("done", `"done"`)
	}()

	for {
		select {
		case <-ctx.Done():
			log.Printf("AgentStream(mode=%s): 客户端断开或超时", mode)
			persist(true)
			return
		default:
		}

		event, ok := iter.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			writeSSE("error", fmt.Sprintf(`{"error":%q}`, event.Err.Error()))
			persist(false)
			return
		}

		mv := event.Output.MessageOutput
		if mv == nil {
			continue
		}

		if mv.IsStreaming && mv.MessageStream != nil {
			// delta 事件已由 runLoop 通过 toolEvents 发送，这里只收集内容用于持久化
			for {
				chunk, err := mv.MessageStream.Recv()
				if err != nil {
					break
				}
				if chunk.Content != "" {
					fullReply.WriteString(chunk.Content)
				}
			}
		} else if mv.Message != nil && mv.Message.Content != "" {
			fullReply.WriteString(mv.Message.Content)
			// 非流式消息仍需发送
			writeSSE("message", fmt.Sprintf(`{"content":%q}`, mv.Message.Content))
		}

	}

	// 正常结束：持久化完整对话
	persist(false)
}

// PermissionResponse 处理前端的权限确认响应
func (h *AgentHandler) PermissionResponse(c *gin.Context) {
	var req struct {
		ID             string `json:"id"`
		ConversationID string `json:"conversation_id"`
		Approved       bool   `json:"approved"`
		Always         bool   `json:"always"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if req.ID == "" {
		c.JSON(400, gin.H{"error": "id is required"})
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

	handled, toolName := permission.HandleResponse(req.ID, sessionID, conversationID, req.Approved)
	if !handled {
		c.JSON(404, gin.H{"error": "permission request not found"})
		return
	}

	if req.Always && req.Approved && toolName != "" {
		// "始终允许"：写入内存规则 + 持久化到 permission.json
		h.service.PermissionManager().AddRule(permission.Rule{Tool: toolName, Behavior: permission.BehaviorAllow})
		if err := permission.PersistAlwaysAllow("permission.json", toolName); err != nil {
			log.Printf("PermissionResponse: 持久化 always 规则失败 %v", err)
		} else {
			log.Printf("PermissionResponse: 用户选择始终允许 tool=%s", toolName)
		}
	}

	c.JSON(200, gin.H{"ok": true})
}
