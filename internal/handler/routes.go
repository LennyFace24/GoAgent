package handler

import (
	"github.com/LennyFace24/MiniAgent/internal/config"
	"github.com/LennyFace24/MiniAgent/internal/service"
	"github.com/LennyFace24/MiniAgent/internal/store"
	"github.com/LennyFace24/MiniAgent/internal/tools"
	"github.com/gin-gonic/gin"
)

var (
	chatStreamHandler   *ChatStreamHandler
	aiopsHandler        *AIOpsHandler
	fileHandler         *FileHandler
	toolsHandler        *tools.ToolHandler
	conversationHandler *ConversationHandler
	metricsHandler      *MetricsHandler

	convStore *store.ConversationStore
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

	chatStreamHandler = NewChatStreamHandler(
		service.NewChatStreamService(toolsHandler, convStore))
	aiopsHandler = NewAIOpsHandler(
		service.NewAIOpsService(toolsHandler, convStore))
	fileHandler = NewFileHandler(fileService)
	conversationHandler = NewConversationHandler(convStore)
	metricsHandler = NewMetricsHandler(config.GetConfig().Prometheus.URL)
}

func SetupRoutes(r *gin.Engine) {
	api := r.Group("/api")
	{
		api.POST("/chat_stream", chatStreamHandler.ChatStream)
		api.POST("/upload_file", fileHandler.UploadFile)
		api.POST("/search", fileHandler.Search)
		api.POST("/ai_ops", aiopsHandler.Diagnose)
		api.GET("/metrics", metricsHandler.GetMetrics)
		api.POST("/permission/response", chatStreamHandler.PermissionResponse)
		api.GET("/conversations", conversationHandler.GetConversations)
		api.POST("/conversation", conversationHandler.CreateConversation)
		api.GET("/conversation/:id", conversationHandler.GetConversation)
		api.DELETE("/conversation/:id", conversationHandler.DeleteConversation)
	}
}
