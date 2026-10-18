package extratool

import (
	"context"
	"fmt"

	cfg "github.com/LennyFace24/MiniAgent/internal/config"
	"github.com/LennyFace24/MiniAgent/internal/modelmgr"
	"github.com/LennyFace24/MiniAgent/internal/tools/basic_tool"
	"github.com/LennyFace24/MiniAgent/internal/util"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

type SubProxyInput struct {
	SystemInstruction string `json:"system_instruction" description:"Instructions for the subtask execution context." required:"true"`
	Content           string `json:"content" description:"The content of the task to be executed." required:"true"`
}

// runSubAggent 使用调用方提供的模型配置运行独立的子代理循环。
func runSubAggent(ctx context.Context, content SubProxyInput, model modelmgr.Model) string {
	chatmodel, err := model.BuildChatModel(ctx)
	if err != nil {
		return fmt.Sprintf("Error creating subagent: %v", err)
	}

	workspaceRoot := ""
	if c := cfg.GetConfig(); c != nil {
		workspaceRoot = c.Workspace.Root
	}

	tools_ := []tool.BaseTool{
		util.Must(basictool.NewBashTool(workspaceRoot)),
		util.Must(basictool.NewReadFileTool(workspaceRoot)),
		util.Must(basictool.NewEditFileTool(workspaceRoot)),
		util.Must(basictool.NewWriteFileTool(workspaceRoot)),
	}

	toolInfos := make([]*schema.ToolInfo, len(tools_))
	toolMap := make(map[string]tool.InvokableTool, len(tools_))
	for i, t := range tools_ {
		info, err := t.Info(ctx)
		if err != nil {
			return fmt.Sprintf("Error getting tool info: %v", err)
		}
		toolInfos[i] = info
		if inv, ok := t.(tool.InvokableTool); ok {
			toolMap[info.Name] = inv
		}
	}

	toolModel, err := chatmodel.WithTools(toolInfos)
	if err != nil {
		return fmt.Sprintf("Error creating tool-calling subagent: %v", err)
	}

	history := []*schema.Message{
		{Role: schema.Assistant, Content: content.SystemInstruction},
	}
	history = append(history, schema.UserMessage(content.Content))

	turn := 0
	const maxTurn = 10

	for {
		turn++
		resp, err := toolModel.Generate(ctx, history)
		if err != nil {
			return fmt.Sprintf("Error generating response: %v", err)
		}
		history = append(history, resp)
		if resp.ToolCalls == nil {
			return resp.Content
		}
		for _, tc := range resp.ToolCalls {
			res, err := executeTool(ctx, tc, toolMap)
			if err != nil {
				return fmt.Sprintf("Error executing tool: %v", err)
			}
			msg := schema.ToolMessage(res, tc.ID, schema.WithToolName(tc.Function.Name))
			history = append(history, msg)
		}
		if turn >= maxTurn {
			break
		}
	}
	history = append(history, schema.SystemMessage("你已调用足够多次工具，现在必须直接回答用户的问题。不要再调用任何工具。"))
	resp, err := toolModel.Generate(ctx, history)
	if err != nil {
		return fmt.Sprintf("Error generating final response: %v", err)
	}
	return resp.Content
}

func executeTool(ctx context.Context, tc schema.ToolCall, toolMap map[string]tool.InvokableTool) (string, error) {
	t, ok := toolMap[tc.Function.Name]
	if !ok {
		return "", fmt.Errorf("tool not found: %s", tc.Function.Name)
	}
	return t.InvokableRun(ctx, tc.Function.Arguments)
}

// NewSubProxyTool 创建 subproxy 工具。
// getModel 在执行时提供模型配置；当前生产装配返回启动时的默认模型。
func NewSubProxyTool(getModel func() modelmgr.Model) (tool.InvokableTool, error) {
	return utils.InferTool(
		"subproxy",
		"Run a subtask in a clean context and return a summary.",
		func(ctx context.Context, input SubProxyInput) (string, error) {
			m := modelmgr.Model{}
			if getModel != nil {
				m = getModel()
			}
			return runSubAggent(ctx, input, m), nil
		},
	)
}
