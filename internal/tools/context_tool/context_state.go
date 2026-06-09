package contexttool

import (
	"fmt"
	"sync"

	"github.com/cloudwego/eino/schema"
)

// ContextState 上下文状态管理器
type ContextState struct {
	mu            sync.RWMutex
	maxTokens     int64 // 最大 token 限制
	currentTokens int64 // 当前 token 使用量
	messageCount  int64 // 消息数量
}

// 默认配置
const (
	DefaultMaxTokens = 200_000 // 200K tokens
)

// 全局实例
var globalContextState *ContextState

func init() {
	globalContextState = NewContextState(DefaultMaxTokens)
}

// NewContextState 创建上下文状态管理器
func NewContextState(maxTokens int64) *ContextState {
	return &ContextState{
		maxTokens: maxTokens,
	}
}

func (cs *ContextState) ConfigureMaxTokens(maxTokens int64) {
	if maxTokens <= 0 {
		return
	}
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.maxTokens = maxTokens
}

// MaxTokens 返回当前最大 token 限制（只读）
func (cs *ContextState) MaxTokens() int64 {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.maxTokens

}

// GetContextState 获取全局上下文状态
func GetContextState() *ContextState {
	return globalContextState
}

// UpdateTokens 更新 token 使用量（增量更新）
func (cs *ContextState) UpdateTokens(tokens int64) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.currentTokens += tokens
	cs.messageCount++
}

// SetTokens 设置 token 使用量（全量更新）
func (cs *ContextState) SetTokens(tokens int64) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.currentTokens = tokens
}

// Reset 重置状态
func (cs *ContextState) Reset() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.currentTokens = 0
	cs.messageCount = 0
}

// GetUsage 获取使用情况
func (cs *ContextState) GetUsage() ContextUsage {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	percentage := float64(0)
	if cs.maxTokens > 0 {
		percentage = float64(cs.currentTokens) / float64(cs.maxTokens) * 100
	}

	return ContextUsage{
		CurrentTokens: cs.currentTokens,
		MaxTokens:     cs.maxTokens,
		Percentage:    percentage,
		MessageCount:  cs.messageCount,
	}
}

// ContextUsage 上下文使用情况
type ContextUsage struct {
	CurrentTokens int64   `json:"current_tokens"`
	MaxTokens     int64   `json:"max_tokens"`
	Percentage    float64 `json:"percentage"`
	MessageCount  int64   `json:"message_count"`
}

// String 格式化输出
func (u ContextUsage) String() string {
	return fmt.Sprintf("Token 使用: %d/%d (%.1f%%), 消息数: %d",
		u.CurrentTokens, u.MaxTokens, u.Percentage, u.MessageCount)
}

// IsWarning 是否警告状态 (>80%)
func (u ContextUsage) IsWarning() bool {
	return u.Percentage > 80
}

// IsCritical 是否危险状态 (>90%)
func (u ContextUsage) IsCritical() bool {
	return u.Percentage > 90
}

// EstimateAndEstimateMessages 估算消息的 token 数并更新状态
func (cs *ContextState) EstimateAndEstimateMessages(messages []*schema.Message) int {
	tokens := EstimateTokens(messages)
	cs.SetTokens(int64(tokens))
	return tokens
}

// AddMessages 添加新消息并更新 token 计数
func (cs *ContextState) AddMessages(newMessages []*schema.Message) int {
	tokens := EstimateTokens(newMessages)
	cs.UpdateTokens(int64(tokens))
	return tokens
}
