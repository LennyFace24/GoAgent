package extratool

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LennyFace24/MiniAgent/internal/tools/basic_tool"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)



type TodoWriterInput struct {
	Path    string `json:"path" description:"待办事项文件路径" required:"true"`
	Content string `json:"content" description:"待办事项文档内容，使用 Markdown 格式" required:"true"`
}

func NewTodoWriter() (tool.InvokableTool, error) {
	return utils.InferTool(
		"write_todo",
		`将 AI 生成的待办事项文档写入指定文件。
期望的文档格式（Markdown）：
# 待办事项
## [状态图标] 任务标题
- 描述：具体要做什么
- 预期产出：完成标准
- 状态：pending / in_progress / completed
---
状态图标：⬜待办 🔄进行中 ✅已完成
AI 应根据用户需求生成结构化的、可执行的任务清单，而不是简单罗列。`,
		func(ctx context.Context, input TodoWriterInput) (string, error) {
			if err := basictool.IsPathSafe(input.Path); err != nil {
				return fmt.Sprintf("错误: %v", err), nil
			}
			if len(input.Content) == 0 {
				return "错误: 待办事项内容为空", nil
			}

			if dir := filepath.Dir(input.Path); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return fmt.Sprintf("错误: 创建目录失败 %v", err), nil
				}
			}

			if err := os.WriteFile(input.Path, []byte(input.Content), 0644); err != nil {
				return fmt.Sprintf("错误: 写入失败 %v", err), nil
			}

			return fmt.Sprintf("已写入 %s (%d 字节)", input.Path, len(input.Content)), nil
		},
	)
}

func NewTodoReader() (tool.InvokableTool, error) {
	return utils.InferTool(
		"read_todo",
		"读取待办事项文件内容，查看当前任务进度。返回带行号的 Markdown 文本，包含各任务的状态（⬜待办/🔄进行中/✅已完成）。",
		func(ctx context.Context, input basictool.ReadFileInput) (string, error) {
			if err := basictool.IsPathSafe(input.Path); err != nil {
				return fmt.Sprintf("错误: %v", err), nil
			}

			info, err := os.Stat(input.Path)
			if err != nil {
				return fmt.Sprintf("错误: %v", err), nil
			}
			if info.IsDir() {
				return "错误: 路径是目录，请使用 bash 工具的 ls 命令", nil
			}
			if info.Size() > basictool.MaxFileReadBytes*5 {
				return fmt.Sprintf("错误: 文件过大 (%d 字节)，超过限制", info.Size()), nil
			}

			f, err := os.Open(input.Path)
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
				if totalBytes > basictool.MaxFileReadBytes {
					fmt.Fprintf(&sb, "\n[输出截断: 超过 %d 字节]", basictool.MaxFileReadBytes)
					break
				}
				fmt.Fprintf(&sb, "%6d\t%s\n", lineNum, line)
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