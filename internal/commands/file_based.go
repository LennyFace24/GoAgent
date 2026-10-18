package commands

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LennyFace24/MiniAgent/internal/skills"
)

// ---------- file-based 命令（markdown 模板命令，对齐 omp） ----------
// 来源：~/.claude/commands/**/*.md（递归）+ <cwd>/.claude/commands/**/*.md（递归，项目优先）
// 子目录 foo/bar.md 注册为 bar + foo:bar 两个名字；builtin 命令优先，同名文件命令不覆盖。

// LoadFileCommands 扫描用户与项目级 .claude/commands 目录并注册 file-based 命令
func LoadFileCommands(r *Registry) {
	cwd, _ := os.Getwd()
	if cwd != "" {
		scanCommandDir(r, filepath.Join(cwd, ".claude", "commands"), "claude-project")
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		scanCommandDir(r, filepath.Join(home, ".claude", "commands"), "claude-user")
	}
}

func scanCommandDir(r *Registry, root, source string) {
	var files []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // 目录不存在或不可读，静默跳过
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)

	for _, f := range files {
		rel, err := filepath.Rel(root, f)
		if err != nil {
			continue
		}
		name := strings.TrimSuffix(rel, ".md")
		base := filepath.Base(name)

		// 子目录 foo/bar.md：注册 bar + foo:bar 别名
		if strings.Contains(name, string(filepath.Separator)) {
			registerFileCommand(r, base, f, source)
			registerFileCommand(r, strings.ReplaceAll(name, string(filepath.Separator), ":"), f, source)
			continue
		}
		registerFileCommand(r, base, f, source)
	}
}

func registerFileCommand(r *Registry, name, path, source string) {
	if r.Has(name) {
		return // builtin/已注册命令优先
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	meta, body := skills.ParseFrontmatter(string(data))
	if meta["name"] != "" {
		name = meta["name"]
	}
	desc := meta["description"]
	if desc == "" {
		first := strings.TrimSpace(body)
		if idx := strings.IndexByte(first, '\n'); idx >= 0 {
			first = first[:idx]
		}
		desc = first
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
	}
	body = strings.TrimSpace(body)

	cmd := Command{
		Name:        name,
		Description: desc,
		Usage:       "/" + name,
		Category:    "custom",
		Type:        CommandTypeServer,
		ArgsPolicy:  ArgsOptional,
		Source:      source,
		Handler: func(ctx context.Context, args []string) (string, error) {
			return RenderTemplate(body, args), nil
		},
	}
	r.Register(cmd)
}

// RenderTemplate 渲染命令模板：
// - $1..$9 位置参数
// - $ARGUMENTS / $@ 聚合参数
// - 未被位置参数消费的尾部参数追加到渲染文本末尾
func RenderTemplate(body string, args []string) string {
	all := strings.Join(args, " ")
	aggregateUsed := strings.Contains(body, "$ARGUMENTS") || strings.Contains(body, "$@")
	rendered := strings.ReplaceAll(body, "$ARGUMENTS", all)
	rendered = strings.ReplaceAll(rendered, "$@", all)

	maxPos := 0
	for i := 1; i <= 9; i++ {
		ph := "$" + string(rune('0'+i))
		if !strings.Contains(rendered, ph) {
			continue
		}
		if i > maxPos {
			maxPos = i
		}
		val := ""
		if i <= len(args) {
			val = args[i-1]
		}
		rendered = strings.ReplaceAll(rendered, ph, val)
	}

	if !aggregateUsed && len(args) > maxPos {
		remaining := strings.TrimSpace(strings.Join(args[maxPos:], " "))
		if remaining != "" {
			rendered = strings.TrimSpace(rendered) + "\n\n" + remaining
		}
	}
	return rendered
}
