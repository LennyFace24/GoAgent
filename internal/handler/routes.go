package handler

import (
	"github.com/LennyFace24/MiniAgent/internal/config"
	"github.com/LennyFace24/MiniAgent/internal/service"
	"github.com/LennyFace24/MiniAgent/internal/store"
	"github.com/LennyFace24/MiniAgent/internal/tools"
	"github.com/gin-gonic/gin"
)

var (
	agentHandler        *AgentHandler
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

	agentService := service.NewAgentService(toolsHandler, convStore)
	agentHandler = NewAgentHandler(agentService)
	fileHandler = NewFileHandler(fileService)
	conversationHandler = NewConversationHandler(convStore)
	metricsHandler = NewMetricsHandler(config.GetConfig().Prometheus.URL)
}

func SetupRoutes(r *gin.Engine) {
	api := r.Group("/api")
	{
		// 统一 Agent 端点，通过中间件注入 mode
		api.POST("/chat_stream", setAgentMode("chat"), agentHandler.Stream)
		api.POST("/ai_ops", setAgentMode("aiops"), agentHandler.Stream)
		api.POST("/permission/response", agentHandler.PermissionResponse)

		api.POST("/upload_file", fileHandler.UploadFile)
		api.POST("/search", fileHandler.Search)
		api.GET("/metrics", metricsHandler.GetMetrics)
		api.GET("/conversations", conversationHandler.GetConversations)
		api.POST("/conversation", conversationHandler.CreateConversation)
		api.GET("/conversation/:id", conversationHandler.GetConversation)
		api.DELETE("/conversation/:id", conversationHandler.DeleteConversation)
	}
}

// setAgentMode 中间件：在 gin.Context 中注入 agent 模式
func setAgentMode(mode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("agent_mode", mode)
		c.Next()
	}
}
