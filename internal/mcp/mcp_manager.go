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

// MCPManager manages configured MCP client sessions and adapts their tools.
//
// Close and ConnectAll guard the session map with a closed flag so a session
// can never be installed after shutdown: a late connect closes its own session
// instead of leaking it.
type MCPManager struct {
	mu      sync.RWMutex
	config  *MCPConfig
	servers map[string]*mcp.ClientSession
	closed  bool
}

// newTransport builds the real SDK transport described by the config.
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
		return &persistentSSETransport{delegate: &mcp.SSEClientTransport{Endpoint: config.URL, HTTPClient: newHTTPClient(config.URL, config.Headers)}}, nil
	case TransportStreamableHTTP:
		return &mcp.StreamableClientTransport{Endpoint: config.URL, HTTPClient: newHTTPClient(config.URL, config.Headers)}, nil
	case TransportWebSocket:
		return nil, fmt.Errorf("websocket MCP transport is not supported by the Go MCP SDK")
	default:
		return nil, fmt.Errorf("unsupported MCP transport: %s", config.Transport)
	}
}

// The SDK's legacy SSE GET inherits its connect context. Keep the established
// stream alive after startup ends, but still cancel a blocked connection attempt.
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

// mergeEnv returns base with the configured entries appended, so a configured
// value wins over any inherited one.
//
// Entries are appended rather than merged in place: exec dedups keeping the
// last occurrence of each key (case-insensitively on Windows), so on Windows an
// inherited entry with different casing would otherwise survive and be picked
// up by a case-insensitive child.
func mergeEnv(base []string, configured map[string]string) []string {
	entries := make([]string, 0, len(base)+len(configured))
	entries = append(entries, base...)
	for key, value := range configured {
		entries = append(entries, key+"="+os.ExpandEnv(value))
	}
	// Order of the config map is random; sort for a deterministic child env.
	slices.Sort(entries[len(base):])
	return entries
}

// newHTTPClient returns a client that sends the configured headers to the
// configured origin, or nil to let the SDK use http.DefaultClient.
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

// headerRoundTripper adds configured headers, but only to the configured
// origin.
//
// net/http strips sensitive headers (Authorization, Cookie,
// WWW-Authenticate) before following a cross-origin redirect precisely so a
// token is not forwarded to another host. Re-applying them here would undo
// that, and the legacy SSE transport derives its message endpoint from the
// server at runtime, so the destination is not known up front. A request to
// any other origin therefore fails loudly instead of leaking the secret.
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
	// Clone so the caller's request is never mutated.
	clone := req.Clone(req.Context())
	for key, value := range t.headers {
		clone.Header.Set(key, value)
	}
	return t.base.RoundTrip(clone)
}

// allows reports whether the request targets the configured origin.
func (t *headerRoundTripper) allows(req *http.Request) bool {
	if t.origin == "" || t.origin == "://" {
		return true
	}
	return req.URL.Scheme+"://"+req.URL.Host == t.origin
}

// originOf returns the scheme://host part of endpoint, which is what a
// redirect may legitimately change.
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

// SetConfig installs a configuration for callers that already loaded one.
// It is primarily useful for embedding GoAgent and deterministic tests.
func (m *MCPManager) SetConfig(config *MCPConfig) {
	m.mu.Lock()
	m.config = config
	m.mu.Unlock()
}

// Init 初始化：加载配置
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

// ConnectAll 连接所有启用的服务器
//
// Connections run concurrently. Successes are retained for callers that allow
// partial connectivity; production startup closes the manager on any failure.
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
				errs = append(errs, fmt.Errorf("connect %s: %w", name, err))
				mu.Unlock()
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

	// installSession stores cs, or reports that the manager is closed. Holding
	// the lock here is cheap: the SDK session is already established, so no
	// blocking work happens under it.
	old, installed := m.installSession(name, cs)
	if !installed {
		// Shutdown raced with this connect: close the session here so it is
		// never leaked, and report why it was discarded.
		return errors.Join(fmt.Errorf("MCP manager is closed, session for %s discarded", name), cs.Close())
	}
	if old != nil {
		_ = old.Close()
	}
	return nil
}

// installSession stores the session unless the manager is already closed.
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

// Tools adapts all tools from connected servers for Eino's model/tool loop.
//
// Servers are visited in deterministic (sorted) order. Two tools that export
// the same name are rejected instead of leaving the dispatch ambiguous: the
// model would call whichever adapter came first.
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
			info, err := adapted.Info(ctx)
			if err != nil {
				return nil, fmt.Errorf("MCP server %s: %w", serverName, err)
			}
			if owner, ok := seen[info.Name]; ok {
				return nil, fmt.Errorf("MCP tool name collision: %q is exported by both %q and %q", info.Name, owner, serverName)
			}
			seen[info.Name] = serverName
			tools = append(tools, adapted)
		}
	}
	return tools, nil
}

// ListTools lists tools exposed by one connected MCP server.
//
// The iterator walks every page, so a server that paginates its tool list is
// fully discovered.
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

// CallTool calls a tool on one connected MCP server.
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

// Reload reloads the configuration and replaces the in-memory config.
// Existing sessions are intentionally kept until the caller reconnects or closes them.
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

// Close 关闭所有连接
//
// The manager is marked closed before any session is touched, so a session
// installed by an in-flight connect is not reachable and is closed by that
// connect instead of being leaked. Sessions are closed after the lock is
// released because Close blocks on the peer.
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
