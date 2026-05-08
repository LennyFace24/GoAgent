package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

const (
	maxBashOutputBytes = 100 * 1024 // 100KB
	maxBashOutputLines = 1000
	bashAuditLogPath   = "data/bash_audit.log"
)

type BashInput struct {
	Command string `json:"command" description:"要执行的 bash 命令" required:"true"`
	Timeout int    `json:"timeout" description:"超时时间（秒），默认 30，最大 120" required:"false"`
}

func isDangerous(cmd string) bool {
	dangerous := []string{
		"rm -rf /", "sudo", "shutdown", "reboot", "halt", "poweroff",
		"> /dev/", "mkfs", "dd if=", ":(){ :|:& };:",
		"chmod 777 /", "chown -R", "iptables -F",
		"nohup", "disown", "&>/dev/null &",
	}
	for _, d := range dangerous {
		if strings.Contains(cmd, d) {
			return true
		}
	}
	return false
}

func truncateOutput(s string) string {
	if len(s) <= maxBashOutputBytes {
		lines := strings.Split(s, "\n")
		if len(lines) <= maxBashOutputLines {
			return s
		}
		return strings.Join(lines[:maxBashOutputLines], "\n") +
			fmt.Sprintf("\n\n[输出被截断，原共 %d 行，仅显示前 %d 行]", len(lines), maxBashOutputLines)
	}
	return s[:maxBashOutputBytes] +
		fmt.Sprintf("\n\n[输出被截断，原共 %d 字节，仅显示前 %d 字节]", len(s), maxBashOutputBytes)
}

func auditBash(command string, exitErr error) {
	_ = os.MkdirAll(filepath.Dir(bashAuditLogPath), 0755)
	f, err := os.OpenFile(bashAuditLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	status := "ok"
	if exitErr != nil {
		status = fmt.Sprintf("err=%v", exitErr)
	}
	fmt.Fprintf(f, "%s [%s] %s\n",
		time.Now().Format("2006-01-02 15:04:05"), status, command)
}

func NewBashTool() (tool.InvokableTool, error) {
	return utils.InferTool(
		"bash",
		"在服务器上执行 bash 命令并返回输出。用于查看系统状态、进程、日志、网络连接等。受危险命令拦截、输出大小、超时限制保护。",
		func(ctx context.Context, input BashInput) (string, error) {
			if strings.TrimSpace(input.Command) == "" {
				return "错误: 命令为空", nil
			}
			if isDangerous(input.Command) {
				auditBash(input.Command, fmt.Errorf("blocked"))
				return "错误: 命令被安全策略拦截，禁止执行危险操作", nil
			}

			timeout := 30
			if input.Timeout > 0 {
				timeout = input.Timeout
			}
			if timeout > 120 {
				timeout = 120
			}

			execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
			defer cancel()

			cmd := exec.CommandContext(execCtx, "sh", "-c", input.Command)
			output, err := cmd.CombinedOutput()

			auditBash(input.Command, err)

			result := strings.TrimSpace(string(output))
			result = truncateOutput(result)

			if err != nil {
				if result == "" {
					return fmt.Sprintf("[执行失败] %v", err), nil
				}
				return fmt.Sprintf("%s\n[退出码: %v]", result, err), nil
			}
			if result == "" {
				return "(命令执行成功，无输出)", nil
			}
			return result, nil
		},
	)
}
