package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/LennyFace24/MiniAgent/internal/skills"
)

// ---------- skill 命令（/skill:<name>） ----------
// 注册后命令面板可见；执行时渲染为 skill 全文 + 用户参数，
// 由前端作为普通消息发出（进管线 → LLM 流程）。

// RegisterSkillCommands 将 SkillRegistry 中可用的 skill 注册为 /skill:<name> 命令。
func RegisterSkillCommands(r *Registry, reg *skills.SkillRegistry) {
	if reg == nil {
		return
	}
	for _, doc := range reg.ListCommandCandidates() {
		name := "skill:" + doc.Manifest.Name
		if r.Has(name) {
			continue
		}
		docCopy := doc
		r.Register(Command{
			Name:        name,
			Description: docCopy.Manifest.Description,
			Usage:       "/" + name + " [args]",
			Category:    "skill",
			Type:        CommandTypeServer,
			ArgsPolicy:  ArgsOptional,
			ArgsHint:    "用法: /" + name + " [args]",
			Source:      "skill",
			Handler: func(ctx context.Context, args []string) (string, error) {
				text := reg.LoadFullText(docCopy.Manifest.Name)
				if len(args) > 0 {
					text += "\n\nUser: " + strings.Join(args, " ")
				}
				return text, nil
			},
		})
	}
}

// FormatSkillCommandText 供测试与前端展示使用
func FormatSkillCommandText(doc skills.SkillDocument) string {
	return fmt.Sprintf("skill:%s", doc.Manifest.Name)
}
