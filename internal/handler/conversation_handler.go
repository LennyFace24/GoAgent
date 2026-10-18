package handler

import (
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"log"

	"github.com/LennyFace24/MiniAgent/internal/permission"
	"github.com/LennyFace24/MiniAgent/internal/store"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type ConversationHandler struct {
	store *store.ConversationStore
	perms *permission.PermissionManager
}

func NewConversationHandler(convStore *store.ConversationStore, perms *permission.PermissionManager) *ConversationHandler {
	return &ConversationHandler{store: convStore, perms: perms}
}
func (h *ConversationHandler) GetConversations(c *gin.Context) {
	sessionID, ok := sessions.Default(c).Get("session_id").(string)
	if !ok || sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "会话 ID 获取失败"})
		return
	}

	conversations, err := h.store.ListConversations(sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取对话列表失败"})
		return
	}
	log.Printf("会话 %s 的对话列表: %+v", sessionID, conversations)
	c.JSON(http.StatusOK, gin.H{"conversations": conversations})
}

func (h *ConversationHandler) CreateConversation(c *gin.Context) {
	sessionID, ok := sessions.Default(c).Get("session_id").(string)
	if !ok || sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "会话 ID 获取失败"})
		return
	}

	var req struct {
		Title string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Title = "新对话"
	}

	meta, err := h.store.CreateConversation(sessionID, req.Title)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建对话失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"conversation": meta})
}

func (h *ConversationHandler) GetConversation(c *gin.Context) {
	sessionID, ok := sessions.Default(c).Get("session_id").(string)
	if !ok || sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "会话 ID 获取失败"})
		return
	}

	conversationID := c.Param("id")
	if conversationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "对话 ID 不能为空"})
		return
	}

	messages, err := h.store.LoadHistory(sessionID, conversationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取对话历史失败"})
		return
	}
	log.Printf("会话 %s 的消息记录是：%+v", sessionID, messages)
	c.JSON(http.StatusOK, gin.H{"messages": messages})
}

// RenameConversation 修改当前会话下指定对话的标题。
func (h *ConversationHandler) RenameConversation(c *gin.Context) {
	sessionID, ok := sessions.Default(c).Get("session_id").(string)
	if !ok || sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "会话 ID 获取失败"})
		return
	}

	conversationID := c.Param("id")
	if conversationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "对话 ID 不能为空"})
		return
	}

	var req struct {
		Title string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体格式错误"})
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "对话名称不能为空"})
		return
	}
	if utf8.RuneCountInString(title) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "对话名称不能超过 100 个字符"})
		return
	}

	meta, err := h.store.RenameConversation(sessionID, conversationID, title)
	if err != nil {
		if errors.Is(err, store.ErrConversationNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "对话不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "修改对话名称失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"conversation": meta})
}

func (h *ConversationHandler) DeleteConversation(c *gin.Context) {
	sessionID, ok := sessions.Default(c).Get("session_id").(string)
	if !ok || sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "会话 ID 获取失败"})
		return
	}

	conversationID := c.Param("id")
	if conversationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "对话 ID 不能为空"})
		return
	}

	if err := h.store.DeleteConversation(sessionID, conversationID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除对话失败"})
		return
	}
	h.perms.ClearScope(permission.ScopeKey(sessionID, conversationID))

	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}
