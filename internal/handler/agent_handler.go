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
				fullReply.WriteString(ev.Content)
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

	// 正文和工具消息都由事件协程收集，等它结束后才能读取并落库。
	// 取消仅停止生成，不丢弃已接收的正文；历史将在下一次请求中载入。
	persist := func() {
		<-toolDone
		if fullReply.Len() == 0 && len(toolMsgs) == 0 {
			return
		}
		reply := fullReply.String()
		if ctx.Err() != nil && reply != "" {
			reply += "\n\n[用户在此处停止回答]"
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
		persist()
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
	}
}

// PermissionResponse 处理前端的权限确认响应
func (h *AgentHandler) PermissionResponse(c *gin.Context) {
	var req struct {
		ID             string `json:"id"`
		ConversationID string `json:"conversation_id"`
		Approved       bool   `json:"approved"`
		Always         bool   `json:"always"`
		Global         bool   `json:"global"`
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

	handled, approval := permission.HandleResponse(req.ID, sessionID, conversationID, req.Approved, req.Always, req.Global)
	if !handled {
		c.JSON(404, gin.H{"error": "permission request not found"})
		return
	}

	// 当前对话范围的规则由工具循环在拿到答复后安装；这里只负责全局规则的落盘。
	if req.Always && req.Approved && req.Global {
		rule := permission.RuleForInvocation(approval.ToolName, approval.ToolInput)
		if err := permission.PersistAlwaysAllow("permission.json", rule); err != nil {
			log.Printf("PermissionResponse: 持久化 always 规则失败 %v", err)
		} else {
			log.Printf("PermissionResponse: 用户选择全局始终允许 tool=%s", approval.ToolName)
		}
	}

	c.JSON(200, gin.H{"ok": true})
}
