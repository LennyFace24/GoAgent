package permission

import "testing"

func TestCheckAskRuleTakesPrecedenceOverAllowRule(t *testing.T) {
	pm := NewPermissionManager(ModeDefault)
	pm.rules = []Rule{
		{Tool: "bash", Content: "npm install *", Behavior: BehaviorAsk},
		{Tool: "bash", Content: "npm *", Behavior: BehaviorAllow},
	}

	decision := pm.Check("bash", map[string]any{"command": "npm install left-pad"})

	if decision.Behavior != BehaviorAsk {
		t.Fatalf("expected ask decision, got %q (%s)", decision.Behavior, decision.Reason)
	}
}

func TestCheckReadOnlyBashIsAllowedBeforeAllowRules(t *testing.T) {
	pm := NewPermissionManager(ModeDefault)
	pm.rules = nil

	decision := pm.Check("bash", map[string]any{"command": "pwd"})

	if decision.Behavior != BehaviorAllow {
		t.Fatalf("expected readonly bash to be allowed, got %q (%s)", decision.Behavior, decision.Reason)
	}
}

func TestCheckDenyRuleTakesPrecedenceOverReadOnlyAnalysis(t *testing.T) {
	pm := NewPermissionManager(ModeDefault)
	pm.rules = []Rule{
		{Tool: "bash", Content: "pwd", Behavior: BehaviorDeny},
	}

	decision := pm.Check("bash", map[string]any{"command": "pwd"})

	if decision.Behavior != BehaviorDeny {
		t.Fatalf("expected deny decision, got %q (%s)", decision.Behavior, decision.Reason)
	}
}
