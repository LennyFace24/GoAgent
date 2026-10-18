package permission

import "testing"

func TestHandleResponseRequiresMatchingSession(t *testing.T) {
	ch := WaitForDecision("req-1", "session-a", "conversation-a", "bash", map[string]any{"command": "pwd"})

	if handled, _ := HandleResponse("req-1", "session-b", "conversation-a", true, false, false); handled {
		t.Fatalf("expected mismatched session to be rejected")
	}

	select {
	case <-ch:
		t.Fatalf("mismatched response should not unblock the waiter")
	default:
	}

	if handled, approval := HandleResponse("req-1", "session-a", "conversation-a", true, false, false); !handled || approval.ToolName != "bash" || approval.ToolInput["command"] != "pwd" {
		t.Fatalf("expected matching response to be accepted: %#v", approval)
	}

	select {
	case response := <-ch:
		if !response.Approved || response.Always || response.Global {
			t.Fatalf("unexpected response: %#v", response)
		}
	default:
		t.Fatalf("expected waiter to receive a response")
	}
}

func TestHandleResponseCarriesAlwaysAndGlobalOnlyWhenApproved(t *testing.T) {
	ch := WaitForDecision("req-2", "session-a", "conversation-a", "bash", map[string]any{"command": "ls"})
	if handled, _ := HandleResponse("req-2", "session-a", "conversation-a", false, true, true); !handled {
		t.Fatal("expected response to be accepted")
	}
	if response := <-ch; response.Approved || response.Always || response.Global {
		t.Fatalf("denied response must not grant always/global: %#v", response)
	}

	ch = WaitForDecision("req-3", "session-a", "conversation-a", "bash", map[string]any{"command": "ls"})
	if handled, _ := HandleResponse("req-3", "session-a", "conversation-a", true, true, true); !handled {
		t.Fatal("expected response to be accepted")
	}
	if response := <-ch; !response.Approved || !response.Always || !response.Global {
		t.Fatalf("approved always/global was not propagated: %#v", response)
	}
}
