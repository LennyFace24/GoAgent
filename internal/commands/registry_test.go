package commands

import "testing"

func TestRegistryRegisterAndList(t *testing.T) {
	r := &Registry{}
	r.Register(Command{Name: "foo", Description: "foo cmd"})
	r.Register(Command{Name: "bar", Description: "bar cmd"})

	list := r.List()
	if len(list) != 2 {
		t.Fatalf("len(List) = %d, want 2", len(list))
	}
	// List 返回副本，修改不影响内部状态
	list[0] = Command{Name: "tampered"}
	again := r.List()
	if again[0].Name != "foo" {
		t.Fatalf("internal state tampered via List() copy: got %q", again[0].Name)
	}
}

func TestRegistryFind(t *testing.T) {
	r := &Registry{}
	r.Register(Command{Name: "foo", Type: CommandTypeClient})

	cmd, ok := r.Find("foo")
	if !ok {
		t.Fatal("Find(foo) = false, want true")
	}
	if cmd.Type != CommandTypeClient {
		t.Fatalf("cmd.Type = %q, want client", cmd.Type)
	}

	if _, ok := r.Find("nope"); ok {
		t.Fatal("Find(nope) = true, want false")
	}
}

// TestBuiltinCommandsRegistered 验证内置命令集合，并回归保护 /new 已被移除。
func TestBuiltinCommandsRegistered(t *testing.T) {
	registry := GetRegistry()
	list := registry.List()

	want := map[string]bool{
		"help":    true,
		"clear":   true,
		"compact": true,
		"context": true,
		"model":   true,
	}
	got := map[string]bool{}
	for _, c := range list {
		got[c.Name] = true
	}

	for name := range want {
		if !got[name] {
			t.Errorf("内置命令缺失: %q (got=%v)", name, got)
		}
	}
	// /new 之前被移除，确保不会回归
	if got["new"] {
		t.Errorf("命令 /new 不应再注册")
	}
}
