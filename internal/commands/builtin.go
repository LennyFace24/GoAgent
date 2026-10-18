package commands

import (
	"context"
	"fmt"

	"github.com/LennyFace24/MiniAgent/internal/modelmgr"
	contexttool "github.com/LennyFace24/MiniAgent/internal/tools/context_tool"
)

// registerBuiltinCommands 注册内置命令
func registerBuiltinCommands(r *Registry) {
	builtins := []Command{
		{
			Name:        "help",
			Description: "显示可用命令列表和使用说明",
			Usage:       "/help",
			Category:    "general",
			Type:        CommandTypeClient,
			ArgsPolicy:  ArgsNone,
			Source:      "builtin",
		},
		{
			Name:        "clear",
			Description: "清空当前对话的所有消息",
			Usage:       "/clear",
			Category:    "conversation",
			Type:        CommandTypeClient,
			ArgsPolicy:  ArgsNone,
			Source:      "builtin",
		},
		{
			Name:        "compact",
			Description: "触发上下文压缩，将旧消息压缩为摘要",
			Usage:       "/compact",
			Category:    "context",
			Type:        CommandTypeServer,
			ArgsPolicy:  ArgsNone,
			ArgsHint:    "/compact 不接受参数",
			Source:      "builtin",
			Handler:     compactHandler,
		},
		{
			Name:        "context",
			Description: "显示当前上下文的 Token 使用情况",
			Usage:       "/context",
			Category:    "context",
			Type:        CommandTypeServer,
			ArgsPolicy:  ArgsNone,
			ArgsHint:    "/context 不接受参数",
			Source:      "builtin",
			Handler:     contextHandler,
		},
		{
			Name:        "model",
			Description: "显示当前使用的模型信息",
			Usage:       "/model",
			Category:    "general",
			Type:        CommandTypeServer,
			ArgsPolicy:  ArgsNone,
			ArgsHint:    "/model 不接受参数",
			Source:      "builtin",
			Handler:     modelHandler,
		},
	}

	r.RegisterAll(builtins)
}

// compactHandler 触发全局压缩标记，下一次回复前执行压缩
func compactHandler(ctx context.Context, args []string) (string, error) {
	contexttool.GetCompactTrigger().Trigger()
	return "已触发上下文压缩，将在下一次回复前自动压缩历史消息。", nil
}

// contextHandler 返回当前请求所属对话的上下文 Token 使用情况。
func contextHandler(ctx context.Context, args []string) (string, error) {
	usage := contexttool.ContextUsage{MaxTokens: contexttool.GetContextRegistry().MaxTokens()}
	if scope, ok := contexttool.ScopeFromContext(ctx); ok {
		usage = contexttool.GetContextRegistry().Usage(scope)
	}
	return usage.String(), nil
}

// modelHandler 返回默认模型信息
func modelHandler(ctx context.Context, args []string) (string, error) {
	// 运行时从 model.json 读取默认模型（与启动时同一注册表语义）
	reg, err := modelmgr.Load("model.json")
	if err != nil {
		return fmt.Sprintf("模型配置加载失败: %v", err), nil
	}
	m := reg.Default()
	return fmt.Sprintf("当前模型: %s（%s）", m.ID, m.BaseURL), nil
}
