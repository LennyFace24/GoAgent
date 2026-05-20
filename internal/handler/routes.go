package handler

import (
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
}

func SetupRoutes(r *gin.Engine) {
	// 聊天接口
	r.POST("/chat_stream", chatStreamHandler.ChatStream)
	// 上传文件接口
	r.POST("/upload_file", fileHandler.UploadFile)
	// 搜索文件接口
	r.POST("/search", fileHandler.Search)
	// AIOps接口
	r.POST("/ai_ops", aiopsHandler.Diagnose)
	// 权限确认接口
	r.POST("/permission/response", chatStreamHandler.PermissionResponse)
	// 对话管理接口
	r.GET("/conversations", conversationHandler.GetConversations)
	r.POST("/conversation", conversationHandler.CreateConversation)
	r.GET("/conversation/:id", conversationHandler.GetConversation)
	r.DELETE("/conversation/:id", conversationHandler.DeleteConversation)

}
