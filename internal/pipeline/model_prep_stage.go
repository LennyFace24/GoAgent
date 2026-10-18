package pipeline

import (
	"context"

	"github.com/LennyFace24/MiniAgent/internal/modelmgr"
	contexttool "github.com/LennyFace24/MiniAgent/internal/tools/context_tool"
)

// ModelPrepStage selects the models and token budget together for one request.
type ModelPrepStage struct {
	Prepare func(context.Context) (modelmgr.ModelSnapshot, error)
}

func NewModelPrepStage(prepare func(context.Context) (modelmgr.ModelSnapshot, error)) *ModelPrepStage {
	return &ModelPrepStage{Prepare: prepare}
}

func (s *ModelPrepStage) Name() string { return "model_prep" }

func (s *ModelPrepStage) Process(ctx context.Context, mc *MessageContext) error {
	snapshot, err := s.Prepare(ctx)
	if err != nil {
		return err
	}
	mc.ChatModel = snapshot.ChatModel
	mc.ToolModel = snapshot.ToolModel
	budget := snapshot.Config.ContextBudgetTokens()
	if mc.CtxState == nil {
		mc.CtxState = contexttool.NewContextState(budget)
	} else {
		mc.CtxState.ConfigureMaxTokens(budget)
	}
	return nil
}
