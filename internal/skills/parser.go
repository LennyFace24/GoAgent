package skills

import (
	"regexp"
	"strings"
)

// SkillManifest 技能的元信息。
type SkillManifest struct {
	Name        string
	Description string
}

// SkillDocument 一个完整 skill 文件的内容。
type SkillDocument struct {
	Manifest SkillManifest
	Body     string
}

// ParseFrontmatter 解析 skill md 文件的 YAML frontmatter + body。
func ParseFrontmatter(text string) (meta map[string]string, body string) {
	re := regexp.MustCompile(`(?s)^---\n(.*?)\n---\n(.*)`)
	m := re.FindStringSubmatch(text)
	if m == nil {
		return nil, text
	}
	meta = make(map[string]string)
	for _, line := range strings.Split(m[1], "\n") {
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		meta[strings.TrimSpace(line[:idx])] = strings.TrimSpace(line[idx+1:])
	}
	return meta, strings.TrimSpace(m[2])
}
