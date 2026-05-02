package handler

import (
	"github.com/LennyFace24/MiniAgent/internal/service"
	"github.com/gin-gonic/gin"
)

type FileHandler struct {
	fileService *service.FileService
}

func NewFileHandler(fileService *service.FileService) *FileHandler {
	return &FileHandler{
		fileService: fileService,
	}
}

func (h *FileHandler) UploadFile(c *gin.Context) {
	// 获取上传的文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{
			"error": "No file uploaded",
		})
		return
	}
	defer file.Close()

	// 处理文件
	if err := h.fileService.ProcessFile(c.Request.Context(), file, header.Filename); err != nil {
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "File uploaded and processed successfully",
		"file":    header.Filename,
	})
}

func (h *FileHandler) Search(c *gin.Context) {
	var req struct {
		Query string `json:"query" binding:"required"`
		TopK  int    `json:"top_k,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{
			"error": err.Error(),
		})
		return
	}

	if req.TopK == 0 {
		req.TopK = 5
	}

	results, err := h.fileService.Search(c.Request.Context(), req.Query, req.TopK)
	if err != nil {
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"status":  "ok",
		"results": results,
	})
}
