package skills

import (
	"regexp"
	"strings"
)

// SkillManifest 技能的元信息。
type SkillManifest struct {
	Name        string
	Description string
	Triggers    []string
	Version     string
	Source      string // user / project / custom
	Path        string // SKILL.md 绝对路径（按需读取）
}

// SkillDocument 一个完整 skill 文件的内容。
type SkillDocument struct {
	Manifest SkillManifest
	Body     string
}

// ParseFrontmatter 解析 skill md 文件的 YAML frontmatter + body。
// 支持：
//   - 单行 `key: value`
//   - 折叠块标量 `key: >-` / `key: >`（多行拼接，空格连接）
//   - 字面块标量 `key: |` / `key: |-`（多行保留换行）
//   - 嵌套映射（如 metadata:）整块跳过
func ParseFrontmatter(text string) (meta map[string]string, body string) {
	re := regexp.MustCompile(`(?s)^---\n(.*?)\n---\n(.*)`)
	m := re.FindStringSubmatch(text)
	if m == nil {
		return nil, text
	}
	meta = make(map[string]string)
	lines := strings.Split(m[1], "\n")
	for i := range lines {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// 嵌套映射行（两空格缩进开头）属于上层 key 的子结构，整块跳过
		if strings.HasPrefix(line, "  ") {
			continue
		}
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])

		// 块标量：收集后续缩进行
		if val == ">-" || val == ">" || val == "|" || val == "|-" {
			var block []string
			for i+1 < len(lines) {
				next := lines[i+1]
				if strings.TrimSpace(next) == "" {
					i++
					continue
				}
				if !strings.HasPrefix(next, " ") {
					break
				}
				block = append(block, strings.TrimSpace(next))
				i++
			}
			if val == "|" || val == "|-" {
				meta[key] = strings.Join(block, "\n")
			} else {
				meta[key] = strings.Join(block, " ")
			}
			continue
		}

		meta[key] = val
	}
	return meta, strings.TrimSpace(m[2])
}
