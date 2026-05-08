package handler

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/LennyFace24/MiniAgent/internal/service"
	"github.com/LennyFace24/MiniAgent/internal/store"
	"github.com/LennyFace24/MiniAgent/internal/tools"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

var (
	chatHandler       *ChatHandler
	chatStreamHandler *ChatStreamHandler
	aiopsHandler      *AIOpsHandler
	fileHandler       *FileHandler
	toolsHandler      *tools.ToolHandler
	convStore         *store.ConversationStore
)

func SetupHandler(r *gin.Engine) {
	var err error
	convStore, err = store.NewConversationStore("data/conversations")
	if err != nil {
		panic("初始化对话存储失败: " + err.Error())
	}

	fileService := service.NewFileService()

	toolsHandler, err = tools.NewToolHandler(fileService)
	if err != nil {
		panic("初始化工具处理器失败: " + err.Error())
	}

	chatHandler = NewChatHandler(
		service.NewChatService(
			toolsHandler,
			convStore,
		))
	chatStreamHandler = NewChatStreamHandler(
		service.NewChatStreamService(
			toolsHandler, 
			convStore,
			))
	aiopsHandler = NewAIOpsHandler(
		service.NewAIOpsService(
			toolsHandler, convStore,
			))
	fileHandler = NewFileHandler(
		fileService,
	)
}

func SetupRoutes(r *gin.Engine) {
	r.POST("/chat", chatHandler.Chat)
	r.POST("/chat_stream", chatStreamHandler.ChatStream)
	r.POST("/upload_file", fileHandler.UploadFile)
	r.POST("/search", fileHandler.Search)
	r.POST("/ai_ops", aiopsHandler.Diagnose)
	r.GET("/history", GetHistory)
}

type historyLine struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func GetHistory(c *gin.Context) {
	sessionID, ok := sessions.Default(c).Get("session_id").(string)
	if !ok || sessionID == "" {
		c.JSON(400, gin.H{"error": "会话 ID 获取失败"})
		return
	}

	path := filepath.Join("data/conversations", sessionID+".jsonl")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(200, []historyLine{})
			return
		}
		c.JSON(500, gin.H{"error": "读取历史失败"})
		return
	}
	defer f.Close()

	var lines []historyLine
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var line historyLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}
		lines = append(lines, line)
	}
	c.JSON(200, lines)
}