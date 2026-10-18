package mcp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"

	"github.com/cloudwego/eino/components/tool"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPManager 管理已配置的 MCP 客户端会话，并把远端工具适配给上层。
//
// Close 和 ConnectAll 通过 closed 标志保护会话表，保证关闭后不会再安装新会话：
// 迟到的连接会自行关闭会话，而不是泄漏。
type MCPManager struct {
	mu      sync.RWMutex
	config  *MCPConfig
	servers map[string]*mcp.ClientSession
	closed  bool
}

// newTransport 按配置构建真实的 SDK 传输层。
func newTransport(config *MCPServerConfig) (mcp.Transport, error) {
	switch config.Transport {
	case TransportStdio:
		if config.Command == "" {
			return nil, fmt.Errorf("stdio MCP server requires command")
		}
		cmd := exec.Command(config.Command, config.Args...)
		cmd.Env = mergeEnv(os.Environ(), config.Env)
		return &mcp.CommandTransport{Command: cmd}, nil
	case TransportSSE:
		return &persistentSSETransport{
			delegate: &mcp.SSEClientTransport{
				Endpoint:   config.URL,
				HTTPClient: newHTTPClient(config.URL, config.Headers)}}, nil
	case TransportStreamableHTTP:
		return &mcp.StreamableClientTransport{
			Endpoint:   config.URL,
			HTTPClient: newHTTPClient(config.URL, config.Headers)}, nil
	default:
		return nil, fmt.Errorf("unsupported MCP transport: %s", config.Transport)
	}
}

// SDK 旧版 SSE 的 GET 会继承建立连接时的 context。这里让已建立的流在启动阶段
// 结束后继续存活，同时仍能取消被阻塞的连接尝试。
type persistentSSETransport struct {
	delegate *mcp.SSEClientTransport
}

func (t *persistentSSETransport) Connect(ctx context.Context) (mcp.Connection, error) {
	lifecycleCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	stop := context.AfterFunc(ctx, cancel)
	connection, err := t.delegate.Connect(lifecycleCtx)
	stop()
	if err != nil {
		cancel()
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		cancel()
		_ = connection.Close()
		return nil, err
	}
	return &persistentSSEConnection{Connection: connection, cancel: cancel}, nil
}

type persistentSSEConnection struct {
	mcp.Connection
	cancel context.CancelFunc
}

func (c *persistentSSEConnection) Close() error {
	defer c.cancel()
	return c.Connection.Close()
}

// mergeEnv 在 base 之后追加配置的环境变量，使配置值覆盖继承值。
//
// 这里选择追加而不是原地合并：exec 对同名键保留最后一次出现（Windows 上不区分大小写）。
// 若原地合并，Windows 上大小写不同的继承项可能残留，并被不区分大小写的子进程取到。
func mergeEnv(base []string, configured map[string]string) []string {
	entries := make([]string, 0, len(base)+len(configured))
	entries = append(entries, base...)
	for key, value := range configured {
		entries = append(entries, key+"="+os.ExpandEnv(value))
	}
	// 配置 map 的遍历顺序是随机的，排序以保证子进程环境变量顺序确定。
	slices.Sort(entries[len(base):])
	return entries
}

// newHTTPClient 返回一个只向配置的 origin 发送配置请求头的客户端；
// 返回 nil 表示让 SDK 使用 http.DefaultClient。
func newHTTPClient(endpoint string, headers map[string]string) *http.Client {
	if len(headers) == 0 {
		return nil
	}
	expanded := make(map[string]string, len(headers))
	for key, value := range headers {
		expanded[key] = os.ExpandEnv(value)
	}
	return &http.Client{Transport: &headerRoundTripper{
		base:    http.DefaultTransport,
		headers: expanded,
		origin:  originOf(endpoint),
	}}
}

// headerRoundTripper 只向配置的 origin 添加配置的请求头。
//
// net/http 在跟随跨域重定向之前会剥离敏感头（Authorization、Cookie、
// WWW-Authenticate），正是为了防止令牌被转发到其他主机。这里若重新添加就会
// 破坏该保护；而旧版 SSE 传输在运行时才从服务端获知消息端点，目标地址事先未知。
// 因此发往其他 origin 的请求会直接失败，而不是泄漏密钥。
type headerRoundTripper struct {
	base    http.RoundTripper
	headers map[string]string
	origin  string
}

func (t *headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if !t.allows(req) {
		return nil, fmt.Errorf("refusing to send configured MCP headers to %s (outside %s)", req.URL, t.origin)
	}
	if len(t.headers) == 0 {
		return t.base.RoundTrip(req)
	}
	// 克隆请求，避免修改调用方的原始请求。
	clone := req.Clone(req.Context())
	for key, value := range t.headers {
		clone.Header.Set(key, value)
	}
	return t.base.RoundTrip(clone)
}

// allows 判断请求是否指向配置的 origin。
func (t *headerRoundTripper) allows(req *http.Request) bool {
	if t.origin == "" || t.origin == "://" {
		return true
	}
	return req.URL.Scheme+"://"+req.URL.Host == t.origin
}

// originOf 返回 endpoint 的 scheme://host 部分，也就是重定向时允许变化的部分。
func originOf(endpoint string) string {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func NewMCPManager() *MCPManager {
	return &MCPManager{servers: make(map[string]*mcp.ClientSession)}
}

// SetConfig 安装一份已经加载好的配置。
// 主要用于嵌入 GoAgent 以及需要确定性的测试。
func (m *MCPManager) SetConfig(config *MCPConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

// Init 初始化：创建配置目录并加载配置。
func (m *MCPManager) Init() error {
	if err := InitConfigDir(); err != nil {
		return fmt.Errorf("初始化配置目录失败: %w", err)
	}

	config, err := LoadMCPConfig()
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}
	m.SetConfig(config)

	enabled := config.GetEnabledServers()
	fmt.Printf("已加载 %d 个 MCP 服务器配置，%d 个已启用\n", len(config.Servers), len(enabled))
	return nil
}

// ConnectAll 连接所有已启用的服务器。
//
// 连接并发执行；允许部分连接失败的调用方可以保留成功的连接；
// 生产启动路径会在任意失败时关闭管理器。
func (m *MCPManager) ConnectAll(ctx context.Context) error {
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return fmt.Errorf("MCP manager is closed")
	}
	if m.config == nil {
		m.mu.RUnlock()
		return fmt.Errorf("MCP manager is not initialized")
	}
	enabled := m.config.GetEnabledServers()
	m.mu.RUnlock()

	names := make([]string, 0, len(enabled))
	for name := range enabled {
		names = append(names, name)
	}
	slices.Sort(names)

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	for _, name := range names {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			if err := m.connectServer(ctx, name, enabled[name]); err != nil {
				mu.Lock()
				defer mu.Unlock()
				errs = append(errs, fmt.Errorf("connect %s: %w", name, err))
			}
		}(name)
	}
	wg.Wait()

	if len(errs) > 0 {
		slices.SortFunc(errs, func(a, b error) int { return strings.Compare(a.Error(), b.Error()) })
		return errors.Join(errs...)
	}
	return nil
}

// connectServer 连接单个服务器
func (m *MCPManager) connectServer(ctx context.Context, name string, config *MCPServerConfig) error {
	transport, err := newTransport(config)
	if err != nil {
		return err
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "goagent", Version: "1.0.0"}, nil)
	cs, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	// installSession 会保存会话，或报告管理器已关闭。这里持锁开销很小：
	// SDK 会话已经建立，锁内没有阻塞操作。
	old, installed := m.installSession(name, cs)
	if !installed {
		// 关闭流程与本次连接竞争：在这里关闭会话，避免泄漏，并说明丢弃原因。
		return errors.Join(fmt.Errorf("MCP manager is closed, session for %s discarded", name), cs.Close())
	}
	if old != nil {
		_ = old.Close()
	}
	return nil
}

// installSession 在管理器尚未关闭时保存会话。
func (m *MCPManager) installSession(name string, cs *mcp.ClientSession) (old *mcp.ClientSession, installed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, false
	}
	old = m.servers[name]
	m.servers[name] = cs
	return old, true
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
	result := make(map[string]*mcp.ClientSession, len(m.servers))
	for name, session := range m.servers {
		result[name] = session
	}
	return result
}

// Tools 把所有已连接服务器的工具适配给 Eino 的模型/工具循环。
//
// 按确定顺序（排序）遍历服务器。两个工具导出同名时直接报错，避免分发歧义：
// 否则模型会调用先注册的那个适配器。
func (m *MCPManager) Tools(ctx context.Context) ([]tool.BaseTool, error) {
	sessions := m.GetAllSessions()

	serverNames := make([]string, 0, len(sessions))
	for serverName := range sessions {
		serverNames = append(serverNames, serverName)
	}
	slices.Sort(serverNames)

	tools := make([]tool.BaseTool, 0)
	seen := make(map[string]string, len(sessions))
	for _, serverName := range serverNames {
		remoteTools, err := m.ListTools(ctx, serverName)
		if err != nil {
			return nil, fmt.Errorf("list tools from %s: %w", serverName, err)
		}
		for _, remote := range remoteTools {
			adapted, err := newMCPTool(m, serverName, remote)
			if err != nil {
				return nil, err
			}
			info := adapted.info
			if owner, ok := seen[info.Name]; ok {
				return nil, fmt.Errorf("MCP tool name collision: %q is exported by both %q and %q", info.Name, owner, serverName)
			}
			seen[info.Name] = serverName
			tools = append(tools, adapted)
		}
	}
	return tools, nil
}

// ListTools 列出某个已连接 MCP 服务器暴露的工具。
//
// 迭代器会遍历所有分页，因此分页返回工具列表的服务器也能被完整发现。
func (m *MCPManager) ListTools(ctx context.Context, serverName string) ([]*mcp.Tool, error) {
	session, exists := m.GetSession(serverName)
	if !exists {
		return nil, fmt.Errorf("服务器 %s 未连接", serverName)
	}

	var tools []*mcp.Tool
	for remoteTool, err := range session.Tools(ctx, nil) {
		if err != nil {
			return nil, err
		}
		tools = append(tools, remoteTool)
	}
	return tools, nil
}

// CallTool 在某个已连接的 MCP 服务器上调用工具。
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
		if server == nil {
			continue
		}
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

// Close 关闭所有连接。
//
// 在触碰任何会话之前先把管理器标记为已关闭，这样正在建立中的连接安装的会话不可达，
// 会由该连接自行关闭，而不是泄漏。会话在释放锁之后关闭，因为 Close 会阻塞等待对端。
func (m *MCPManager) Close() error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	servers := m.servers
	m.servers = make(map[string]*mcp.ClientSession)
	m.mu.Unlock()

	errs := make([]error, 0, len(servers))
	for name, session := range servers {
		if err := session.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close %s: %w", name, err))
		}
	}
	if joined := errors.Join(errs...); joined != nil {
		return fmt.Errorf("close MCP servers: %w", joined)
	}
	return nil
}

// ServerStatus 服务器状态
type ServerStatus struct {
	Name        string `json:"name"`
	Enabled     bool   `json:"enabled"`
	State       string `json:"state"` // disabled、disconnected、connected
	Transport   string `json:"transport"`
	Description string `json:"description"`
}

// PrintStatus 打印状态信息
func (m *MCPManager) PrintStatus() {
	status := m.GetStatus()

	fmt.Println("\n=== MCP 服务器状态 ===")
	fmt.Printf("%-20s %-10s %-15s %-10s %s\n", "名称", "启用", "状态", "传输", "描述")
	fmt.Println("----------------------------------------------------------------------")

	for _, name := range sortedStatusNames(status) {
		s := status[name]
		fmt.Printf("%-20s %-10t %-15s %-10s %s\n",
			s.Name, s.Enabled, s.State, s.Transport, s.Description)
	}
	fmt.Println()
}

func sortedStatusNames(status map[string]ServerStatus) []string {
	names := make([]string, 0, len(status))
	for name := range status {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
