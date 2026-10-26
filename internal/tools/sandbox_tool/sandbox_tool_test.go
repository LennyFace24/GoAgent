package sandboxtool

import (
	"context"
	"strings"
	"testing"
)

func TestSandboxCreateRequiresMode(t *testing.T) {
	tool, err := NewSandBoxTool("", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tool.InvokableRun(context.Background(), `{"operation":"create"}`); err == nil {
		t.Fatal("create without mode should fail")
	}
}

func TestSandboxRemoteRequiresEndpoint(t *testing.T) {
	tool, err := NewSandBoxTool("", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tool.InvokableRun(context.Background(), `{"operation":"create","mode":"remote"}`); err == nil || !strings.Contains(err.Error(), "sandbox.endpoint") {
		t.Fatalf("remote without endpoint error = %v", err)
	}
}

func TestSandboxRejectsUnknownOperation(t *testing.T) {
	tool, err := NewSandBoxTool("", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tool.InvokableRun(context.Background(), `{"operation":"list"}`); err == nil {
		t.Fatal("unknown operation should fail")
	}
}
