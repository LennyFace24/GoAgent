package pipeline

import (
	"context"
	"log"

	"github.com/LennyFace24/MiniAgent/internal/skills"
)

// ---------- Stage 3: SkillExpand ----------
// 普通消息命中 skill triggers 时，注入 skill 全文到上下文。
// （显式 /skill:<name> 已在命令层处理，这里覆盖隐式触发词匹配）

// SkillExpandStage 匹配 skill 触发词并注入
type SkillExpandStage struct {
	// Matcher 由外部注入，避免 pipeline 直接依赖 SkillRegistry
	Matcher func(text string) (*skills.SkillDocument, bool)
}

// NewSkillExpandStage 创建 skill 展开阶段
func NewSkillExpandStage(matcher func(text string) (*skills.SkillDocument, bool)) *SkillExpandStage {
	return &SkillExpandStage{Matcher: matcher}
}

func (s *SkillExpandStage) Name() string { return "skill_expand" }

func (s *SkillExpandStage) Process(ctx context.Context, mc *MessageContext) error {
	if s.Matcher == nil {
		return nil
	}
	doc, ok := s.Matcher(mc.Raw)
	if !ok || doc == nil {
		return nil
	}
	log.Printf("pipeline: 命中 skill 触发词 %q，注入 skill 全文", doc.Manifest.Name)

	// 注入 skill 全文到上下文（由 ContextBuild 阶段放入 system skills slot）
	mc.Skill = doc
	return nil
}
