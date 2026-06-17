package util

import (
	"strings"
	"sync"
	"context"
	"log"

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

// AnalyzeBashCommand 使用 tree-sitter AST 分析 bash 命令的访问类型
func AnalyzeBashCommand(cmd string) BashAccessType {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return BashAccessUnknown
	}

	// 使用 defer + recover 捕获所有 panic（保留你原本的安全兜底）
	defer func() {
		if r := recover(); r != nil {
			// 打印日志
			log.Printf("[FATAL] Tree-sitter panicked during parsing: %v", r)
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
	if nameNode := node.ChildByFieldName("name");nameNode != nil {
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
	// 提取命令名
	cmdName := extractCommandName(node, source)
	if cmdName == "" {
		return BashAccessUnknown
	}

	// 检查是否有重定向
	if hasRedirect(node) {
		return BashAccessWrite
	}

	args := extractCommandArgs(node, source)

	// 特殊处理 bash 命令：如果参数是数字，自动放行
	if cmdName == "bash" || cmdName == "sh" || cmdName == "zsh" || cmdName == "dash" {
		if len(args) > 0 && isNumeric(args[0]) {
			return BashAccessRead
		}
	}

	// 分类命令
	if cmdName == "git" {
		// 核心安全修复：过滤掉 git 的全局选项（如 -C, --git-dir 等）
        var subCmd string
        for _, arg := range args {
            if !strings.HasPrefix(arg, "-") {
                subCmd = arg
                break
            }
        }

        // 如果连子命令都没找到，说明语法不完整或未知
        if subCmd == "" {
            return BashAccessUnknown
        }

        // 校验精准路由
        if gitReadonlySubcommands[subCmd] {
            return BashAccessRead
        }
        if gitWriteSubcommands[subCmd] {
            return BashAccessWrite
        }
        
        // 未知的 git 子命令（例如 fetch, clone），显式返回 Unknown，不交给通用函数
        return BashAccessUnknown
	}
	return classifyCommand(cmdName)
}


// isNumeric 检查字符串是否是纯数字
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
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
