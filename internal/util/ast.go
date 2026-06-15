package util

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/bash"
)

// BashAccessType bash 命令的访问类型
type BashAccessType int

const (
	BashAccessRead  BashAccessType = iota // 只读操作
	BashAccessWrite                       // 写操作
	BashAccessUnknown                     // 未知
)

// 只读命令白名单
var readonlyCommands = map[string]bool{
	// 文件查看
	"cat": true, "head": true, "tail": true, "less": true, "more": true,
	"strings": true, "hexdump": true, "od": true, "nl": true,
	// 文件信息
	"wc": true, "stat": true, "file": true, "readlink": true, "realpath": true,
	"basename": true, "dirname": true,
	// 文本处理
	"cut": true, "paste": true, "tr": true, "column": true, "tac": true,
	"rev": true, "fold": true, "expand": true, "unexpand": true, "fmt": true,
	"comm": true, "cmp": true, "numfmt": true, "sort": true, "uniq": true,
	"diff": true, "grep": true, "egrep": true, "fgrep": true, "rg": true,
	"awk": true, "sed": true,
	// 系统信息
	"id": true, "uname": true, "hostname": true, "date": true, "whoami": true,
	"pwd": true, "arch": true, "locale": true, "groups": true, "nproc": true,
	"uptime": true, "df": true, "du": true, "free": true, "top": true,
	"ps": true, "env": true, "printenv": true, "which": true, "type": true,
	"tree": true, "ls": true, "dir": true, "find": true,
	// 版本查询
	"node": true, "python": true, "python3": true, "go": true,
	"java": true, "gcc": true, "make": true,
	// 网络只读
	"ping": true, "traceroute": true, "nslookup": true, "dig": true,
	"curl": true, "wget": true,
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

// git 只读子命令
var gitReadonlySubcommands = map[string]bool{
	"status": true, "log": true, "show": true, "diff": true, "blame": true,
	"branch": true, "tag": true, "ls-files": true, "remote": true,
	"describe": true, "rev-parse": true, "rev-list": true,
	"count-objects": true, "cat-file": true, "ls-tree": true,
}

// git 写子命令
var gitWriteSubcommands = map[string]bool{
	"add": true, "commit": true, "push": true, "pull": true, "merge": true,
	"rebase": true, "reset": true, "checkout": true, "stash": true,
	"cherry-pick": true, "revert": true, "clean": true, "rm": true,
}

// AnalyzeBashCommand 使用 tree-sitter AST 分析 bash 命令的访问类型
func AnalyzeBashCommand(cmd string) BashAccessType {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return BashAccessUnknown
	}

	// 解析为 AST
	parser := sitter.NewParser()
	parser.SetLanguage(bash.GetLanguage())
	tree, err := parser.ParseCtx(nil, nil, []byte(cmd))
	if err != nil {
		return analyzeFallback(cmd)
	}
	defer tree.Close()

	root := tree.RootNode()

	// 遍历 AST 分析
	access := analyzeNode(root, cmd)
	return access
}

// analyzeNode 递归分析 AST 节点
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
		// 递归分析子节点
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			access := analyzeNode(child, source)
			if access == BashAccessWrite {
				return BashAccessWrite
			}
		}
	}
	return BashAccessRead
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

// analyzeCommand 分析命令节点
func analyzeCommand(node *sitter.Node, source string) BashAccessType {
	// 提取命令名
	cmdName := extractCommandName(node, source)
	if cmdName == "" {
		return BashAccessUnknown
	}

	// 检查是否有重定向
	if hasRedirect(node) {
		return BashAccessWrite
	}

	// 分类命令
	return classifyCommand(cmdName)
}

// extractCommandName 从命令节点提取命令名
func extractCommandName(node *sitter.Node, source string) string {
	// 查找第一个 word 或 command_name 节点
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "command_name" {
			return nodeContent(child, source)
		}
		if child.Type() == "word" {
			return nodeContent(child, source)
		}
	}
	return ""
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

// analyzePipeline 分析管道
func analyzePipeline(node *sitter.Node, source string) BashAccessType {
	// 管道中如果有写命令，则为写操作
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		access := analyzeNode(child, source)
		if access == BashAccessWrite {
			return BashAccessWrite
		}
	}
	return BashAccessRead
}

// analyzeList 分析命令列表
func analyzeList(node *sitter.Node, source string) BashAccessType {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		access := analyzeNode(child, source)
		if access == BashAccessWrite {
			return BashAccessWrite
		}
	}
	return BashAccessRead
}

// analyzeCompound 分析复合语句
func analyzeCompound(node *sitter.Node, source string) BashAccessType {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		access := analyzeNode(child, source)
		if access == BashAccessWrite {
			return BashAccessWrite
		}
	}
	return BashAccessRead
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
