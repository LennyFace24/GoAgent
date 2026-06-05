package commands

// ---------- 斜杠命令类型定义 ----------

// CommandType 命令执行类型
type CommandType string

const (
	CommandTypeClient CommandType = "client" // 前端直接处理
	CommandTypeServer CommandType = "server" // 后端处理
)

// Command 斜杠命令定义
type Command struct {
	Name        string      `json:"name"`        // 命令名，如 "clear"
	Description string      `json:"description"` // 命令描述
	Usage       string      `json:"usage"`       // 用法示例，如 "/clear"
	Category    string      `json:"category"`    // 分类：general, context, conversation
	Type        CommandType `json:"type"`        // 执行类型
}

// CommandListResponse 命令列表 API 响应
type CommandListResponse struct {
	Commands []Command `json:"commands"`
}
