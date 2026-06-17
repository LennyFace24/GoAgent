package basictool

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

// ---------- Bash Security Validator ----------

type validator struct {
	name    string
	pattern *regexp.Regexp
	severity string // "severe" = 直接 deny, "suspicious" = 可升级为 ask
}

var validators = []validator{
	// severe: 直接拒绝
	{"sudo", regexp.MustCompile(`\bsudo\b`), "severe"},
	{"rm_rf", regexp.MustCompile(`\brm\s+(-[a-zA-Z]*)?r`), "severe"},
	{"mkfs", regexp.MustCompile(`\bmkfs\b`), "severe"},
	{"dd_if", regexp.MustCompile(`\bdd\s+if=`), "severe"},
	{"chmod_777_root", regexp.MustCompile(`chmod\s+777\s+/`), "severe"},
	{"chown_root", regexp.MustCompile(`chown\s+-R\s+.*/`), "severe"},
	{"iptables_flush", regexp.MustCompile(`iptables\s+-F`), "severe"},
	{"fork_bomb", regexp.MustCompile(`:\(\)\s*\{.*:.*\|.*:.*\}`), "severe"},
	{"shutdown", regexp.MustCompile(`\b(shutdown|reboot|halt|poweroff)\b`), "severe"},
	{"dev_null_write", regexp.MustCompile(`>\s*/dev/`), "severe"},

	// suspicious: 可疑但可由用户确认
	{"shell_metachar", regexp.MustCompile(`[;&|` + "`" + `]`), "suspicious"},
	{"cmd_substitution", regexp.MustCompile(`\$\(`), "suspicious"},
	{"ifs_injection", regexp.MustCompile(`\bIFS\s*=`), "suspicious"},
	{"nohup_disown", regexp.MustCompile(`\b(nohup|disown)\b`), "suspicious"},
	{"background_redirect", regexp.MustCompile(`&>/dev/null\s*&`), "suspicious"},
	{"pipe_to_shell", regexp.MustCompile(`\|\s*(sh|bash)`), "suspicious"},
}

type ValidationResult struct {
	Severe   bool     // true = 直接拒绝, false = 可疑但可让用户确认
	Failures []string // 命中的规则名称列表
}

func validateBash(cmd string) ValidationResult {
	var failures []string
	severe := false
	for _, v := range validators {
		if v.pattern.MatchString(cmd) {
			failures = append(failures, v.name)
			if v.severity == "severe" {
				severe = true
			}
		}
	}
	return ValidationResult{Severe: severe, Failures: failures}
}

// ValidateBash 导出给权限系统使用
func ValidateBash(cmd string) ValidationResult {
	return validateBash(cmd)
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

func NewBashTool(workspaceRoot string) (tool.InvokableTool, error) {
	return utils.InferTool(
		"bash",
		"在服务器上执行 bash 命令并返回输出。命令默认在 agent 工作区执行；如需查看源码，请显式使用源码路径。受危险命令拦截、输出大小、超时限制保护。",
		func(ctx context.Context, input BashInput) (string, error) {
			if strings.TrimSpace(input.Command) == "" {
				return "错误: 命令为空", nil
			}
			vr := validateBash(input.Command)
			if vr.Severe {
				auditBash(input.Command, fmt.Errorf("blocked: %v", vr.Failures))
				return fmt.Sprintf("错误: 命令被安全策略拦截（触发规则: %s）", strings.Join(vr.Failures, ", ")), nil
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
			if workspaceRoot != "" {
				workdir := filepath.Clean(workspaceRoot)
				if err := os.MkdirAll(workdir, 0755); err != nil {
					return fmt.Sprintf("错误: 创建工作区失败 %v", err), nil
				}
				cmd.Dir = workdir
			}
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
