package context

import "testing"

func TestNewCreatesRequestScopedManager(t *testing.T) {
	first := New(nil)
	first.AppendSlot("request", "first", 0)

	second := New(nil)
	if got := second.Build(); got != "" {
		t.Fatalf("new manager inherited slots from another request: %q", got)
	}
}
