package tools

import (
	"fmt"

	"github.com/LennyFace24/MiniAgent/internal/config"
	"github.com/cloudwego/eino/components/tool"
)

type ToolHandler struct {
	tools []tool.BaseTool
}

func NewToolHandler(fileSearcher FileSearcher) (*ToolHandler, error) {
	cfg := config.GetConfig()

	bashTool, err := NewBashTool()
	if err != nil {
		return nil, fmt.Errorf("创建 bash 工具失败: %w", err)
	}

	healthCheckTool, err := NewHealthCheckTool(cfg.Prometheus.URL)
	if err != nil {
		return nil, fmt.Errorf("创建 health_check 工具失败: %w", err)
	}

	knowledgeTool, err := NewKnowledgeSearchTool(fileSearcher)
	if err != nil {
		return nil, fmt.Errorf("创建 knowledge_search 工具失败: %w", err)
	}

	readFileTool, err := NewReadFileTool()
	if err != nil {
		return nil, fmt.Errorf("创建 read_file 工具失败: %w", err)
	}

	writeFileTool, err := NewWriteFileTool()
	if err != nil {
		return nil, fmt.Errorf("创建 write_file 工具失败: %w", err)
	}

	editFileTool, err := NewEditFileTool()
	if err != nil {
		return nil, fmt.Errorf("创建 edit_file 工具失败: %w", err)
	}

	return &ToolHandler{
		tools: []tool.BaseTool{
			bashTool,
			readFileTool,
			writeFileTool,
			editFileTool,
			healthCheckTool,
			knowledgeTool,
		},
	}, nil
}

func (h *ToolHandler) Tools() []tool.BaseTool {
	return h.tools
}
