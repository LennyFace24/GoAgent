package permission

import (
	"regexp"
	"strings"
)

// IsBashReadOnly 判断 bash 命令是否为只读操作
// 判定链：未引用展开 → flags 白名单 → regex 白名单 → 非只读
func IsBashReadOnly(cmd string) bool {
	cmd = stripStderrRedirect(cmd)

	// 前置过滤：未引用的 glob 和 $ 变量
	if containsUnquotedExpansion(cmd) {
		return false
	}

	// 复合命令拆分：按 | && || ; 拆分为子命令，每个独立判定
	subs := splitCommand(cmd)
	for _, sub := range subs {
		sub = strings.TrimSpace(sub)
		if sub == "" {
			continue
		}
		if !isSingleCommandReadOnly(sub) {
			return false
		}
	}
	return true
}

// isSingleCommandReadOnly 判定单条命令是否只读
func isSingleCommandReadOnly(cmd string) bool {
	// 1. flags 白名单
	if isCommandSafeViaFlags(cmd) {
		return true
	}
	// 2. regex 白名单
	for _, re := range readonlyRegexes {
		if re.MatchString(cmd) {
			// 特殊命令需要额外检查危险 flags
			name := extractCommandName(cmd)
			if name == "find" && !isFindSafe(cmd) {
				return false
			}
			if name == "jq" && !isJqSafe(cmd) {
				return false
			}
			return true
		}
	}
	return false
}

// find 的危险 flags
var findDangerousFlags = []string{
	"-exec", "-execdir", "-delete", "-ok", "-okdir",
	"-fprint", "-fprint0", "-fls", "-fprintf",
}

// isFindSafe 检查 find 命令是否不包含危险 flags
func isFindSafe(cmd string) bool {
	fields := strings.Fields(cmd)
	for _, f := range fields {
		for _, dangerous := range findDangerousFlags {
			if f == dangerous {
				return false
			}
		}
	}
	return true
}

// jq 的危险 flags
var jqDangerousFlags = []string{
	"-f", "--from-file", "--rawfile", "--slurpfile",
	"--run-tests", "-L", "--library-path",
}

// isJqSafe 检查 jq 命令是否不包含危险 flags
func isJqSafe(cmd string) bool {
	fields := strings.Fields(cmd)
	for _, f := range fields {
		for _, dangerous := range jqDangerousFlags {
			if f == dangerous {
				return false
			}
		}
	}
	return true
}

// ---------- 前置过滤 ----------

// stripStderrRedirect 去掉 2>&1 等 stderr 重定向
func stripStderrRedirect(cmd string) string {
	cmd = regexp.MustCompile(`\s*2>&1\s*$`).ReplaceAllString(cmd, "")
	cmd = regexp.MustCompile(`\s*2>/dev/null\s*$`).ReplaceAllString(cmd, "")
	return cmd
}

// containsUnquotedExpansion 检测未引用的 glob（*?[]）和 $ 变量
func containsUnquotedExpansion(cmd string) bool {
	inSingle := false
	inDouble := false
	escaped := false

	for i := 0; i < len(cmd); i++ {
		ch := cmd[i]

		if escaped {
			escaped = false
			continue
		}

		if ch == '\\' && !inSingle {
			escaped = true
			continue
		}

		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}

		if ch == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}

		if inSingle {
			continue // 单引号内一切 literal
		}

		// 双引号内 $ 变量展开
		if inDouble && ch == '$' {
			return true
		}

		// 未引用的 glob 字符
		if !inDouble && (ch == '*' || ch == '?' || ch == '[') {
			// 检查是否在命令名位置（第一个 token）
			// 允许命令名后的参数中出现 glob
			// 但 glob 在参数中意味着文件名展开，不可预测
			return true
		}
	}
	return false
}

// ---------- 复合命令拆分 ----------

// splitCommand 按管道和链式操作符拆分子命令
func splitCommand(cmd string) []string {
	var subs []string
	var current strings.Builder
	inSingle := false
	inDouble := false
	escaped := false

	for i := 0; i < len(cmd); i++ {
		ch := cmd[i]

		if escaped {
			current.WriteByte(ch)
			escaped = false
			continue
		}

		if ch == '\\' && !inSingle {
			current.WriteByte(ch)
			escaped = true
			continue
		}

		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			current.WriteByte(ch)
			continue
		}

		if ch == '"' && !inSingle {
			inDouble = !inDouble
			current.WriteByte(ch)
			continue
		}

		if inSingle || inDouble {
			current.WriteByte(ch)
			continue
		}

		// 管道
		if ch == '|' {
			if i+1 < len(cmd) && cmd[i+1] == '|' {
				// ||
				subs = append(subs, current.String())
				current.Reset()
				i++ // skip next |
				continue
			}
			subs = append(subs, current.String())
			current.Reset()
			continue
		}

		// &&
		if ch == '&' && i+1 < len(cmd) && cmd[i+1] == '&' {
			subs = append(subs, current.String())
			current.Reset()
			i++
			continue
		}

		// 分号
		if ch == ';' {
			subs = append(subs, current.String())
			current.Reset()
			continue
		}

		current.WriteByte(ch)
	}

	if current.Len() > 0 {
		subs = append(subs, current.String())
	}
	return subs
}

// ---------- Flags 白名单 ----------

type commandFlags struct {
	safeFlags map[string]bool // flag 名 → 是否安全
}

var safeFlagsMap = map[string]commandFlags{
	"grep": {map[string]bool{
		"-e": true, "-i": true, "-v": true, "-l": true, "-n": true,
		"-r": true, "-R": true, "-c": true, "-w": true, "-x": true,
		"-A": true, "-B": true, "-C": true, "-m": true, "-o": true,
		"-q": true, "-s": true, "-h": true, "-H": true, "--include": true,
		"--exclude": true, "--exclude-dir": true,
	}},
	"find": {map[string]bool{
		"-name": true, "-type": true, "-size": true, "-mtime": true,
		"-mmin": true, "-atime": true, "-ctime": true, "-newer": true,
		"-user": true, "-group": true, "-perm": true, "-maxdepth": true,
		"-mindepth": true, "-empty": true, "-not": true, "-and": true, "-or": true,
		"-print": true, "-print0": true, "-ls": true, "-printf": true,
	}},
	"ls": {map[string]bool{
		"-a": true, "-A": true, "-l": true, "-h": true, "-S": true,
		"-t": true, "-r": true, "-R": true, "-d": true, "-1": true,
		"--color": true, "--group-directories-first": true,
	}},
	"git": {map[string]bool{
		"--porcelain": true, "--short": true, "--oneline": true,
		"--all": true, "--graph": true, "--decorate": true,
		"-n": true, "-p": true, "--stat": true, "--cached": true,
		"--name-only": true, "--name-status": true, "--diff-filter": true,
		"--format": true, "--pretty": true, "--branches": true, "--tags": true,
		"--remotes": true, "--staged": true, "--untracked-files": true,
		"--no-pager": true, "--no-optional-locks": true,
	}},
	"head": {map[string]bool{"-n": true, "-c": true}},
	"tail": {map[string]bool{"-n": true, "-c": true, "-f": true}},
	"wc":  {map[string]bool{"-l": true, "-w": true, "-c": true, "-m": true}},
	"sort": {map[string]bool{
		"-r": true, "-n": true, "-k": true, "-t": true, "-u": true,
		"-f": true, "-h": true, "-R": true,
	}},
	"diff": {map[string]bool{
		"-u": true, "-r": true, "-q": true, "--brief": true,
		"--stat": true, "-y": true, "--side-by-side": true,
	}},
	"du":   {map[string]bool{"-h": true, "-s": true, "-a": true, "--max-depth": true}},
	"df":   {map[string]bool{"-h": true, "-T": true, "-i": true}},
	"free": {map[string]bool{"-h": true, "-m": true, "-g": true, "-k": true}},
	"ps":   {map[string]bool{"-aux": true, "-ef": true, "-p": true, "-u": true}},
	"id":   {map[string]bool{"-u": true, "-g": true, "-n": true, "-G": true}},
	"stat": {map[string]bool{"-c": true, "--format": true}},
}

// extractCommandName 提取命令名（第一个 token）
func extractCommandName(cmd string) string {
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// extractFlags 提取命令中的所有 flags
func extractFlags(cmd string) []string {
	fields := strings.Fields(cmd)
	var flags []string
	for _, f := range fields[1:] {
		if strings.HasPrefix(f, "-") {
			flags = append(flags, f)
		}
	}
	return flags
}

// isCommandSafeViaFlags 检查命令的 flags 是否全在安全白名单中
func isCommandSafeViaFlags(cmd string) bool {
	name := extractCommandName(cmd)
	cf, ok := safeFlagsMap[name]
	if !ok {
		return false // 命令不在 flags 白名单中
	}

	flags := extractFlags(cmd)
	for _, f := range flags {
		// 处理 --flag=value 形式
		flagName := f
		if idx := strings.Index(f, "="); idx != -1 {
			flagName = f[:idx]
		}
		if !cf.safeFlags[flagName] {
			return false // 有不安全的 flag
		}
	}
	return true
}

// ---------- Regex 白名单 ----------

// makeRegexForSafeCommand 为简单只读命令生成 regex
// 匹配：命令名 + 参数（参数中不含 shell 元字符）
func makeRegexForSafeCommand(cmd string) *regexp.Regexp {
	// 参数中禁止：<>() ` | {} & ; 和换行
	return regexp.MustCompile(`^` + regexp.QuoteMeta(cmd) + `(?:\s|$)[^<>()\x60|{}&;\n\r]*$`)
}

var readonlyRegexes = []*regexp.Regexp{
	// 简单只读命令
	makeRegexForSafeCommand("cat"),
	makeRegexForSafeCommand("head"),
	makeRegexForSafeCommand("tail"),
	makeRegexForSafeCommand("less"),
	makeRegexForSafeCommand("more"),
	makeRegexForSafeCommand("strings"),
	makeRegexForSafeCommand("hexdump"),
	makeRegexForSafeCommand("od"),
	makeRegexForSafeCommand("nl"),
	makeRegexForSafeCommand("wc"),
	makeRegexForSafeCommand("stat"),
	makeRegexForSafeCommand("file"),
	makeRegexForSafeCommand("readlink"),
	makeRegexForSafeCommand("realpath"),
	makeRegexForSafeCommand("basename"),
	makeRegexForSafeCommand("dirname"),
	makeRegexForSafeCommand("cut"),
	makeRegexForSafeCommand("paste"),
	makeRegexForSafeCommand("tr"),
	makeRegexForSafeCommand("column"),
	makeRegexForSafeCommand("tac"),
	makeRegexForSafeCommand("rev"),
	makeRegexForSafeCommand("fold"),
	makeRegexForSafeCommand("expand"),
	makeRegexForSafeCommand("unexpand"),
	makeRegexForSafeCommand("fmt"),
	makeRegexForSafeCommand("comm"),
	makeRegexForSafeCommand("cmp"),
	makeRegexForSafeCommand("numfmt"),

	// 系统信息
	makeRegexForSafeCommand("id"),
	makeRegexForSafeCommand("uname"),
	makeRegexForSafeCommand("hostname"),
	makeRegexForSafeCommand("date"),
	makeRegexForSafeCommand("whoami"),
	makeRegexForSafeCommand("pwd"),
	makeRegexForSafeCommand("arch"),
	makeRegexForSafeCommand("locale"),
	makeRegexForSafeCommand("groups"),
	makeRegexForSafeCommand("nproc"),
	makeRegexForSafeCommand("uptime"),

	// 目录/查找
	makeRegexForSafeCommand("which"),
	makeRegexForSafeCommand("type"),
	makeRegexForSafeCommand("tree"),

	// 环境变量（只读查看）
	makeRegexForSafeCommand("env"),
	makeRegexForSafeCommand("printenv"),

	// 版本查询
	makeRegexForSafeCommand("node"),
	makeRegexForSafeCommand("python"),
	makeRegexForSafeCommand("python3"),
	makeRegexForSafeCommand("go"),
	makeRegexForSafeCommand("java"),
	makeRegexForSafeCommand("gcc"),
	makeRegexForSafeCommand("make"),

	// docker 只读
	makeRegexForSafeCommand("docker ps"),
	makeRegexForSafeCommand("docker images"),
	makeRegexForSafeCommand("docker version"),
	makeRegexForSafeCommand("docker info"),

	// 特殊命令：echo（允许引号字符串，禁止 $ 展开）
	regexp.MustCompile(`^echo(?:\s+(?:'[^']*'|"[^"$<>\n\r]*"|[^|;&\x60$(){}><#\\!"'\s]+))*\s*$`),

	// find（基础匹配，危险 flags 由 isFindSafe 单独检查）
	regexp.MustCompile(`^find(?:\s+[^<>()\x60$|{}&;\n\r]*)?$`),

	// ls（允许常规参数）
	regexp.MustCompile(`^ls(?:\s+[^<>()\x60$|{}&;\n\r]*)?$`),

	// cd（允许引号路径）
	regexp.MustCompile(`^cd(?:\s+(?:'[^']*'|"[^"]*"|[^\s;|&\x60$(){}><#\\]+))?$`),

	// jq（基础匹配，危险 flags 由 isJqSafe 单独检查）
	regexp.MustCompile(`^jq(?:\s+[^<>()\x60$|{}&;\n\r]*)?$`),

	// sed 只读模式（只有 -n，没有 -i）
	regexp.MustCompile(`^sed\s+(?:-n\s+)?(?:'[^']*'|"[^"]*")+(?:\s+[^<>()\x60$|{}&;\n\r]+)*\s*$`),

	// git 只读命令
	regexp.MustCompile(`^git\s+(?:status|log|show|diff|blame|branch|tag|ls-files|remote|describe|rev-parse|rev-list|count-objects|cat-file|ls-tree)(?:\s|$)`),
}
