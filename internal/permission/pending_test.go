package permission

import "testing"

func TestHandleResponseRequiresMatchingSession(t *testing.T) {
	ch := WaitForDecision("req-1", "session-a", "conversation-a", "bash")

	if handled, _ := HandleResponse("req-1", "session-b", "conversation-a", true); handled {
		t.Fatalf("expected mismatched session to be rejected")
	}

	select {
	case <-ch:
		t.Fatalf("mismatched response should not unblock the waiter")
	default:
	}

	if handled, toolName := HandleResponse("req-1", "session-a", "conversation-a", true); !handled || toolName != "bash" {
		t.Fatalf("expected matching response to be accepted")
	}

	select {
	case approved := <-ch:
		if !approved {
			t.Fatalf("expected approval to be delivered")
		}
	default:
		t.Fatalf("expected waiter to receive approval")
	}
}

