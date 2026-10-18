package pipeline

import (
	"context"
	"log"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	contexttool "github.com/LennyFace24/MiniAgent/internal/tools/context_tool"
)

// CompactCheckStage checks the assembled request, including system slots and
// the current user message. runLoop rechecks after tools add more messages.
type CompactCheckStage struct {
	CompactFull func(context.Context, []*schema.Message, model.BaseChatModel, string, string) ([]*schema.Message, *contexttool.CompactionResult)
	ShouldForce func() bool
}

func NewCompactCheckStage(compact func(context.Context, []*schema.Message, model.BaseChatModel, string, string) ([]*schema.Message, *contexttool.CompactionResult), shouldForce func() bool) *CompactCheckStage {
	return &CompactCheckStage{CompactFull: compact, ShouldForce: shouldForce}
}

func (s *CompactCheckStage) Name() string { return "compact_check" }

func (s *CompactCheckStage) Process(ctx context.Context, mc *MessageContext) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	level := contexttool.ShouldCompact(mc.Messages, mc.CtxState)
	if s.ShouldForce != nil && s.ShouldForce() {
		level = contexttool.CompactFull
	}
	var result *contexttool.CompactionResult
	switch level {
	case contexttool.CompactFull:
		log.Printf("pipeline: 发送前触发完整压缩")
		mc.Messages, result = s.CompactFull(ctx, mc.Messages, mc.ChatModel, mc.SessionID, mc.ConversationID)
		if result != nil && mc.PersistCompaction != nil {
			if err := mc.PersistCompaction(result); err != nil {
				log.Printf("pipeline: 压缩结果落盘失败: %v", err)
			}
		}
	case contexttool.CompactMicro:
		log.Printf("pipeline: 发送前触发微压缩")
		mc.Messages = contexttool.MicroCompactFunc(mc.Messages)
	}
	mc.CtxState.EstimateAndEstimateMessages(mc.Messages)
	return ctx.Err()
}
