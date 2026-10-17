package pipeline

import (
	"context"
	"sync"
)

// 事件名称（对齐 omp hooks 命名）
const (
	EventPreProcess  = "pre_process"  // 管线开始（步骤 2）
	EventPreSend     = "pre_send"     // 发送前（步骤 7 前）
	EventPostSend    = "post_send"    // 流结束
)

// Hook 事件钩子：插件注入点
type Hook func(ctx context.Context, mc *MessageContext) error

// EventBus 事件总线
type EventBus struct {
	mu    sync.RWMutex
	hooks map[string][]Hook
}

// NewEventBus 创建事件总线
func NewEventBus() *EventBus {
	return &EventBus{hooks: make(map[string][]Hook)}
}

// RegisterHook 注册钩子到指定事件
func (b *EventBus) RegisterHook(event string, h Hook) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.hooks[event] = append(b.hooks[event], h)
}

// Emit 触发事件，按注册顺序执行所有钩子
func (b *EventBus) Emit(event string, ctx context.Context, mc *MessageContext) error {
	b.mu.RLock()
	hooks := append([]Hook(nil), b.hooks[event]...)
	b.mu.RUnlock()
	for _, h := range hooks {
		if err := h(ctx, mc); err != nil {
			return err
		}
	}
	return nil
}
