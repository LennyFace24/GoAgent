package modelmgr

import (
	"context"

	"github.com/cloudwego/eino-ext/components/model/openai"
)

// Model 单个模型配置（model.json 条目）
type Model struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	BaseURL             string `json:"base_url"`
	APIKey              string `json:"api_key"`
	ContextWindow       int    `json:"context_window"`
	MaxCompletionTokens int    `json:"max_completion_tokens"`
	SafetyMarginTokens  int    `json:"safety_margin_tokens"`
}

// DefaultContextWindow 默认上下文窗口（与旧 config 一致）
const DefaultContextWindow = 200_000

// ContextBudgetTokens 上下文预算 = 窗口 - 最大输出 - 安全余量
func (m Model) ContextBudgetTokens() int64 {
	window := m.ContextWindow
	if window <= 0 {
		window = DefaultContextWindow
	}
	budget := window - m.MaxCompletionTokens - m.SafetyMarginTokens
	if budget <= 0 {
		return int64(window)
	}
	return int64(budget)
}

// BuildChatModel 构建 eino ChatModel（返回具体类型，支持 WithTools）
func (m *Model) BuildChatModel(ctx context.Context) (*openai.ChatModel, error) {
	// 只有配置了正数才发送该参数；0 交给服务端默认，避免部分兼容服务把 0
	// 解释为“不限制”或直接报错。
	var maxTokens *int
	if m.MaxCompletionTokens > 0 {
		value := m.MaxCompletionTokens
		maxTokens = &value
	}
	return openai.NewChatModel(ctx, &openai.ChatModelConfig{
		Model:               m.ID,
		APIKey:              m.APIKey,
		BaseURL:             m.BaseURL,
		MaxCompletionTokens: maxTokens,
	})
}
