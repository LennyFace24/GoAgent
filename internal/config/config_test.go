package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func resetConfigForTest() {
	config = nil
	once = sync.Once{}
}

func TestLoadConfigNormalizesLLMContextBudget(t *testing.T) {
	resetConfigForTest()
	t.Cleanup(resetConfigForTest)

	path := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte(`
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
`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg := LoadConfig(path)
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
		t.Fatalf("ContextBudgetTokens() = %d, want %d", got, DefaultContextWindow)
	}
}
