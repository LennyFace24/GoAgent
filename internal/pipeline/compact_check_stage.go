package pipeline

import (
	"context"
	"log"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	contexttool "github.com/LennyFace24/MiniAgent/internal/tools/context_tool"
)

// ---------- Stage 5: CompactCheck ----------
// 发送前预检历史预算，超阈值执行 MicroCompact / CompactFunc。
// （runLoop 内另有每轮兜底检查，两者并存；预检保证首次发送前不超载）

// CompactCheckStage 历史预算压缩
type CompactCheckStage struct {
	ChatModel model.BaseChatModel
	// CompactFull 完整压缩函数（AgentService 提供，含 LLM 摘要）
	CompactFull func(ctx context.Context, messages []*schema.Message, m model.BaseChatModel, sessionID, conversationID string) []*schema.Message
}

// NewCompactCheckStage 创建压缩预检阶段
func NewCompactCheckStage(m model.BaseChatModel) *CompactCheckStage {
	return &CompactCheckStage{ChatModel: m}
}

func (s *CompactCheckStage) Name() string { return "compact_check" }

func (s *CompactCheckStage) Process(ctx context.Context, mc *MessageContext) error {
	if len(mc.History) == 0 {
		return nil
	}
	if mc.CtxState == nil {
		mc.CtxState = contexttool.NewContextState(200000)
	}
	level := contexttool.ShouldCompact(mc.History, mc.CtxState)
	switch level {
	case contexttool.CompactFull:
		log.Printf("pipeline: 发送前触发完整压缩")
		if s.CompactFull != nil {
			mc.History = s.CompactFull(ctx, mc.History, s.ChatModel, mc.SessionID, mc.ConversationID)
		}
	case contexttool.CompactMicro:
		log.Printf("pipeline: 发送前触发微压缩")
		mc.History = contexttool.MicroCompactFunc(mc.History)
	}
	return nil
}
