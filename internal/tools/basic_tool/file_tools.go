package basictool

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

const (
	MaxFileReadBytes = 200 * 1024 // 200KB 单次读取上限
	maxFileWriteBytes = 1024 * 1024 // 1MB 单次写入上限
)

type ReadFileInput struct {
	Path   string `json:"path" description:"文件绝对路径或相对工作目录的路径" required:"true"`
	Offset int    `json:"offset" description:"起始行号(从1开始)，默认 1" required:"false"`
	Limit  int    `json:"limit" description:"读取最大行数，默认 2000，最大 5000" required:"false"`
}

type WriteFileInput struct {
	Path    string `json:"path" description:"文件路径，将完全覆盖现有内容" required:"true"`
	Content string `json:"content" description:"要写入的完整文件内容" required:"true"`
}

type EditFileInput struct {
	Path    string `json:"path" description:"要编辑的文件路径" required:"true"`
	OldText string `json:"old_text" description:"要被替换的原文（必须在文件中唯一）" required:"true"`
	NewText string `json:"new_text" description:"替换后的新文本" required:"true"`
}

func IsPathSafe(path string) error {
	if strings.Contains(path, "..") {
		return fmt.Errorf("路径不允许包含 ..")
	}
	dangerousPrefixes := []string{
		"/etc/", "/sys/", "/proc/", "/dev/", "/boot/",
		"/root/.ssh", "/var/log/auth", "/var/log/secure",
	}
	cleanPath := filepath.Clean(path)
	for _, p := range dangerousPrefixes {
		if strings.HasPrefix(cleanPath, p) {
			return fmt.Errorf("禁止访问敏感路径 %s", p)
		}
	}
	return nil
}

func resolveWorkspacePath(workspaceRoot, path string) string {
	if filepath.IsAbs(path) || workspaceRoot == "" {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(workspaceRoot, path))
}

func NewReadFileTool(workspaceRoot string) (tool.InvokableTool, error) {
	return utils.InferTool(
		"read_file",
		"读取本地文件内容。相对路径默认从 agent 工作区读取；如需查看源码，可传入源码文件的绝对路径。支持按行偏移和数量限制读取。",
		func(ctx context.Context, input ReadFileInput) (string, error) {
			path := resolveWorkspacePath(workspaceRoot, input.Path)
			if err := IsPathSafe(path); err != nil {
				return fmt.Sprintf("错误: %v", err), nil
			}

			info, err := os.Stat(path)
			if err != nil {
				return fmt.Sprintf("错误: %v", err), nil
			}
			if info.IsDir() {
				return "错误: 路径是目录，请使用 bash 工具的 ls 命令", nil
			}
			if info.Size() > MaxFileReadBytes*5 {
				return fmt.Sprintf("错误: 文件过大 (%d 字节)，超过限制", info.Size()), nil
			}

			f, err := os.Open(path)
			if err != nil {
				return fmt.Sprintf("错误: %v", err), nil
			}
			defer f.Close()

			offset := input.Offset
			if offset < 1 {
				offset = 1
			}
			limit := input.Limit
			if limit <= 0 {
				limit = 2000
			}
			if limit > 5000 {
				limit = 5000
			}

			scanner := bufio.NewScanner(f)
			scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

			var sb strings.Builder
			lineNum := 0
			read := 0
			totalBytes := 0

			for scanner.Scan() {
				lineNum++
				if lineNum < offset {
					continue
				}
				if read >= limit {
					break
				}
				line := scanner.Text()
				totalBytes += len(line) + 1
				if totalBytes > MaxFileReadBytes {
					sb.WriteString(fmt.Sprintf("\n[输出截断: 超过 %d 字节]", MaxFileReadBytes))
					break
				}
				sb.WriteString(fmt.Sprintf("%6d\t%s\n", lineNum, line))
				read++
			}

			if err := scanner.Err(); err != nil {
				return fmt.Sprintf("错误: 读取过程出错 %v", err), nil
			}

			if sb.Len() == 0 {
				return fmt.Sprintf("(文件为空或起始行号 %d 超出范围)", offset), nil
			}
			return sb.String(), nil
		},
	)
}

func NewWriteFileTool(workspaceRoot string) (tool.InvokableTool, error) {
	return utils.InferTool(
		"write_file",
		"创建或完全覆盖一个文件。相对路径默认写入 agent 工作区；如需写源码，请显式传入绝对路径。优先使用 edit_file 修改现有文件。",
		func(ctx context.Context, input WriteFileInput) (string, error) {
			path := resolveWorkspacePath(workspaceRoot, input.Path)
			if err := IsPathSafe(path); err != nil {
				return fmt.Sprintf("错误: %v", err), nil
			}
			if len(input.Content) > maxFileWriteBytes {
				return fmt.Sprintf("错误: 内容过大 (%d 字节)，超过限制", len(input.Content)), nil
			}

			if dir := filepath.Dir(path); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return fmt.Sprintf("错误: 创建目录失败 %v", err), nil
				}
			}

			if err := os.WriteFile(path, []byte(input.Content), 0644); err != nil {
				return fmt.Sprintf("错误: 写入失败 %v", err), nil
			}

			return fmt.Sprintf("已写入 %s (%d 字节)", path, len(input.Content)), nil
		},
	)
}

func NewEditFileTool(workspaceRoot string) (tool.InvokableTool, error) {
	return utils.InferTool(
		"edit_file",
		"在已有文件中精确替换文本。相对路径默认定位到 agent 工作区；如需编辑源码，请显式传入绝对路径。old_text 必须在文件中唯一出现一次，否则会报错。",
		func(ctx context.Context, input EditFileInput) (string, error) {
			path := resolveWorkspacePath(workspaceRoot, input.Path)
			if err := IsPathSafe(path); err != nil {
				return fmt.Sprintf("错误: %v", err), nil
			}
			if input.OldText == "" {
				return "错误: old_text 不能为空", nil
			}
			if input.OldText == input.NewText {
				return "错误: old_text 与 new_text 相同，无需修改", nil
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Sprintf("错误: 读取文件失败 %v", err), nil
			}

			content := string(data)
			count := strings.Count(content, input.OldText)
			if count == 0 {
				return "错误: old_text 在文件中未找到", nil
			}
			if count > 1 {
				return fmt.Sprintf("错误: old_text 在文件中匹配到 %d 处，必须唯一。请扩大上下文使其唯一", count), nil
			}

			newContent := strings.Replace(content, input.OldText, input.NewText, 1)

			if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
				return fmt.Sprintf("错误: 写入失败 %v", err), nil
			}

			return fmt.Sprintf("已修改 %s", path), nil
		},
	)
}
