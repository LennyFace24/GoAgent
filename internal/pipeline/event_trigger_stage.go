package pipeline

import (
	"context"
	"log"
)

// ---------- Stage 2: EventTrigger ----------
// 触发 pre_process 事件，让插件有机会在管线早期介入。

// EventTriggerStage 触发事件总线的 pre_process 钩子
type EventTriggerStage struct {
	bus *EventBus
}

// NewEventTriggerStage 创建事件触发阶段
func NewEventTriggerStage(bus *EventBus) *EventTriggerStage {
	return &EventTriggerStage{bus: bus}
}

func (s *EventTriggerStage) Name() string { return "event_trigger" }

func (s *EventTriggerStage) Process(ctx context.Context, mc *MessageContext) error {
	if s.bus == nil {
		return nil
	}
	if err := s.bus.Emit(EventPreProcess, ctx, mc); err != nil {
		log.Printf("pipeline: pre_process hook 失败: %v", err)
		return err
	}
	return nil
}
