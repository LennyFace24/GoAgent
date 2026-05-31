package mcp

import (
	"context"
	"fmt"
	"os/exec"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPManager MCP Server 管理器
type MCPManager struct {
	mu      sync.RWMutex
	config  *MCPConfig
	servers map[string]*mcp.ClientSession
}

// NewMCPManager 创建管理器实例
func NewMCPManager() *MCPManager {
	return &MCPManager{
		servers: make(map[string]*mcp.ClientSession),
	}
}

// Init 初始化：加载配置
func (m *MCPManager) Init() error {
	// 初始化配置目录
	if err := InitConfigDir(); err != nil {
		return fmt.Errorf("初始化配置目录失败: %w", err)
	}

	// 加载配置
	config, err := LoadMCPConfig()
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	m.mu.Lock()
	m.config = config
	m.mu.Unlock()

	enabled := config.GetEnabledServers()
	fmt.Printf("已加载 %d 个 MCP 服务器配置，%d 个已启用\n",
		len(config.Servers), len(enabled))

	return nil
}

// ConnectAll 连接所有启用的服务器
func (m *MCPManager) ConnectAll(ctx context.Context) error {
	m.mu.RLock()
	enabled := m.config.GetEnabledServers()
	m.mu.RUnlock()

	var wg sync.WaitGroup
	errChan := make(chan error, len(enabled))

	for name, server := range enabled {
		wg.Add(1)
		go func(name string, server *MCPServerConfig) {
			defer wg.Done()
			if err := m.connectServer(ctx, name, server); err != nil {
				errChan <- fmt.Errorf("连接 %s 失败: %w", name, err)
			}
		}(name, server)
	}

	wg.Wait()
	close(errChan)

	// 收集错误
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("部分服务器连接失败: %v", errs)
	}

	return nil
}

// connectServer 连接单个服务器
func (m *MCPManager) connectServer(ctx context.Context, name string, config *MCPServerConfig) error {
	fmt.Printf("连接 MCP 服务器: %s (%s)\n", name, config.Transport)

	// 创建传输层
	var transport mcp.Transport
	switch config.Transport {
	case TransportStdio:
		cmd := exec.Command(config.Command, config.Args...)
		transport = &mcp.CommandTransport{Command: cmd}
	case TransportSSE:
		transport = &mcp.StreamableClientTransport{Endpoint: config.URL}
	default:
		return fmt.Errorf("不支持的传输类型: %s", config.Transport)
	}

	// 创建客户端
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "goagent",
		Version: "1.0.0",
	}, nil)

	// 连接
	cs, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}

	m.mu.Lock()
	m.servers[name] = cs
	m.mu.Unlock()

	fmt.Printf("✓ 已连接: %s\n", name)
	return nil
}

// GetSession 获取已连接的会话
func (m *MCPManager) GetSession(name string) (*mcp.ClientSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.servers[name]
	return session, exists
}

// GetAllSessions 获取所有已连接的会话
func (m *MCPManager) GetAllSessions() map[string]*mcp.ClientSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*mcp.ClientSession)
	for name, session := range m.servers {
		result[name] = session
	}
	return result
}

// ListTools 列出服务器提供的工具
func (m *MCPManager) ListTools(ctx context.Context, serverName string) ([]*mcp.Tool, error) {
	session, exists := m.GetSession(serverName)
	if !exists {
		return nil, fmt.Errorf("服务器 %s 未连接", serverName)
	}

	var tools []*mcp.Tool
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			return nil, err
		}
		tools = append(tools, tool)
	}

	return tools, nil
}

// CallTool 调用工具
func (m *MCPManager) CallTool(ctx context.Context, serverName, toolName string, args map[string]any) (*mcp.CallToolResult, error) {
	session, exists := m.GetSession(serverName)
	if !exists {
		return nil, fmt.Errorf("服务器 %s 未连接", serverName)
	}

	return session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
}

// GetStatus 获取所有服务器状态
func (m *MCPManager) GetStatus() map[string]ServerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make(map[string]ServerStatus)

	if m.config == nil {
		return status
	}

	for name, server := range m.config.Servers {
		s := ServerStatus{
			Name:        name,
			Enabled:     server.Enabled,
			Transport:   string(server.Transport),
			Description: server.Description,
		}

		if server.Enabled {
			if _, connected := m.servers[name]; connected {
				s.State = "connected"
			} else {
				s.State = "disconnected"
			}
		} else {
			s.State = "disabled"
		}

		status[name] = s
	}

	return status
}

// Reload 重新加载配置
func (m *MCPManager) Reload() error {
	config, err := LoadMCPConfig()
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.config = config
	m.mu.Unlock()

	return nil
}

// Close 关闭所有连接
func (m *MCPManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, session := range m.servers {
		if err := session.Close(); err != nil {
			fmt.Printf("关闭 %s 失败: %v\n", name, err)
		}
	}

	m.servers = make(map[string]*mcp.ClientSession)
	return nil
}

// ServerStatus 服务器状态
type ServerStatus struct {
	Name        string `json:"name"`
	Enabled     bool   `json:"enabled"`
	State       string `json:"state"` // disabled, disconnected, connected
	Transport   string `json:"transport"`
	Description string `json:"description"`
}

// PrintStatus 打印状态信息
func (m *MCPManager) PrintStatus() {
	status := m.GetStatus()

	fmt.Println("\n=== MCP 服务器状态 ===")
	fmt.Printf("%-20s %-10s %-15s %-10s %s\n", "名称", "启用", "状态", "传输", "描述")
	fmt.Println("----------------------------------------------------------------------")

	for _, s := range status {
		fmt.Printf("%-20s %-10t %-15s %-10s %s\n",
			s.Name, s.Enabled, s.State, s.Transport, s.Description)
	}
	fmt.Println()
}
