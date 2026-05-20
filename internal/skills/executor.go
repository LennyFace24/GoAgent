package skills

import (
	"context"
	"os/exec"
	"strings"
)

// SkillExecutor 执行 skill 中定义的脚本。
type SkillExecutor struct{}

// NewSkillExecutor 创建脚本执行器。
func NewSkillExecutor() *SkillExecutor {
	return &SkillExecutor{}
}

// Execute 在给定工作目录下执行命令，返回 stdout+stderr。
func (e *SkillExecutor) Execute(ctx context.Context, workDir string, command string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	if workDir != "" {
		cmd.Dir = workDir
	}
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}
