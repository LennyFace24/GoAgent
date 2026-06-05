package contexttool

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// ContextStatusInput 上下文状态查询输入
type ContextStatusInput struct{}

// NewContextStatusTool 创建上下文状态查询工具
func NewContextStatusTool() (tool.InvokableTool, error) {
	return utils.InferTool(
		"context_status",
		"查询当前上下文的 token 使用情况，包括使用百分比和消息数量",
		func(ctx context.Context, input ContextStatusInput) (string, error) {
			state := GetContextState()
			usage := state.GetUsage()

			status := "正常"
			if usage.IsCritical() {
				status = "危险"
			} else if usage.IsWarning() {
				status = "警告"
			}

			return fmt.Sprintf(`上下文状态:
- Token 使用: %d / %d (%.1f%%)
- 消息数量: %d
- 状态: %s`,
				usage.CurrentTokens, usage.MaxTokens, usage.Percentage,
				usage.MessageCount, status), nil
		},
	)
}
