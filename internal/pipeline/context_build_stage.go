package pipeline

import (
	"context"
	"log"
	"strings"

	ctxmgr "github.com/LennyFace24/MiniAgent/internal/context"
)

// ---------- Stage 6: ContextBuild ----------
// 组装 system slots + history + user 消息。

// ContextBuildStage 组装最终消息
type ContextBuildStage struct {
	// BuildManager 由外部注入（AgentService 提供 skillReg、budget 等）
	BuildManager func(mc *MessageContext) *ctxmgr.ContextManager
}

// NewContextBuildStage 创建上下文组装阶段
func NewContextBuildStage(build func(mc *MessageContext) *ctxmgr.ContextManager) *ContextBuildStage {
	return &ContextBuildStage{BuildManager: build}
}

func (s *ContextBuildStage) Name() string { return "context_build" }

func (s *ContextBuildStage) Process(ctx context.Context, mc *MessageContext) error {
	var cm *ctxmgr.ContextManager
	if s.BuildManager != nil {
		cm = s.BuildManager(mc)
	} else {
		cm = ctxmgr.New(200000, nil)
	}
	mc.Manager = cm

	cm.SetMode(mc.Mode)
	cm.LoadSystemInfo()
	cm.LoadSoul()
	cm.LoadProjectContext()
	cm.LoadSkills()
	cm.LoadMemory()

	// skill 注入：放进 skills slot（与 LoadSkills 索引并列）
	if mc.Skill != nil {
		skillContent := cm.Build() // 触发排序前的当前内容
		_ = skillContent
		// 通过注入 slot 追加 skill 全文
		cm.AppendSlot("skill_active", strings.TrimSpace(mc.Skill.Body), 4)
	}

	// 历史预算压缩在 CompactCheck 阶段已完成，这里直接组装
	mc.Messages = cm.BuildMessages(mc.History, mc.Raw)
	log.Printf("pipeline: 组装完成，共 %d 条消息", len(mc.Messages))
	return nil
}
