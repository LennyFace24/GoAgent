package commands

import "context"

// ---------- 斜杠命令类型定义 ----------

// CommandType 命令执行类型
type CommandType string

const (
	CommandTypeClient CommandType = "client" // 前端直接处理
	CommandTypeServer CommandType = "server" // 后端处理
)

// ArgsPolicy 参数契约：命令是否需要/允许附带内容
type ArgsPolicy string

const (
	ArgsNone     ArgsPolicy = "none"     // 不接受参数（compact/context/model）
	ArgsOptional ArgsPolicy = "optional" // 参数可选，模板占位符驱动（file-based 默认）
	ArgsRequired ArgsPolicy = "required" // 必需参数（/skill:<name> 的 skill 名）
)

// CommandHandler 命令执行器：builtin/server 命令的 Go handler
type CommandHandler func(ctx context.Context, args []string) (string, error)

// Command 斜杠命令定义
type Command struct {
	Name        string         `json:"name"`         // 命令名，如 "clear"
	Description string         `json:"description"`  // 命令描述
	Usage       string         `json:"usage"`        // 用法示例，如 "/clear"
	Category    string         `json:"category"`     // 分类：general, context, conversation
	Type        CommandType    `json:"type"`         // 执行类型
	ArgsPolicy  ArgsPolicy     `json:"args_policy"`  // 参数契约
	ArgsHint    string         `json:"args_hint"`    // 用法提示，如 "[/compact] 不接受参数"
	Source      string         `json:"source"`       // 来源：builtin / claude-user / claude-project / skill
	Handler     CommandHandler `json:"-"`            // builtin/server 命令的执行器（不序列化给前端）
}

// CommandListResponse 命令列表 API 响应
type CommandListResponse struct {
	Commands []Command `json:"commands"`
}
