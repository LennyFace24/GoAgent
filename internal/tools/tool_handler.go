package tools

import (
	"fmt"

	"github.com/LennyFace24/MiniAgent/internal/config"
	"github.com/LennyFace24/MiniAgent/internal/tools/basic_tool"
	"github.com/LennyFace24/MiniAgent/internal/tools/context_tool"
	"github.com/LennyFace24/MiniAgent/internal/tools/extra_tool"
	"github.com/LennyFace24/MiniAgent/internal/tools/skill_tool"
	"github.com/cloudwego/eino/components/tool"
)

type ToolHandler struct {
	tools        []tool.BaseTool
	CompactTrigger *contexttool.CompactTrigger
}

func NewToolHandler(fileSearcher extratool.FileSearcher, skillLoader skilltool.SkillLoader) (*ToolHandler, error) {
	cfg := config.GetConfig()

	bashTool, err := basictool.NewBashTool()
	if err != nil {
		return nil, fmt.Errorf("创建 bash 工具失败: %w", err)
	}

	healthCheckTool, err := extratool.NewHealthCheckTool(cfg.Prometheus.URL)
	if err != nil {
		return nil, fmt.Errorf("创建 health_check 工具失败: %w", err)
	}

	knowledgeTool, err := extratool.NewKnowledgeSearchTool(fileSearcher)
	if err != nil {
		return nil, fmt.Errorf("创建 knowledge_search 工具失败: %w", err)
	}

	readFileTool, err := basictool.NewReadFileTool()
	if err != nil {
		return nil, fmt.Errorf("创建 read_file 工具失败: %w", err)
	}

	writeFileTool, err := basictool.NewWriteFileTool()
	if err != nil {
		return nil, fmt.Errorf("创建 write_file 工具失败: %w", err)
	}

	editFileTool, err := basictool.NewEditFileTool()
	if err != nil {
		return nil, fmt.Errorf("创建 edit_file 工具失败: %w", err)
	}
	taskTool,err := extratool.NewTaskTool()
	if err != nil {
		return nil, fmt.Errorf("创建 task 工具失败: %w", err)
	}

	skillTool, err := skilltool.NewSkillTool(skillLoader)
	if err != nil {
		return nil, fmt.Errorf("创建 skill 工具失败: %w", err)
	}

	compactTool, compactTrigger, err := contexttool.NewCompactTool()
	if err != nil {
		return nil, fmt.Errorf("创建 compact 工具失败: %w", err)
	}

	return &ToolHandler{
		tools: []tool.BaseTool{
			bashTool,
			readFileTool,
			writeFileTool,
			editFileTool,
			healthCheckTool,
			knowledgeTool,
			taskTool,
			skillTool,
			compactTool,
		},
		CompactTrigger: compactTrigger,
	}, nil
}

func (h *ToolHandler) Tools() []tool.BaseTool {
	return h.tools
}
