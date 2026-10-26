package sandbox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	maxWorktreeOutputBytes = 100 * 1024
	maxWorktreeOutputLines = 1000
)

// LocalWorktree 是本地 git worktree 沙箱。
// 它隔离源码分支和工作目录，但不是进程/网络安全沙箱。
type LocalWorktree struct {
	ID       string
	RepoRoot string
	Path     string
	WorkDir  string
	Branch   string
}

// CreateLocalWorktree 从当前 HEAD 创建独立分支和 worktree。
// 未提交的工作区修改不会进入新 worktree。
func CreateLocalWorktree(workspaceRoot string) (*LocalWorktree, error) {
	if strings.TrimSpace(workspaceRoot) == "" {
		workspaceRoot = "."
	}
	absWorkspace, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("解析工作区路径失败: %w", err)
	}

	repoRoot, err := gitOutput(absWorkspace, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("workspace 不是 git 仓库: %w", err)
	}
	if _, err := gitOutput(repoRoot, "rev-parse", "--verify", "HEAD"); err != nil {
		return nil, fmt.Errorf("git 仓库还没有提交，无法创建 worktree: %w", err)
	}

	rel, err := filepath.Rel(repoRoot, absWorkspace)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("workspace 必须位于 git 仓库内: %s", absWorkspace)
	}

	base, err := os.MkdirTemp("", "goagent-worktree-")
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}
	path := filepath.Join(base, "tree")
	branch := "goagent/sandbox-" + uuid.NewString()[:8]
	if _, err := gitOutput(repoRoot, "worktree", "add", "-b", branch, path, "HEAD"); err != nil {
		_ = os.RemoveAll(base)
		return nil, fmt.Errorf("创建 git worktree 失败: %w", err)
	}

	workDir := path
	if rel != "." {
		workDir = filepath.Join(path, rel)
	}
	return &LocalWorktree{
		ID:       "wt_" + uuid.NewString()[:8],
		RepoRoot: repoRoot,
		Path:     path,
		WorkDir:  workDir,
		Branch:   branch,
	}, nil
}

func (w *LocalWorktree) Execute(ctx context.Context, command string, timeout time.Duration) (string, error) {
	if strings.TrimSpace(command) == "" {
		return "", errors.New("command is empty")
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if timeout > 120*time.Second {
		timeout = 120 * time.Second
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	shell, args := localShell(command)
	cmd := exec.CommandContext(execCtx, shell, args...)
	cmd.Dir = w.WorkDir
	output, err := cmd.CombinedOutput()
	result := truncateWorktreeOutput(strings.TrimSpace(string(output)))
	if err != nil {
		return result, err
	}
	return result, nil
}

// Destroy 移除 worktree，但保留分支供检查和合并。
func (w *LocalWorktree) Destroy() error {
	output, err := exec.Command("git", "-C", w.RepoRoot, "worktree", "remove", "--force", w.Path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("移除 worktree 失败: %s: %w", strings.TrimSpace(string(output)), err)
	}
	_ = os.RemoveAll(filepath.Dir(w.Path))
	return nil
}

func gitOutput(dir string, args ...string) (string, error) {
	cmdArgs := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w", strings.TrimSpace(string(output)), err)
	}
	return strings.TrimSpace(string(output)), nil
}

func localShell(command string) (string, []string) {
	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("sh"); err == nil {
			return "sh", []string{"-c", command}
		}
		return "cmd.exe", []string{"/C", command}
	}
	return "sh", []string{"-c", command}
}

func truncateWorktreeOutput(s string) string {
	if len(s) <= maxWorktreeOutputBytes {
		lines := strings.Split(s, "\n")
		if len(lines) <= maxWorktreeOutputLines {
			return s
		}
		return strings.Join(lines[:maxWorktreeOutputLines], "\n") +
			fmt.Sprintf("\n\n[输出被截断，原共 %d 行，仅显示前 %d 行]", len(lines), maxWorktreeOutputLines)
	}
	return s[:maxWorktreeOutputBytes] +
		fmt.Sprintf("\n\n[输出被截断，原共 %d 字节，仅显示前 %d 字节]", len(s), maxWorktreeOutputBytes)
}
