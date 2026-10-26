package sandboxtool

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/LennyFace24/MiniAgent/internal/sandbox"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type SandBoxToolInput struct {
	Mode        string `json:"mode" description:"create 时必填：local 使用当前仓库的 git worktree；remote 使用配置的远程沙箱服务" required:"false"`
	Operation   string `json:"operation" description:"操作类型：create、execute 或 destroy" required:"true"`
	Cmd         string `json:"cmd" description:"execute 时要执行的命令" required:"false"`
	SandboxID   string `json:"sandbox_id" description:"create 返回的沙箱 ID，execute/destroy 时使用" required:"false"`
	ContainerID string `json:"container_id" description:"兼容旧字段，等价于 sandbox_id" required:"false"`
	Timeout     int    `json:"timeout" description:"本地命令超时秒数，默认 30，最大 120" required:"false"`
}

type sandboxEntry struct {
	mode   string
	remote *sandbox.SandBox
	local  *sandbox.LocalWorktree
}

type sandboxManager struct {
	endpoint      string
	workspaceRoot string

	mu      sync.Mutex
	entries map[string]*sandboxEntry
}

func NewSandBoxTool(endpoint, workspaceRoot string) (tool.InvokableTool, error) {
	m := &sandboxManager{
		endpoint:      strings.TrimSpace(endpoint),
		workspaceRoot: workspaceRoot,
		entries:       make(map[string]*sandboxEntry),
	}
	return utils.InferTool(
		"sandbox_tool",
		"创建和管理隔离沙箱。根据用户需求选择 mode：local 使用当前 Git 仓库的 git worktree（适合本地改代码、跑测试）；remote 使用配置中的远程沙箱服务（适合容器隔离）。create 必须指定 mode。",
		m.handle,
	)
}

func (m *sandboxManager) handle(ctx context.Context, input SandBoxToolInput) (string, error) {
	switch strings.ToLower(strings.TrimSpace(input.Operation)) {
	case "create":
		return m.create(input)
	case "execute":
		return m.execute(ctx, input)
	case "destroy":
		return m.destroy(input)
	default:
		return "", fmt.Errorf("未知操作类型 %q，可选 create、execute、destroy", input.Operation)
	}
}

func (m *sandboxManager) create(input SandBoxToolInput) (string, error) {
	mode := normalizeMode(input.Mode)
	switch mode {
	case "local":
		worktree, err := sandbox.CreateLocalWorktree(m.workspaceRoot)
		if err != nil {
			return "", err
		}
		m.mu.Lock()
		m.entries[worktree.ID] = &sandboxEntry{mode: "local", local: worktree}
		m.mu.Unlock()
		return fmt.Sprintf(
			"本地 worktree 创建完成\nsandbox_id=%s\npath=%s\nbranch=%s\n注意：基于当前 HEAD 创建，未提交的工作区修改不会带入。",
			worktree.ID, worktree.Path, worktree.Branch,
		), nil

	case "remote":
		if m.endpoint == "" {
			return "", fmt.Errorf("remote 沙箱未配置 sandbox.endpoint")
		}
		remote := sandbox.CreateContainer(m.endpoint)
		if remote == nil {
			return "", fmt.Errorf("创建远程沙箱容器失败")
		}
		m.mu.Lock()
		m.entries[remote.ContainerID] = &sandboxEntry{mode: "remote", remote: remote}
		m.mu.Unlock()
		return "远程沙箱创建完成\nsandbox_id=" + remote.ContainerID, nil

	default:
		return "", fmt.Errorf("create 必须指定 mode=local 或 mode=remote")
	}
}

func (m *sandboxManager) execute(ctx context.Context, input SandBoxToolInput) (string, error) {
	id := sandboxID(input)
	if id == "" {
		return "", fmt.Errorf("execute 需要 sandbox_id")
	}
	entry, ok := m.lookup(id)
	if !ok {
		return "", fmt.Errorf("沙箱 %s 不存在或已销毁", id)
	}
	if strings.TrimSpace(input.Cmd) == "" {
		return "", fmt.Errorf("execute 需要 cmd")
	}

	if entry.mode == "local" {
		timeout := time.Duration(input.Timeout) * time.Second
		output, err := entry.local.Execute(ctx, input.Cmd, timeout)
		if err != nil {
			if output == "" {
				return "", fmt.Errorf("命令执行失败: %w", err)
			}
			return fmt.Sprintf("%s\n[执行失败: %v]", output, err), nil
		}
		if output == "" {
			return "(命令执行成功，无输出)", nil
		}
		return output, nil
	}

	response := entry.remote.Execute(input.Cmd)
	if response.Error != "" {
		return "", fmt.Errorf("执行命令失败: %s", response.Error)
	}
	if response.Output == "" {
		return "(命令执行成功，无输出)", nil
	}
	return response.Output, nil
}

func (m *sandboxManager) destroy(input SandBoxToolInput) (string, error) {
	id := sandboxID(input)
	if id == "" {
		return "", fmt.Errorf("destroy 需要 sandbox_id")
	}
	entry, ok := m.lookup(id)
	if !ok {
		return "", fmt.Errorf("沙箱 %s 不存在或已销毁", id)
	}

	if entry.mode == "local" {
		if err := entry.local.Destroy(); err != nil {
			return "", err
		}
		m.mu.Lock()
		delete(m.entries, id)
		m.mu.Unlock()
		return fmt.Sprintf("worktree 已移除；分支 %s 保留，可检查或合并", entry.local.Branch), nil
	}

	if err := entry.remote.Destroy(); err != nil {
		return "", fmt.Errorf("销毁容器失败: %v", err)
	}
	m.mu.Lock()
	delete(m.entries, id)
	m.mu.Unlock()
	return "容器已销毁", nil
}

func (m *sandboxManager) lookup(id string) (*sandboxEntry, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.entries[id]
	return entry, ok
}

func sandboxID(input SandBoxToolInput) string {
	id := strings.TrimSpace(input.SandboxID)
	if id == "" {
		id = strings.TrimSpace(input.ContainerID)
	}
	return id
}

func normalizeMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "local", "worktree", "git":
		return "local"
	case "remote", "docker", "container":
		return "remote"
	default:
		return ""
	}
}
