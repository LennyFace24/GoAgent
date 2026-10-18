package permission

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestCheckAskRuleTakesPrecedenceOverAllowRule(t *testing.T) {
	pm := NewPermissionManager()
	pm.rules = []Rule{
		{Tool: "bash", Content: "npm install *", Behavior: BehaviorAsk},
		{Tool: "bash", Content: "npm *", Behavior: BehaviorAllow},
	}

	decision := pm.Check("", "bash", map[string]any{"command": "npm install left-pad"})

	if decision.Behavior != BehaviorAsk {
		t.Fatalf("expected ask decision, got %q (%s)", decision.Behavior, decision.Reason)
	}
}

func TestCheckReadOnlyBashIsAllowedBeforeAllowRules(t *testing.T) {
	pm := NewPermissionManager()
	pm.rules = nil

	decision := pm.Check("", "bash", map[string]any{"command": "pwd"})

	if decision.Behavior != BehaviorAllow {
		t.Fatalf("expected readonly bash to be allowed, got %q (%s)", decision.Behavior, decision.Reason)
	}
}

func TestCheckDenyRuleTakesPrecedenceOverReadOnlyAnalysis(t *testing.T) {
	pm := NewPermissionManager()
	pm.rules = []Rule{
		{Tool: "bash", Content: "pwd", Behavior: BehaviorDeny},
	}

	decision := pm.Check("", "bash", map[string]any{"command": "pwd"})

	if decision.Behavior != BehaviorDeny {
		t.Fatalf("expected deny decision, got %q (%s)", decision.Behavior, decision.Reason)
	}
}

func TestScopedAllowDoesNotLeakToAnotherConversation(t *testing.T) {
	pm := NewPermissionManager()
	pm.rules = nil
	scopeA := ScopeKey("session", "conversation-a")
	scopeB := ScopeKey("session", "conversation-b")

	if got := pm.Check(scopeA, "bash", map[string]any{"command": "custom-tool --flag"}); got.Behavior != BehaviorAsk {
		t.Fatalf("unknown command should ask, got %q", got.Behavior)
	}

	pm.AddRule(scopeA, RuleForInvocation("bash", map[string]any{"command": "custom-tool --flag"}))

	if got := pm.Check(scopeA, "bash", map[string]any{"command": "custom-tool --flag"}); got.Behavior != BehaviorAllow {
		t.Fatalf("approved command should be allowed in its conversation, got %q", got.Behavior)
	}
	if got := pm.Check(scopeB, "bash", map[string]any{"command": "custom-tool --flag"}); got.Behavior != BehaviorAsk {
		t.Fatalf("approval leaked into another conversation: %q", got.Behavior)
	}
}

func TestGlobalDenyCannotBeOverriddenByScopedAllow(t *testing.T) {
	pm := NewPermissionManager()
	pm.rules = []Rule{{Tool: "bash", Content: "git push *", Behavior: BehaviorDeny}}
	scope := ScopeKey("session", "conversation")

	pm.AddRule(scope, RuleForInvocation("bash", map[string]any{"command": "git push origin main"}))

	decision := pm.Check(scope, "bash", map[string]any{"command": "git push origin main"})
	if decision.Behavior != BehaviorDeny {
		t.Fatalf("scoped allow overrode a global deny: %q (%s)", decision.Behavior, decision.Reason)
	}
}

func TestAlwaysAllowMatchesOnlyTheApprovedInvocation(t *testing.T) {
	pm := NewPermissionManager()
	pm.rules = nil
	scope := ScopeKey("session", "conversation")

	pm.AddRule(scope, RuleForInvocation("bash", map[string]any{"command": "custom-script --flag"}))
	if got := pm.Check(scope, "bash", map[string]any{"command": "custom-script --flag"}); got.Behavior != BehaviorAllow {
		t.Fatalf("approved command not allowed: %q", got.Behavior)
	}
	if got := pm.Check(scope, "bash", map[string]any{"command": "custom-script --other"}); got.Behavior != BehaviorAsk {
		t.Fatalf("rule widened to unapproved arguments: %q", got.Behavior)
	}

	pm.AddRule(scope, RuleForInvocation("write_file", map[string]any{"path": "internal/a.txt"}))
	if got := pm.Check(scope, "write_file", map[string]any{"path": "internal/a.txt"}); got.Behavior != BehaviorAllow {
		t.Fatalf("approved path not allowed: %q", got.Behavior)
	}
	if got := pm.Check(scope, "write_file", map[string]any{"path": "internal/b.txt"}); got.Behavior != BehaviorAsk {
		t.Fatalf("rule widened to another path: %q", got.Behavior)
	}
}

func TestPathRulePatternsSupportDoubleStar(t *testing.T) {
	toolName, pathPattern, content := parseToolPattern("write_file(internal/**)")
	if toolName != "write_file" || pathPattern != "internal/**" || content != "" {
		t.Fatalf("unexpected parse: %q %q %q", toolName, pathPattern, content)
	}
	cases := []struct {
		pattern, value string
		want           bool
	}{
		{"internal/**", "internal/a/b.go", true},
		{"internal/**", "internal/a.go", true},
		{"internal/**", "cmd/a.go", false},
		{"internal/*.go", "internal/a.go", true},
		{"internal/*.go", "internal/sub/a.go", false},
		{"**/*.go", "a.go", true},
		{"**/*.go", "pkg/a.go", true},
		{"**/*.go", "pkg/a.txt", false},
	}
	for _, tc := range cases {
		if got := matchPath(tc.pattern, tc.value); got != tc.want {
			t.Errorf("matchPath(%q, %q) = %v, want %v", tc.pattern, tc.value, got, tc.want)
		}
	}
}

func TestPersistAlwaysAllowCreatesDedupesAndStaysValidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "permission.json")

	if err := PersistAlwaysAllow(path, Rule{Tool: "bash", Content: "git status", Behavior: BehaviorAllow}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := PersistAlwaysAllow(path, Rule{Tool: "bash", Content: "git status", Behavior: BehaviorAllow}); err != nil {
		t.Fatalf("dedupe: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var file permissionFile
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatalf("invalid JSON after persist: %v\n%s", err, data)
	}
	if len(file.Permissions.Allow) != 1 || file.Permissions.Allow[0] != "Bash(git status)" {
		t.Fatalf("unexpected allow rules: %#v", file.Permissions.Allow)
	}

	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".permission-*.tmp"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary files left behind: %v (%v)", matches, err)
	}
}

func TestPersistAlwaysAllowConcurrentWritesKeepEveryRule(t *testing.T) {
	path := filepath.Join(t.TempDir(), "permission.json")
	const writers = 16
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rule := Rule{Tool: "bash", Content: "echo writer-" + string(rune('a'+i)), Behavior: BehaviorAllow}
			if err := PersistAlwaysAllow(path, rule); err != nil {
				t.Errorf("writer %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var file permissionFile
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatalf("concurrent writes corrupted JSON: %v\n%s", err, data)
	}
	if len(file.Permissions.Allow) != writers {
		t.Fatalf("lost rules: got %d, want %d: %v", len(file.Permissions.Allow), writers, file.Permissions.Allow)
	}
	seen := make(map[string]bool, writers)
	for _, rule := range file.Permissions.Allow {
		if !strings.HasPrefix(rule, "Bash(echo writer-") {
			t.Fatalf("unexpected rule: %q", rule)
		}
		seen[rule] = true
	}
	if len(seen) != writers {
		t.Fatalf("duplicate rules written: %v", file.Permissions.Allow)
	}
}

func TestPersistAlwaysAllowKeepsExistingRulesAndFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "permission.json")
	if err := os.WriteFile(path, []byte(`{"permissions":{"allow":["read_file"],"ask":["Bash(git reset *)"],"deny":["Bash(sudo *)"]}}`), 0600); err != nil {
		t.Fatal(err)
	}

	if err := PersistAlwaysAllow(path, Rule{Tool: "write_file", Path: "internal/a.txt", Behavior: BehaviorAllow}); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	var file permissionFile
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Permissions.Ask) != 1 || len(file.Permissions.Deny) != 1 || len(file.Permissions.Allow) != 2 {
		t.Fatalf("existing rules were dropped: %#v", file.Permissions)
	}
	if file.Permissions.Allow[1] != "write_file(internal/a.txt)" {
		t.Fatalf("path rule serialized incorrectly: %q", file.Permissions.Allow[1])
	}
}
