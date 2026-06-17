package permission

import "sync"

type pendingDecision struct {
	sessionID      string
	conversationID string
	ch             chan bool
}

var pendingRequests sync.Map

func WaitForDecision(requestID, sessionID, conversationID string) <-chan bool {
	ch := make(chan bool, 1)
	pendingRequests.Store(requestID, pendingDecision{
		sessionID:      sessionID,
		conversationID: conversationID,
		ch:             ch,
	})
	return ch
}

func HandleResponse(requestID, sessionID, conversationID string, approved bool) bool {
	v, ok := pendingRequests.LoadAndDelete(requestID)
	if !ok {
		return false
	}

	decision := v.(pendingDecision)
	if decision.sessionID != sessionID || decision.conversationID != conversationID {
		pendingRequests.Store(requestID, decision)
		return false
	}

	decision.ch <- approved
	return true
}

func CancelDecision(requestID string) {
	pendingRequests.Delete(requestID)
}
