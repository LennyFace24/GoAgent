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

func TestLoadConfigNormalizesLLMContextBudget(t *testing.T) {
	resetConfigForTest()
	t.Cleanup(resetConfigForTest)

	path := writeTestConfig(t, `
server:
    host: 127.0.0.1
    port: 8080
llm:
    base_url: https://example.com/v1
    api_key: test-key
    model: test-model
    context_window: 200000
    max_completion_tokens: 8192
    safety_margin_tokens: 4096
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

	if cfg.Llm.ContextWindow != 200000 {
		t.Fatalf("ContextWindow = %d, want 200000", cfg.Llm.ContextWindow)
	}
	if cfg.Llm.SafetyMarginTokens != 4096 {
		t.Fatalf("SafetyMarginTokens = %d, want 4096", cfg.Llm.SafetyMarginTokens)
	}
	if got, want := cfg.ContextBudgetTokens(), int64(187712); got != want {
		t.Fatalf("ContextBudgetTokens() = %d, want %d", got, want)
	}
}

func TestConfigDefaultsContextWindowWhenMissing(t *testing.T) {
	cfg := &Config{}
	cfg.ApplyDefaults()

	if cfg.Llm.ContextWindow != DefaultContextWindow {
		t.Fatalf("ContextWindow = %d, want %d", cfg.Llm.ContextWindow, DefaultContextWindow)
	}
	if got := cfg.ContextBudgetTokens(); got != int64(DefaultContextWindow) {
		t.Fatalf("ContextBudgetTokens() = %d, want %d", got, int64(DefaultContextWindow))
	}
	// ApplyDefaults 未配置 workspace.root 时回退到当前目录
	if cfg.Workspace.Root != "." {
		t.Fatalf("Workspace.Root = %q, want %q", cfg.Workspace.Root, ".")
	}
}

func TestValidateRejectsMissingRequiredFields(t *testing.T) {
	cases := []struct {
		name    string
		setup   func(c *Config)
		wantSub string
	}{
		{"nil", nil, "nil"},
		{"missing api_key", func(c *Config) { c.Llm.BaseUrl = "u"; c.Llm.Model = "m" }, "api_key"},
		{"missing base_url", func(c *Config) { c.Llm.ApiKey = "k"; c.Llm.Model = "m" }, "base_url"},
		{"missing model", func(c *Config) { c.Llm.BaseUrl = "u"; c.Llm.ApiKey = "k" }, "model"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var c *Config
			if tc.setup != nil {
				c = &Config{}
				tc.setup(c)
			}
			err := c.Validate()
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantSub)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tc.wantSub)
			}
		})
	}
}

func TestLoadConfigFailsOnMissingSecretKey(t *testing.T) {
	resetConfigForTest()
	t.Cleanup(resetConfigForTest)

	// 故意省略 session.secret_key
	path := writeTestConfig(t, `
llm:
    base_url: https://example.com/v1
    api_key: test-key
    model: test-model
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
