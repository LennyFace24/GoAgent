package contexttool

import (
	"context"
	"strings"
	"testing"
)

func TestConfigureContextStateUpdatesMaxTokens(t *testing.T) {
	state := NewContextState(DefaultMaxTokens)
	state.ConfigureMaxTokens(1000)
	state.SetTokens(250)

	usage := state.GetUsage()
	if usage.MaxTokens != 1000 {
		t.Fatalf("MaxTokens = %d, want 1000", usage.MaxTokens)
	}
	if usage.Percentage != 25 {
		t.Fatalf("Percentage = %.1f, want 25.0", usage.Percentage)
	}
}

func TestContextRegistryIsolatesConversations(t *testing.T) {
	registry := NewContextRegistry(1000)
	scopeA := ScopeKey("session", "a")
	scopeB := ScopeKey("session", "b")

	registry.State(scopeA).SetUsage(100, 2)
	if got := registry.Usage(scopeA); got.CurrentTokens != 100 || got.MessageCount != 2 {
		t.Fatalf("scope A usage = %+v", got)
	}
	if got := registry.Usage(scopeB); got.CurrentTokens != 0 || got.MessageCount != 0 || got.MaxTokens != 1000 {
		t.Fatalf("scope B should be empty: %+v", got)
	}

	registry.SetMaxTokens(2000)
	if got := registry.Usage(scopeB); got.MaxTokens != 2000 {
		t.Fatalf("new scope budget = %d, want 2000", got.MaxTokens)
	}
	if got := registry.Usage(scopeA); got.MaxTokens != 2000 {
		t.Fatalf("existing scope budget = %d, want 2000", got.MaxTokens)
	}

	registry.Delete(scopeA)
	if got := registry.Usage(scopeA); got.CurrentTokens != 0 {
		t.Fatalf("deleted scope retained state: %+v", got)
	}
}

func TestContextStatusToolUsesRequestScope(t *testing.T) {
	registry := GetContextRegistry()
	registry.Reset()
	registry.SetMaxTokens(1000)
	t.Cleanup(registry.Reset)

	registry.State(ScopeKey("session", "a")).SetUsage(42, 3)

	tool, err := NewContextStatusTool()
	if err != nil {
		t.Fatal(err)
	}
	output, err := tool.InvokableRun(WithScope(context.Background(), "session", "a"), `{}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "42 / 1000") || !strings.Contains(output, "3") {
		t.Fatalf("tool used the wrong scope: %q", output)
	}

	other, err := tool.InvokableRun(WithScope(context.Background(), "session", "b"), `{}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(other, "0 / 1000") {
		t.Fatalf("unused conversation should start at zero: %q", other)
	}
}
