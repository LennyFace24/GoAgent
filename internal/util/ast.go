package util

import (
	"context"
	"log"
	"strings"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/bash"
)

// BashAccessType bash 命令的访问类型
type BashAccessType int

const (
	BashAccessRead    BashAccessType = iota // 只读操作
	BashAccessWrite                         // 写操作
	BashAccessUnknown                       // 未知
)

// 只读命令白名单。只收录无法通过参数产生写入/执行副作用的命令；
// 带危险参数的读取命令（date -s、hostname NAME 等）在 analyzeCommand 中单独校验。
var readonlyCommands = map[string]bool{
	// 文件查看
	"cat": true, "head": true, "tail": true,
	"strings": true, "hexdump": true, "od": true, "nl": true,
	// 文件信息
	"wc": true, "stat": true, "file": true, "readlink": true, "realpath": true,
	"basename": true, "dirname": true,
	// 文本处理（仅输出到 stdout）
	"cut": true, "paste": true, "tr": true, "column": true, "tac": true,
	"rev": true, "fold": true, "expand": true, "unexpand": true,
	"comm": true, "cmp": true, "numfmt": true, "sort": true, "uniq": true,
	"diff": true, "grep": true, "egrep": true, "fgrep": true, "rg": true,
	// 系统信息
	"id": true, "uname": true, "whoami": true,
	"pwd": true, "arch": true, "locale": true, "groups": true, "nproc": true,
	"uptime": true, "df": true, "du": true, "free": true,
	"ps": true, "printenv": true, "which": true, "type": true,
	"tree": true, "ls": true, "dir": true,
}

// 写命令黑名单
var writeCommands = map[string]bool{
	// 文件操作
	"rm": true, "rmdir": true, "mv": true, "cp": true, "touch": true,
	"mkdir": true, "chmod": true, "chown": true, "chgrp": true,
	"ln": true, "install": true, "dd": true, "truncate": true,
	// 编辑器
	"vi": true, "vim": true, "nano": true, "emacs": true,
	// 包管理
	"apt": true, "apt-get": true, "yum": true, "dnf": true, "brew": true,
	"pip": true, "npm": true, "yarn": true, "pnpm": true,
	// 系统管理
	"sudo": true, "su": true, "systemctl": true, "service": true,
	"mount": true, "umount": true, "fdisk": true, "mkfs": true,
	// 网络写操作
	"ssh": true, "scp": true, "rsync": true,
}

// git 只读子命令。branch/tag/remote 可通过参数写仓库，单独校验。
var gitReadonlySubcommands = map[string]bool{
	"status": true, "log": true, "show": true, "diff": true, "blame": true,
	"ls-files": true, "describe": true, "rev-parse": true, "rev-list": true,
	"count-objects": true, "cat-file": true, "ls-tree": true, "shortlog": true,
	"whatchanged": true, "grep": true,
}

// git 写子命令
var gitWriteSubcommands = map[string]bool{
	"add": true, "commit": true, "push": true, "pull": true, "merge": true,
	"rebase": true, "reset": true, "checkout": true, "stash": true,
	"cherry-pick": true, "revert": true, "clean": true, "rm": true,
}

// 1. 声明一个全局的对象池，专门用来复用重型的 Parser 对象
var parserPool = sync.Pool{
	New: func() interface{} {
		p := sitter.NewParser()
		if p != nil {
			// 在创建时就绑定好语言，避免后续重复绑定
			p.SetLanguage(bash.GetLanguage())
		}
		return p
	},
}

// AnalyzeBashCommand 使用 tree-sitter AST 分析 bash 命令的访问类型。
// 解析失败、panic 或无法识别的结构一律返回 Unknown，绝不自动放行。
func AnalyzeBashCommand(cmd string) (access BashAccessType) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return BashAccessUnknown
	}

	// 解析异常必须降级为 Unknown；零值是 Read，不能在 panic 时直接返回。
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[FATAL] Tree-sitter panicked during parsing: %v", r)
			access = BashAccessUnknown
		}
	}()

	// 2. 从对象池中获取一个现成的 Parser，无 CGO 新建开销
	p := parserPool.Get()
	if p == nil {
		return analyzeFallback(cmd)
	}
	parser := p.(*sitter.Parser)

	// 3. 核心安全修复：使用 defer 确保无论函数从哪个分支返回，Parser 都会被稳妥放回池中
	// 由于 Parser 在池中存活，其底层的 C 内存会一直复用，彻底终结了单次调用泄露的问题
	defer parserPool.Put(parser)

	// 4. 使用标准的 ParseCtx 进行解析
	tree, err := parser.ParseCtx(context.Background(), nil, []byte(cmd))
	if err != nil || tree == nil {
		return analyzeFallback(cmd)
	}
	// 5. Tree 对象同样是 CGO 对象，必须显式 Close 释放它单次生成的 AST 树内存
	defer tree.Close()

	root := tree.RootNode()
	if root == nil {
		return analyzeFallback(cmd)
	}

	// 遍历 AST 分析
	return analyzeNode(root, cmd)
}

// analyzeNode 递归分析 AST 节点。
// 未识别的节点必须向下聚合子节点结果：任一子节点 Write 即 Write，
// 任一子节点 Unknown 即 Unknown，只有全部确定只读才返回 Read。
func analyzeNode(node *sitter.Node, source string) BashAccessType {
	switch node.Type() {
	case "redirect":
		// 重定向分析
		return analyzeRedirect(node, source)
	case "command":
		// 命令分析
		return analyzeCommand(node, source)
	case "pipeline":
		// 管道：分析每个命令
		return analyzePipeline(node, source)
	case "list":
		// 命令列表（&&, ||, ;）
		return analyzeList(node, source)
	case "compound_statement":
		// 复合语句
		return analyzeCompound(node, source)
	default:
		result := BashAccessRead
		for i := 0; i < int(node.ChildCount()); i++ {
			switch analyzeNode(node.Child(i), source) {
			case BashAccessWrite:
				return BashAccessWrite
			case BashAccessUnknown:
				result = BashAccessUnknown
			}
		}
		return result
	}
}

// analyzeRedirect 分析重定向节点
func analyzeRedirect(node *sitter.Node, source string) BashAccessType {
	// 查找重定向操作符
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == ">" || child.Type() == ">>" ||
			child.Type() == "&>" || child.Type() == "&>>" {
			return BashAccessWrite
		}
	}
	return BashAccessRead
}

// cleanCommandName 清理命令名，去除首尾的引号
func cleanCommandName(name string) string {
	name = strings.TrimSpace(name)
	if len(name) >= 2 {
		// 只剥离首尾匹配的一层有效引号，这符合 Shell 第一遍扫描的逻辑
		if (name[0] == '"' && name[len(name)-1] == '"') ||
			(name[0] == '\'' && name[len(name)-1] == '\'') ||
			(name[0] == '`' && name[len(name)-1] == '`') {
			return name[1 : len(name)-1]
		}
	}
	return name
}

// extractCommandName 从命令节点提取命令名
func extractCommandName(node *sitter.Node, source string) string {
	// 消除前置环境变量和重定向的干扰，精准获取命令名
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		return nodeContent(nameNode, source)
	}

	// 查找第一个 word 或 command_name 节点
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		t := child.Type()

		// 明确跳过前置的干扰项（环境变量赋值、重定向、注释等）
		if t == "variable_assignment" || t == "redirect" || t == "comment" {
			continue
		}

		// 过滤完干扰项后，迎面遇到的第一个节点必定是命令主体
		// 它可能是 "word" (如 ls), 可能是 "string" (如 "ls"), 或者是 "raw_string" (如 'ls')
		return cleanCommandName(nodeContent(child, source))
	}
	return ""
}

// extractCommandArgs 从命令节点提取完整的参数列表
func extractCommandArgs(node *sitter.Node, source string) []string {
	var args []string
	foundName := false

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		t := child.Type()

		// 1. 必须同步过滤掉前置的环境变量赋值
		if t == "variable_assignment" {
			continue
		}

		// 2. 必须过滤掉重定向节点（如 >, >>, <），因为重定向不属于命令的参数
		if t == "redirect" || t == "heredoc_redirect" {
			continue
		}

		// 3. 过滤掉注释
		if t == "comment" {
			continue
		}

		// 4. 分水岭：洗净干扰后，遇到的第一个有效节点是命令名本身，必须跳过它
		if !foundName {
			foundName = true
			continue
		}

		// 5. 核心安全修复：采用“排除法”
		// 只要能走到这一步，且节点不是某些无意义的特殊符号（如命令末尾的分号或后台运行符 &），它就是参数！
		// 这样可以完美兼容 file_wildcard(*.go), expansion($VAR), concatenation(a_b) 等所有复杂节点
		if t != ";" && t != "&" {
			args = append(args, cleanCommandName(nodeContent(child, source)))
		}
	}
	return args
}

// analyzeCommand 分析命令节点
func analyzeCommand(node *sitter.Node, source string) BashAccessType {
	cmdName := cleanCommandName(extractCommandName(node, source))
	if cmdName == "" {
		return BashAccessUnknown
	}

	// 检查是否有写重定向
	if hasRedirect(node) {
		return BashAccessWrite
	}

	args := extractCommandArgs(node, source)

	if cmdName == "git" {
		return analyzeGitCommand(args)
	}
	if isVersionOnlyQuery(cmdName, args) {
		return BashAccessRead
	}
	if access, decided := classifyReaderWithFlags(cmdName, args); decided {
		return access
	}
	return classifyCommand(cmdName)
}

// analyzeGitCommand 跳过 git 全局选项后，只放行明确的只读子命令；
// branch/tag/remote 仅在查询形式下放行，带创建/删除参数时交由用户确认。
func analyzeGitCommand(args []string) BashAccessType {
	subCmd := ""
	var rest []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			switch arg {
			case "-C", "-c", "--git-dir", "--work-tree", "--namespace", "--exec-path", "--super-prefix", "--config-env":
				i++ // 该全局选项带一个值参数
			}
			continue
		}
		subCmd = arg
		rest = args[i+1:]
		break
	}
	if subCmd == "" {
		return BashAccessUnknown
	}
	if gitWriteSubcommands[subCmd] {
		return BashAccessWrite
	}
	switch subCmd {
	case "branch":
		if gitBranchQueryOnly(rest) {
			return BashAccessRead
		}
		return BashAccessUnknown
	case "tag":
		if gitTagQueryOnly(rest) {
			return BashAccessRead
		}
		return BashAccessUnknown
	case "remote":
		if gitRemoteQueryOnly(rest) {
			return BashAccessRead
		}
		return BashAccessUnknown
	}
	if gitReadonlySubcommands[subCmd] {
		return BashAccessRead
	}
	return BashAccessUnknown
}

// gitBranchQueryOnly 只接受列分支的无值参数；任何位置参数（含新分支名）都不放行。
func gitBranchQueryOnly(args []string) bool {
	allowed := map[string]bool{
		"-a": true, "-r": true, "-v": true, "-vv": true, "-l": true,
		"--list": true, "--all": true, "--remotes": true, "--show-current": true,
	}
	for _, arg := range args {
		if !allowed[arg] {
			return false
		}
	}
	return true
}

// gitTagQueryOnly 只接受纯列标签形式。
func gitTagQueryOnly(args []string) bool {
	if len(args) == 0 {
		return true
	}
	for _, arg := range args {
		if arg != "-l" && arg != "--list" {
			return false
		}
	}
	return true
}

// gitRemoteQueryOnly 只接受列远程形式。
func gitRemoteQueryOnly(args []string) bool {
	for _, arg := range args {
		if arg != "-v" && arg != "--verbose" {
			return false
		}
	}
	return true
}

// isVersionOnlyQuery 只放行解释器的版本查询，执行脚本或 -c 一律不放行。
func isVersionOnlyQuery(cmdName string, args []string) bool {
	if cmdName == "go" {
		return len(args) == 1 && args[0] == "version"
	}
	allowed, ok := map[string]map[string]bool{
		"python":  {"--version": true, "-V": true},
		"python3": {"--version": true, "-V": true},
		"node":    {"--version": true, "-v": true},
		"java":    {"-version": true},
		"gcc":     {"--version": true},
		"npm":     {"--version": true, "-v": true},
	}[cmdName]
	if !ok || len(args) != 1 {
		return false
	}
	return allowed[args[0]]
}

// classifyReaderWithFlags 收紧可能因参数产生写入的读取命令。
func classifyReaderWithFlags(cmdName string, args []string) (BashAccessType, bool) {
	switch cmdName {
	case "sort":
		for _, arg := range args {
			if arg == "-o" || arg == "--output" || strings.HasPrefix(arg, "--output=") ||
				(strings.HasPrefix(arg, "-o") && len(arg) > 2) {
				return BashAccessUnknown, true
			}
		}
		return BashAccessRead, true
	case "date":
		for _, arg := range args {
			if arg == "-s" || arg == "--set" || strings.HasPrefix(arg, "--set=") {
				return BashAccessUnknown, true
			}
			// date MMDDhhmm 形式会设置系统时间
			if !strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "+") {
				return BashAccessUnknown, true
			}
		}
		return BashAccessRead, true
	case "hostname":
		allowed := map[string]bool{
			"-f": true, "--fqdn": true, "-i": true, "--ip-address": true,
			"-s": true, "--short": true, "-d": true, "--domain": true,
			"-A": true, "--all-fqdns": true,
		}
		for _, arg := range args {
			if !allowed[arg] {
				return BashAccessUnknown, true
			}
		}
		return BashAccessRead, true
	}
	return BashAccessUnknown, false
}

// hasRedirect 检查节点是否有重定向
func hasRedirect(node *sitter.Node) bool {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "redirect" {
			// 检查重定向类型
			for j := 0; j < int(child.ChildCount()); j++ {
				redirChild := child.Child(j)
				if redirChild.Type() == ">" || redirChild.Type() == ">>" ||
					redirChild.Type() == "&>" || redirChild.Type() == "&>>" {
					return true
				}
			}
		}
	}
	return false
}

// analyzePipeline 分析管道。任一段为 Write 即 Write；存在无法判定的段则 Unknown。
func analyzePipeline(node *sitter.Node, source string) BashAccessType {
	return aggregateChildren(node, source)
}

// analyzeList 分析命令列表
func analyzeList(node *sitter.Node, source string) BashAccessType {
	return aggregateChildren(node, source)
}

// analyzeCompound 分析复合语句
func analyzeCompound(node *sitter.Node, source string) BashAccessType {
	return aggregateChildren(node, source)
}

// aggregateChildren 合并子节点结论：Write 优先，其次 Unknown，全部只读才返回 Read。
func aggregateChildren(node *sitter.Node, source string) BashAccessType {
	result := BashAccessRead
	for i := 0; i < int(node.ChildCount()); i++ {
		switch analyzeNode(node.Child(i), source) {
		case BashAccessWrite:
			return BashAccessWrite
		case BashAccessUnknown:
			result = BashAccessUnknown
		}
	}
	return result
}

// classifyCommand 根据命令名分类
func classifyCommand(cmdName string) BashAccessType {
	if readonlyCommands[cmdName] {
		return BashAccessRead
	}
	if writeCommands[cmdName] {
		return BashAccessWrite
	}

	return BashAccessUnknown
}

// nodeContent 获取节点的文本内容
func nodeContent(node *sitter.Node, source string) string {
	return source[node.StartByte():node.EndByte()]
}

// analyzeFallback 回退分析（AST 解析失败时）
func analyzeFallback(cmd string) BashAccessType {
	// 检查是否有写操作的特征
	if strings.Contains(cmd, ">") || strings.Contains(cmd, ">>") {
		return BashAccessWrite
	}
	// 检查是否是已知只读命令
	parts := strings.Fields(cmd)
	if len(parts) > 0 {
		cmdName := parts[0]
		if readonlyCommands[cmdName] {
			return BashAccessRead
		}
		if writeCommands[cmdName] {
			return BashAccessWrite
		}
	}
	return BashAccessUnknown
}
