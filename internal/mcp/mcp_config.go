package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MCPTransport 传输类型
type MCPTransport string

const (
	TransportStdio          MCPTransport = "stdio"
	TransportSSE            MCPTransport = "sse"
	TransportStreamableHTTP MCPTransport = "streamable-http"
)

// MCPServerConfig 单个 MCP Server 配置
type MCPServerConfig struct {
	Enabled     bool              `json:"enabled"`
	Transport   MCPTransport      `json:"transport"`
	Command     string            `json:"command,omitempty"`     // stdio: 可执行命令
	Args        []string          `json:"args,omitempty"`        // stdio: 命令参数
	Env         map[string]string `json:"env,omitempty"`         // stdio: 环境变量
	URL         string            `json:"url,omitempty"`         // sse/streamable-http: 服务地址
	Headers     map[string]string `json:"headers,omitempty"`     // sse/streamable-http: 请求头
	Description string            `json:"description,omitempty"` // 描述信息
}

// MCPConfig MCP 配置文件结构
type MCPConfig struct {
	Version string                      `json:"version"`
	Servers map[string]*MCPServerConfig `json:"servers"`
}

// GetConfigDir 获取用户配置目录
func GetConfigDir() (string, error) {
	// 优先使用环境变量
	if dir := os.Getenv("GOAGENT_CONFIG"); dir != "" {
		return dir, nil
	}

	// 获取用户主目录
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("获取用户目录失败: %w", err)
	}

	return filepath.Join(home, ".goagent"), nil
}

// GetMCPConfigPath 获取 MCP 配置文件路径
func GetMCPConfigPath() (string, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "mcp_servers.json"), nil
}

// InitConfigDir 初始化配置目录
func InitConfigDir() error {
	dir, err := GetConfigDir()
	if err != nil {
		return err
	}

	// 创建目录（如果不存在）
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	// 检查 MCP 配置文件是否存在
	configPath := filepath.Join(dir, "mcp_servers.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// 复制默认配置
		if err := createDefaultMCPConfig(configPath); err != nil {
			return fmt.Errorf("创建默认配置失败: %w", err)
		}
		fmt.Printf("已创建默认配置文件: %s\n", configPath)
	}

	return nil
}

// createDefaultMCPConfig 创建默认 MCP 配置文件
func createDefaultMCPConfig(path string) error {
	defaultConfig := MCPConfig{
		Version: "1.0",
		Servers: map[string]*MCPServerConfig{
			"example-stdio": {
				Enabled:     false,
				Transport:   TransportStdio,
				Command:     "npx",
				Args:        []string{"-y", "mcp-conv2prompt"},
				Description: "示例：本地 stdio 服务",
			},
			"example-sse": {
				Enabled:     false,
				Transport:   TransportSSE,
				URL:         "http://localhost:8080/sse",
				Description: "示例：远程 SSE 服务",
			},
		},
	}

	data, err := json.MarshalIndent(defaultConfig, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// LoadMCPConfig 按顺序读取第一个存在的配置文件：优先可执行文件旁的便携
// 配置，其次用户配置目录（可用 GOAGENT_CONFIG 覆盖）。只有“文件不存在”
// 才会继续找下一个位置；读不到或解析不了都是错误。所有位置都没有文件时
// 返回空的默认配置。
func LoadMCPConfig() (*MCPConfig, error) {
	var candidates []string
	// 可执行文件目录下的自定义配置优先
	if exePath, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exePath), "config", "mcp_servers.json"))
	}
	if path, err := GetMCPConfigPath(); err == nil {
		candidates = append(candidates, path)
	}

	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("读取 MCP 配置失败 %s: %w", path, err)
		}
		config := &MCPConfig{}
		if err := json.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("解析 MCP 配置失败 %s: %w", path, err)
		}
		config.dropNilServers()
		fmt.Printf("已加载 MCP 配置: %s\n", path)
		return config, nil
	}

	fmt.Println("使用默认 MCP 配置")
	return getDefaultMCPConfig(), nil
}

// dropNilServers 删除 `"name": null` 这类没有配置内容的条目，避免调用方
// 读字段时 panic。
func (c *MCPConfig) dropNilServers() {
	if c == nil {
		return
	}
	for name, server := range c.Servers {
		if server == nil {
			delete(c.Servers, name)
		}
	}
}

// getDefaultMCPConfig 获取默认配置
func getDefaultMCPConfig() *MCPConfig {
	return &MCPConfig{
		Version: "1.0",
		Servers: make(map[string]*MCPServerConfig),
	}
}

// GetEnabledServers 获取所有启用的 Server
func (c *MCPConfig) GetEnabledServers() map[string]*MCPServerConfig {
	enabled := make(map[string]*MCPServerConfig)
	for name, server := range c.Servers {
		if server == nil {
			continue
		}
		if server.Enabled {
			enabled[name] = server
		}
	}
	return enabled
}
