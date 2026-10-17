package handler

import (
	"github.com/LennyFace24/MiniAgent/internal/commands"
	"github.com/LennyFace24/MiniAgent/internal/metrics"
	"github.com/LennyFace24/MiniAgent/internal/modelmgr"
	"github.com/LennyFace24/MiniAgent/internal/service"
	"github.com/LennyFace24/MiniAgent/internal/store"
	"github.com/LennyFace24/MiniAgent/internal/tools"
	contexttool "github.com/LennyFace24/MiniAgent/internal/tools/context_tool"
	"github.com/gin-gonic/gin"
)

// Handlers 聚合所有 handler 依赖，支持依赖注入（测试用）
type Handlers struct {
	Agent        *AgentHandler
	File         *FileHandler
	Conversation *ConversationHandler
	Command      *CommandHandler
	Metrics      *MetricsHandler
}

// NewHandlers 以注入的依赖构建 handlers（不触碰全局状态）
func NewHandlers(convStore *store.ConversationStore, agentService *service.AgentService, fileService *service.FileService, collector metrics.Collector) *Handlers {
	return &Handlers{
		Agent:        NewAgentHandler(agentService),
		File:         NewFileHandler(fileService),
		Conversation: NewConversationHandler(convStore),
		Command:      NewCommandHandler(convStore),
		Metrics:      NewMetricsHandler(collector),
	}
}

var (
	handlers *Handlers
	convStore *store.ConversationStore
)

// SetupHandler 生产装配：初始化真实依赖（registry 由 main 从 model.json 加载）
func SetupHandler(r *gin.Engine, registry *modelmgr.Registry) {
	var err error
	convStore, err = store.NewConversationStore("data/conversations")
	if err != nil {
		panic("初始化对话存储失败: " + err.Error())
	}

	fileService := service.NewFileService()
	collector := metrics.NewCollector()

	// getModel 闭包：延迟读取 registry 默认模型（ModelSet 在 AgentService 中创建）
	getModel := func() modelmgr.Model {
		return registry.Default()
	}

	toolsHandler, err := tools.NewToolHandler(fileService, collector, getModel)
	if err != nil {
		panic("初始化工具处理器失败: " + err.Error())
	}

	agentService := service.NewAgentService(toolsHandler, convStore, registry)
	handlers = NewHandlers(convStore, agentService, fileService, collector)
}

// SetupRoutes 注册全部路由（依赖 SetupHandler 构建的 handlers）
func SetupRoutes(r *gin.Engine) {
	SetupRoutesWith(r, handlers)
}

// SetupRoutesWith 以注入的 handlers 注册路由（测试用）
func SetupRoutesWith(r *gin.Engine, hs *Handlers) {
	api := r.Group("/api")
	{
		// 统一 Agent 端点，通过中间件注入 mode
		api.POST("/chat_stream", setAgentMode("chat"), hs.Agent.Stream)
		api.POST("/ai_ops", setAgentMode("aiops"), hs.Agent.Stream)
		api.POST("/permission/response", hs.Agent.PermissionResponse)

		api.POST("/upload_file", hs.File.UploadFile)
		api.POST("/search", hs.File.Search)
		api.GET("/metrics", hs.Metrics.GetMetrics)
		api.GET("/conversations", hs.Conversation.GetConversations)
		api.POST("/conversation", hs.Conversation.CreateConversation)
		api.GET("/conversation/:id", hs.Conversation.GetConversation)
		api.DELETE("/conversation/:id", hs.Conversation.DeleteConversation)

		// 斜杠命令列表
		api.GET("/commands", getCommands)
		api.GET("/model", hs.GetModel)
		api.POST("/model", hs.SwitchModel)
		// 上下文状态查询（统一 API 前缀）
		api.GET("/context/status", getContextStatus)

		// 斜杠命令执行
		api.POST("/commands/execute", hs.Command.Execute)
	}

	// 上下文状态查询（兼容旧路径）
	r.GET("/context", getContextStatus)
}

// GetModel 返回模型列表与当前模型；model.json 加载失败时附带 error 字段
func (hs *Handlers) GetModel(c *gin.Context) {
	ms := hs.Agent.service.ModelSet()
	resp := gin.H{
		"models":  ms.ListModels(),
		"current": ms.CurrentID(),
	}
	if modelmgr.LoadError != "" {
		resp["error"] = modelmgr.LoadError
	}
	c.JSON(200, resp)
}

// getCommands 获取斜杠命令列表
func getCommands(c *gin.Context) {
	registry := commands.GetRegistry()
	cmds := registry.List()
	c.JSON(200, gin.H{"commands": cmds})
}


// SwitchModel 切换当前模型
func (hs *Handlers) SwitchModel(c *gin.Context) {
	var req struct {
		ID string `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == "" {
		c.JSON(400, gin.H{"error": "id is required"})
		return
	}
	if err := hs.Agent.service.SwitchModel(c.Request.Context(), req.ID); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"current": hs.Agent.service.ModelSet().CurrentID()})
}

// getContextStatus 获取上下文使用状态
func getContextStatus(c *gin.Context) {
	state := contexttool.GetContextState()
	usage := state.GetUsage()
	c.JSON(200, usage)
}

// setAgentMode 中间件：在 gin.Context 中注入 agent 模式
func setAgentMode(mode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("agent_mode", mode)
		c.Next()
	}
}
