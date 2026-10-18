package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SkillRegistry 管理已发现的 skills。
// 来源（对齐 omp，直接读取零迁移）：
//   - project: <cwd>/.agents/skills/<name>/SKILL.md（项目级，可进 git，优先级最高）
//   - user:    ~/.claude/skills/<name>/SKILL.md（复用用户现有 skill）
//   - custom:  显式配置的目录（config.yaml skills.customDirectories）
// 同名 skill：project > user > custom 优先，冲突打日志。
type SkillRegistry struct {
	documents map[string]SkillDocument
}

// NewSkillRegistry 创建 registry 并扫描所有来源。
func NewSkillRegistry() *SkillRegistry {
	r := &SkillRegistry{documents: make(map[string]SkillDocument)}
	r.loadAll()
	return r
}

func (r *SkillRegistry) loadAll() {
	// 优先级：project > user > custom
	r.scanDir(".agents/skills", "project")
	home, _ := os.UserHomeDir()
	if home != "" {
		r.scanDir(filepath.Join(home, ".claude", "skills"), "user")
	}
}

// scanDir 一级非递归扫描 <dir>/*/SKILL.md
func (r *SkillRegistry) scanDir(root, source string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return // 目录不存在，静默跳过
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		skillDir := filepath.Join(root, e.Name())
		skillFile := filepath.Join(skillDir, "SKILL.md")
		data, err := os.ReadFile(skillFile)
		if err != nil {
			continue
		}
		meta, body := ParseFrontmatter(string(data))
		name := meta["name"]
		if name == "" {
			name = e.Name()
		}
		if meta["description"] == "" {
			// 无 description 的 skill 不注册为命令，但仍保留文档
		}
		// 同名冲突：高优先级 source 已存在则跳过（首次加载顺序 project→user→custom）
		if _, exists := r.documents[name]; exists {
			fmt.Printf("SkillRegistry: skill %q 冲突，跳过 %s 来源\n", name, source)
			continue
		}
		triggers := splitList(meta["triggers"])
		r.documents[name] = SkillDocument{
			Manifest: SkillManifest{
				Name:        name,
				Description: meta["description"],
				Triggers:    triggers,
				Version:     meta["version"],
				Source:      source,
				Path:        skillFile,
			},
			Body: body,
		}
	}
}

// splitList 解析 frontmatter 中的逗号/换行分隔列表
func splitList(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, part := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == '\n'
	}) {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// Match 根据触发词匹配 skill（普通消息命中 triggers 时自动注入）
func (r *SkillRegistry) Match(text string) *SkillDocument {
	for _, doc := range r.documents {
		for _, t := range doc.Manifest.Triggers {
			if t != "" && strings.Contains(text, t) {
				copy := doc
				return &copy
			}
		}
	}
	return nil
}

// DescribeAvailable 返回所有 skill 的摘要列表（system prompt 用）。
func (r *SkillRegistry) DescribeAvailable() string {
	if len(r.documents) == 0 {
		return ""
	}
	names := make([]string, 0, len(r.documents))
	for name := range r.documents {
		names = append(names, name)
	}
	sort.Strings(names)

	var sb strings.Builder
	for _, name := range names {
		doc := r.documents[name]
		fmt.Fprintf(&sb, "- %s: %s\n", doc.Manifest.Name, doc.Manifest.Description)
	}
	return sb.String()
}

// LoadFullText 返回指定 skill 的完整内容（按需读取）。
func (r *SkillRegistry) LoadFullText(name string) string {
	doc, ok := r.documents[name]
	if !ok {
		known := make([]string, 0, len(r.documents))
		for k := range r.documents {
			known = append(known, k)
		}
		sort.Strings(known)
		return fmt.Sprintf("Error: 未知技能 '%s'。可用技能: %s", name, strings.Join(known, ", "))
	}
	return fmt.Sprintf("<skill name=\"%s\">\n%s\n</skill>", doc.Manifest.Name, doc.Body)
}

// ListCommandCandidates 返回可注册为命令的 skill（有 description 的）。
// 供 commands 包注册 /skill:<name> 命令。
func (r *SkillRegistry) ListCommandCandidates() []SkillDocument {
	out := make([]SkillDocument, 0, len(r.documents))
	for _, doc := range r.documents {
		if doc.Manifest.Description != "" {
			out = append(out, doc)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Manifest.Name < out[j].Manifest.Name
	})
	return out
}
