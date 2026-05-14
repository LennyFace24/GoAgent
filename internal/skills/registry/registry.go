package skills_registry

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type SkillManifest struct {
	Name        string
	Description string
}

// skill 文档结构
type SkillDocument struct {
	Manifest SkillManifest
	Body     string
}

type SkillRegistry struct {
	documents map[string]SkillDocument
}

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
		meta, body := parseFrontmatter(string(content))
		name := meta["name"]
		if name == "" {
			name = filepath.Base(filepath.Dir(skillPath))
		}
		r.documents[name] = SkillDocument{
			Manifest: SkillManifest{
				Name:        name,
				Description: meta["description"],
			},
			Body: strings.TrimSpace(body),
		}
	}
}

func parseFrontmatter(text string) (map[string]string, string) {
	re := regexp.MustCompile(`(?s)^---\n(.*?)\n---\n(.*)`)
	m := re.FindStringSubmatch(text)
	if m == nil {
		return nil, text
	}
	meta := make(map[string]string)
	for _, line := range strings.Split(m[1], "\n") {
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		meta[strings.TrimSpace(line[:idx])] = strings.TrimSpace(line[idx+1:])
	}
	return meta, m[2]
}

// DescribeAvailable 返回所有技能摘要，注入 system prompt
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

// LoadFullText 返回指定技能完整内容，作为 skill tool 的返回值
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
