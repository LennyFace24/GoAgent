package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMCPConfigLoadReportsInvalidFiles(t *testing.T) {
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
	write(`{"servers":`)
	if _, err := LoadMCPConfig(); err == nil {
		t.Fatal("invalid config was silently accepted")
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

func TestMCPConfigLoadsEnabledServersFromSelectedDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "selected")
	t.Setenv("GOAGENT_CONFIG", dir)
	if err := InitConfigDir(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "mcp_servers.json")
	data := `{"version":"1.0","servers":{"enabled":{"enabled":true,"transport":"stdio","command":"server","env":{"TOKEN":"${SECRET}"}},"disabled":{"enabled":false,"transport":"sse","url":"http://127.0.0.1/sse"},"empty":null}}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
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
	if _, ok := loaded.Servers["empty"]; ok {
		t.Fatal("null server remained in the loaded configuration")
	}
}
