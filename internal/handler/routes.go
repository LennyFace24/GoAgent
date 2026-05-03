package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/LennyFace24/MiniAgent/internal/service"
	"github.com/LennyFace24/MiniAgent/internal/store"
)


var (
	chatHandler       *ChatHandler
	chatStreamHandler *ChatStreamHandler
	fileHandler       *FileHandler
)

func SetupHandler(r *gin.Engine) {
	convStore, err := store.NewConversationStore("data/conversations")
	if err != nil {
		panic("初始化对话存储失败: " + err.Error())
	}
    chatHandler = NewChatHandler(service.NewChatService(
		service.NewFileService(),
		convStore,
	))
	
	chatStreamHandler = NewChatStreamHandler(service.NewChatStreamService(
		service.NewFileService(),
		convStore,
	))
	fileHandler = NewFileHandler(service.NewFileService())
}


func SetupRoutes(r *gin.Engine) {
	// 设置路由
	r.POST("/chat", chatHandler.Chat)
	r.POST("/chat_stream", chatStreamHandler.ChatStream)
	r.POST("/upload_file", fileHandler.UploadFile)
	r.POST("/search", fileHandler.Search)
}