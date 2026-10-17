package pipeline

import (
	"context"
	"log"

	"github.com/cloudwego/eino/components/model"

	"github.com/LennyFace24/MiniAgent/internal/modelmgr"
)

// ---------- Stage 4: ModelPrep ----------
// 模型实例与认证准备。构建函数由外部注入（AgentService 提供 ModelSet 惰性构建）。

// ModelPrepStage 准备模型实例
type ModelPrepStage struct {
	// BuildModel 由外部注入，返回 ChatModel
	BuildModel func(ctx context.Context) (model.BaseChatModel, error)
	// BuildToolModel 由外部注入，返回带工具的模型
	BuildToolModel func(ctx context.Context) (model.ToolCallingChatModel, error)
	// GetModel 返回当前激活模型配置（用于独立使用管线时的默认构建）
	GetModel func() modelmgr.Model
}

// NewModelPrepStage 创建模型准备阶段
func NewModelPrepStage(build func(ctx context.Context) (model.BaseChatModel, error), buildTool func(ctx context.Context) (model.ToolCallingChatModel, error)) *ModelPrepStage {
	return &ModelPrepStage{BuildModel: build, BuildToolModel: buildTool}
}

// WithGetModel 设置模型提供器（独立使用管线时用于默认构建）
func (s *ModelPrepStage) WithGetModel(get func() modelmgr.Model) *ModelPrepStage {
	s.GetModel = get
	return s
}

func (s *ModelPrepStage) Name() string { return "model_prep" }

func (s *ModelPrepStage) Process(ctx context.Context, mc *MessageContext) error {
	if s.BuildToolModel != nil {
		tm, err := s.BuildToolModel(ctx)
		if err != nil {
			return err
		}
		mc.ToolModel = tm
		return nil
	}
	if s.BuildModel != nil {
		cm, err := s.BuildModel(ctx)
		if err != nil {
			return err
		}
		mc.ChatModel = cm
		return nil
	}

	// 默认实现：从模型提供器构建（独立使用管线）
	if s.GetModel != nil {
		m := s.GetModel()
		cm, err := m.BuildChatModel(ctx)
		if err != nil {
			return err
		}
		mc.ChatModel = cm
		return nil
	}

	log.Printf("pipeline: 未配置模型构建器，跳过模型准备")
	return nil
}
