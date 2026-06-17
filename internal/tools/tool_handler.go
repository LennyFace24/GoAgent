package tools

import (
	"fmt"

	"github.com/LennyFace24/MiniAgent/internal/config"
	"github.com/LennyFace24/MiniAgent/internal/metrics"
	"github.com/LennyFace24/MiniAgent/internal/task"
	"github.com/LennyFace24/MiniAgent/internal/tools/basic_tool"
	"github.com/LennyFace24/MiniAgent/internal/tools/context_tool"
	"github.com/LennyFace24/MiniAgent/internal/tools/extra_tool"
	"github.com/cloudwego/eino/components/tool"
)

type ToolHandler struct {
	tools          []tool.BaseTool
	CompactTrigger *contexttool.CompactTrigger
}

func NewToolHandler(fileSearcher extratool.FileSearcher, collector metrics.Collector) (*ToolHandler, error) {
	cfg := config.GetConfig()
	workspaceRoot := ""
	if cfg != nil {
		workspaceRoot = cfg.Workspace.Root
	}

	taskManager := task.NewTaskManager()
	taskCreateTool, err := taskManager.CreateTaskTool()
	if err != nil {
		return nil, fmt.Errorf("创建 create_task 工具失败: %w", err)
	}
	taskUpdateStatusTool, err := taskManager.UpdateStatusTool()
	if err != nil {
		return nil, fmt.Errorf("创建 update_task_status 工具失败: %w", err)
	}
	taskGetTool, err := taskManager.GetTaskTool()
	if err != nil {
		return nil, fmt.Errorf("创建 get_task 工具失败: %w", err)
	}
	taskListTool, err := taskManager.ListTasksTool()
	if err != nil {
		return nil, fmt.Errorf("创建 list_tasks 工具失败: %w", err)
	}

	bashTool, err := basictool.NewBashTool(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("创建 bash 工具失败: %w", err)
	}

	healthCheckTool, err := extratool.NewHealthCheckTool(collector)
	if err != nil {
		return nil, fmt.Errorf("创建 health_check 工具失败: %w", err)
	}

	knowledgeTool, err := extratool.NewKnowledgeSearchTool(fileSearcher)
	if err != nil {
		return nil, fmt.Errorf("创建 knowledge_search 工具失败: %w", err)
	}

	readFileTool, err := basictool.NewReadFileTool(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("创建 read_file 工具失败: %w", err)
	}

	writeFileTool, err := basictool.NewWriteFileTool(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("创建 write_file 工具失败: %w", err)
	}

	editFileTool, err := basictool.NewEditFileTool(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("创建 edit_file 工具失败: %w", err)
	}
	subProxyTool, err := extratool.NewSubProxyTool()
	if err != nil {
		return nil, fmt.Errorf("创建 subproxy 工具失败: %w", err)
	}

	compactTool, compactTrigger, err := contexttool.NewCompactTool()
	if err != nil {
		return nil, fmt.Errorf("创建 compact 工具失败: %w", err)
	}

	contextStatusTool, err := contexttool.NewContextStatusTool()
	if err != nil {
		return nil, fmt.Errorf("创建 context_status 工具失败: %w", err)
	}

	return &ToolHandler{
		tools: []tool.BaseTool{
			bashTool,
			readFileTool,
			writeFileTool,
			editFileTool,
			healthCheckTool,
			knowledgeTool,
			subProxyTool,
			compactTool,
			contextStatusTool,
			taskCreateTool,
			taskUpdateStatusTool,
			taskGetTool,
			taskListTool,
		},
		CompactTrigger: compactTrigger,
	}, nil
}

func (h *ToolHandler) Tools() []tool.BaseTool {
	return h.tools
}
