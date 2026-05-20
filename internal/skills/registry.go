package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// SkillRegistry 管理已安装的 skills。
type SkillRegistry struct {
	documents map[string]SkillDocument
}

// NewSkillRegistry 创建 registry 并加载 route.md 索引的所有 skill。
func NewSkillRegistry() *SkillRegistry {
	r := &SkillRegistry{documents: make(map[string]SkillDocument)}
	r.loadAll()
	return r
}

func (r *SkillRegistry) loadAll() {
	routePath := filepath.Join("internal", "skills", "route.md")
	routeData, err := os.ReadFile(routePath)
	if err != nil {
		return
	}

	linkRegex := regexp.MustCompile(`\[.+?\]\((.+?)\)`)
	matches := linkRegex.FindAllStringSubmatch(string(routeData), -1)

	for _, m := range matches {
		skillPath := strings.TrimPrefix(m[1], "/")
		content, err := os.ReadFile(skillPath)
		if err != nil {
			continue
		}
		meta, body := ParseFrontmatter(string(content))
		name := meta["name"]
		if name == "" {
			name = filepath.Base(filepath.Dir(skillPath))
		}
		r.documents[name] = SkillDocument{
			Manifest: SkillManifest{
				Name:        name,
				Description: meta["description"],
			},
			Body: body,
		}
	}
}

// DescribeAvailable 返回所有 skill 的摘要列表。
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

// LoadFullText 返回指定 skill 的完整内容。
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

// Documents 返回所有已加载的 skill 文档。
func (r *SkillRegistry) Documents() map[string]SkillDocument {
	return r.documents
}
