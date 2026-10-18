package commands

import (
	"context"
	"strings"
	"testing"

	contexttool "github.com/LennyFace24/MiniAgent/internal/tools/context_tool"
)

func TestContextCommandReportsRequestConversation(t *testing.T) {
	registry := contexttool.GetContextRegistry()
	registry.Reset()
	registry.SetMaxTokens(1000)
	t.Cleanup(registry.Reset)

	registry.State(contexttool.ScopeKey("session", "a")).SetUsage(120, 4)

	output, err := contextHandler(contexttool.WithScope(context.Background(), "session", "a"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "120/1000") || !strings.Contains(output, "消息数: 4") {
		t.Fatalf("unexpected context output: %q", output)
	}

	other, err := contextHandler(contexttool.WithScope(context.Background(), "session", "b"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(other, "0/1000") {
		t.Fatalf("unused conversation should be empty: %q", other)
	}
}
