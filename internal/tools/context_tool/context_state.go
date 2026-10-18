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

// ContextRegistry 按 sessionID + conversationID 维护独立的上下文状态。
type ContextRegistry struct {
	mu        sync.RWMutex
	maxTokens int64
	states    map[string]*ContextState
}

var globalRegistry = NewContextRegistry(DefaultMaxTokens)

// NewContextRegistry 创建上下文状态注册表。
func NewContextRegistry(maxTokens int64) *ContextRegistry {
	return &ContextRegistry{
		maxTokens: maxTokens,
		states:    make(map[string]*ContextState),
	}
}

// GetContextRegistry 获取全局注册表。
func GetContextRegistry() *ContextRegistry {
	return globalRegistry
}

// ScopeKey 生成对话级上下文状态键。
func ScopeKey(sessionID, conversationID string) string {
	return sessionID + "\x00" + conversationID
}

// State 获取（必要时创建）指定对话的状态。
func (r *ContextRegistry) State(scope string) *ContextState {
	r.mu.Lock()
	defer r.mu.Unlock()
	if state, ok := r.states[scope]; ok {
		return state
	}
	state := NewContextState(r.maxTokens)
	r.states[scope] = state
	return state
}

// Delete 删除指定对话的状态（对话删除时调用）。
func (r *ContextRegistry) Delete(scope string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.states, scope)
}

// SetMaxTokens 更新默认预算，并同步已有状态。
func (r *ContextRegistry) SetMaxTokens(maxTokens int64) {
	if maxTokens <= 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.maxTokens = maxTokens
	for _, state := range r.states {
		state.ConfigureMaxTokens(maxTokens)
	}
}

// MaxTokens 返回当前默认预算。
func (r *ContextRegistry) MaxTokens() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.maxTokens
}

// Usage 返回指定对话的使用情况；不存在时返回 0/当前预算。
func (r *ContextRegistry) Usage(scope string) ContextUsage {
	r.mu.RLock()
	state, ok := r.states[scope]
	maxTokens := r.maxTokens
	r.mu.RUnlock()
	if !ok {
		return ContextUsage{MaxTokens: maxTokens}
	}
	return state.GetUsage()
}

// Reset 清空所有对话状态（测试或重载配置时使用）。
func (r *ContextRegistry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.states = make(map[string]*ContextState)
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

// SetUsage 同时更新 token 使用量与消息数量。
func (cs *ContextState) SetUsage(tokens, messages int64) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.currentTokens = tokens
	cs.messageCount = messages
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
	cs.SetUsage(int64(tokens), int64(len(messages)))
	return tokens
}

// AddMessages 添加新消息并更新 token 计数
func (cs *ContextState) AddMessages(newMessages []*schema.Message) int {
	tokens := EstimateTokens(newMessages)
	cs.UpdateTokens(int64(tokens))
	return tokens
}
