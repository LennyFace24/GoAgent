package commands

// ---------- 内置命令 ----------

// registerBuiltinCommands 注册内置命令
func registerBuiltinCommands(r *Registry) {
	builtins := []Command{
		{
			Name:        "help",
			Description: "显示可用命令列表和使用说明",
			Usage:       "/help",
			Category:    "general",
			Type:        CommandTypeClient,
		},
		{
			Name:        "clear",
			Description: "清空当前对话的所有消息",
			Usage:       "/clear",
			Category:    "conversation",
			Type:        CommandTypeClient,
		},
		{
			Name:        "new",
			Description: "创建一个新的对话",
			Usage:       "/new",
			Category:    "conversation",
			Type:        CommandTypeClient,
		},
		{
			Name:        "compact",
			Description: "触发上下文压缩，将旧消息压缩为摘要",
			Usage:       "/compact",
			Category:    "context",
			Type:        CommandTypeClient,
		},
		{
			Name:        "context",
			Description: "显示当前上下文的 Token 使用情况",
			Usage:       "/context",
			Category:    "context",
			Type:        CommandTypeClient,
		},
		{
			Name:        "model",
			Description: "显示当前使用的模型信息",
			Usage:       "/model",
			Category:    "general",
			Type:        CommandTypeClient,
		},
	}

	for _, cmd := range builtins {
		r.Register(cmd)
	}
}
