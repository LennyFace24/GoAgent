package handler

import (
	"net/http"

	"log"

	"github.com/LennyFace24/MiniAgent/internal/store"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type ConversationHandler struct {
	store *store.ConversationStore
}

func NewConversationHandler(convStore *store.ConversationStore) *ConversationHandler {
	return &ConversationHandler{store: convStore}
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

	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}
