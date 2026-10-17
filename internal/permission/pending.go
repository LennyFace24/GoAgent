package permission

import "sync"

type pendingDecision struct {
	sessionID      string
	conversationID string
	toolName       string
	ch             chan bool
}

var pendingRequests sync.Map

func WaitForDecision(requestID, sessionID, conversationID, toolName string) <-chan bool {
	ch := make(chan bool, 1)
	pendingRequests.Store(requestID, pendingDecision{
		sessionID:      sessionID,
		conversationID: conversationID,
		toolName:       toolName,
		ch:             ch,
	})
	return ch
}

func HandleResponse(requestID, sessionID, conversationID string, approved bool) (bool, string) {
	v, ok := pendingRequests.LoadAndDelete(requestID)
	if !ok {
		return false, ""
	}

	decision := v.(pendingDecision)
	if decision.sessionID != sessionID || decision.conversationID != conversationID {
		pendingRequests.Store(requestID, decision)
		return false, ""
	}

	decision.ch <- approved
	return true, decision.toolName
}

func CancelDecision(requestID string) {
	pendingRequests.Delete(requestID)
}
