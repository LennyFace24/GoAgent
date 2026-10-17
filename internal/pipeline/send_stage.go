package pipeline

import (
	"context"
)

// ---------- Stage 7: Send ----------
// 消息已组装完成（mc.Messages），由 AgentService.runLoop 消费启动流式。
// 本阶段是管线终点标记：runLoop 直接使用 mc.Messages，无需额外处理。

// SendStage 管线终点
type SendStage struct{}

// NewSendStage 创建发送阶段
func NewSendStage() *SendStage {
	return &SendStage{}
}

func (s *SendStage) Name() string { return "send" }

func (s *SendStage) Process(ctx context.Context, mc *MessageContext) error {
	// 最终消息已在 ContextBuild 阶段组装进 mc.Messages
	// 触发 pre_send 钩子（由外部 EventBus 在管线外部调用）
	return nil
}
