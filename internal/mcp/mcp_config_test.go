package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMCPConfigReloadReadsDiskAndReportsInvalidFiles(t *testing.T) {
	t.Setenv("GOAGENT_CONFIG", t.TempDir())
	path, err := GetMCPConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	write := func(data string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(`{"version":"1.0","servers":{"first":{"enabled":false,"transport":"stdio","command":"first"}}}`)
	manager := NewMCPManager()
	if err := manager.Init(); err != nil {
		t.Fatal(err)
	}
	write(`{"version":"1.0","servers":{"second":{"enabled":false,"transport":"stdio","command":"second"}}}`)
	if err := manager.Reload(); err != nil {
		t.Fatal(err)
	}
	status := manager.GetStatus()
	if _, ok := status["second"]; !ok {
		t.Fatalf("Reload did not observe disk update: %v", status)
	}
	if _, ok := status["first"]; ok {
		t.Fatal("Reload retained removed configuration")
	}
	write(`{"servers":`)
	if err := manager.Reload(); err == nil {
		t.Fatal("invalid config silently replaced the active configuration")
	}
	if _, ok := manager.GetStatus()["second"]; !ok {
		t.Fatal("failed Reload discarded the last valid configuration")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadMCPConfig(); err == nil {
		t.Fatal("unreadable config path silently fell back")
	}
}

func TestMCPConfigSaveLoadsFromSelectedDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "selected")
	t.Setenv("GOAGENT_CONFIG", dir)
	if err := InitConfigDir(); err != nil {
		t.Fatal(err)
	}
	original := &MCPConfig{Version: "1.0", Servers: map[string]*MCPServerConfig{
		"enabled":  {Enabled: true, Transport: TransportStdio, Command: "server", Env: map[string]string{"TOKEN": "${SECRET}"}},
		"disabled": {Enabled: false, Transport: TransportSSE, URL: "http://127.0.0.1/sse"},
		"empty":    nil,
	}}
	if err := SaveMCPConfig(original); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadMCPConfig()
	if err != nil {
		t.Fatal(err)
	}
	enabled := loaded.GetEnabledServers()
	if len(enabled) != 1 || enabled["enabled"].Command != "server" || enabled["enabled"].Env["TOKEN"] != "${SECRET}" {
		t.Fatalf("wrong enabled config loaded from selected directory: %#v", enabled)
	}
	if _, err := os.Stat(filepath.Join(dir, "mcp_servers.json")); err != nil {
		t.Fatal("selected config was not saved:", err)
	}
}
