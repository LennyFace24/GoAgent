package config

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func resetConfigForTest() {
	config = nil
	once = sync.Once{}
	loadErr = nil
}

func writeTestConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoadConfig(t *testing.T) {
	resetConfigForTest()
	t.Cleanup(resetConfigForTest)

	path := writeTestConfig(t, `
server:
    host: 127.0.0.1
    port: 8080
embedding:
    base_url: https://embedding.example.com/v1
    api_key: test-key
    model: test-embedding
session:
    secret_key: test-secret
`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadConfig returned nil")
	}
	if cfg.Server.Port != "8080" {
		t.Fatalf("Server.Port = %q, want 8080", cfg.Server.Port)
	}
}

func TestConfigDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.ApplyDefaults()

	if cfg.Server.Host != "127.0.0.1" {
		t.Fatalf("Server.Host = %q, want 127.0.0.1", cfg.Server.Host)
	}
	if cfg.Server.Port != "8080" {
		t.Fatalf("Server.Port = %q, want 8080", cfg.Server.Port)
	}
	// ApplyDefaults 未配置 workspace.root 时回退到当前目录
	if cfg.Workspace.Root != "." {
		t.Fatalf("Workspace.Root = %q, want %q", cfg.Workspace.Root, ".")
	}
}

func TestValidateRejectsNilAndMissingSecretKey(t *testing.T) {
	if err := (*Config)(nil).Validate(); err == nil || !strings.Contains(err.Error(), "nil") {
		t.Fatalf("nil Validate err = %v, want nil error", err)
	}
	c := &Config{}
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "secret_key") {
		t.Fatalf("missing secret_key err = %v, want secret_key error", err)
	}
}

func TestLoadConfigFailsOnMissingSecretKey(t *testing.T) {
	resetConfigForTest()
	t.Cleanup(resetConfigForTest)

	// 故意省略 session.secret_key
	path := writeTestConfig(t, `
server:
    host: 127.0.0.1
    port: 8080
`)
	cfg, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected validation error for missing secret_key, got nil")
	}
	if cfg != nil {
		t.Fatalf("expected nil config on validation error, got %+v", cfg)
	}
	if !strings.Contains(err.Error(), "secret_key") {
		t.Fatalf("error = %q, want substring 'secret_key'", err.Error())
	}
}
