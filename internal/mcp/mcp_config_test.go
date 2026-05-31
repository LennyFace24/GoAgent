package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGetConfigDir(t *testing.T) {
	dir, err := GetConfigDir()
	if err != nil {
		t.Fatalf("GetConfigDir() error: %v", err)
	}

	if dir == "" {
		t.Fatal("GetConfigDir() returned empty string")
	}

	t.Logf("Config directory: %s", dir)
}

func TestLoadMCPConfig(t *testing.T) {
	config, err := LoadMCPConfig()
	if err != nil {
		t.Fatalf("LoadMCPConfig() error: %v", err)
	}

	if config == nil {
		t.Fatal("LoadMCPConfig() returned nil")
	}

	t.Logf("Loaded config version: %s", config.Version)
	t.Logf("Servers count: %d", len(config.Servers))
}

func TestGetEnabledServers(t *testing.T) {
	config, err := LoadMCPConfig()
	if err != nil {
		t.Fatalf("LoadMCPConfig() error: %v", err)
	}

	enabled := config.GetEnabledServers()
	t.Logf("Enabled servers: %d", len(enabled))

	for name := range enabled {
		t.Logf("  - %s", name)
	}
}

func TestSaveAndLoadMCPConfig(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mcp_servers.json")

	// 创建测试配置
	original := &MCPConfig{
		Version: "1.0",
		Servers: map[string]*MCPServerConfig{
			"test-server": {
				Enabled:     true,
				Transport:   TransportStdio,
				Command:     "echo",
				Args:        []string{"hello"},
				Description: "Test server",
			},
		},
	}

	// 序列化
	data, err := json.MarshalIndent(original, "", "  ")
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	// 写入文件
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	// 读取并解析
	loaded, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	parsed := &MCPConfig{}
	if err := json.Unmarshal(loaded, parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	// 验证
	if parsed.Version != original.Version {
		t.Errorf("Version mismatch: got %s, want %s", parsed.Version, original.Version)
	}

	server, exists := parsed.Servers["test-server"]
	if !exists {
		t.Fatal("test-server not found")
	}

	if server.Command != "echo" {
		t.Errorf("Command mismatch: got %s, want echo", server.Command)
	}
}
