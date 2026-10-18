package permission

import "sync"

// PendingApproval 是等待用户确认的工具调用快照。
type PendingApproval struct {
	ToolName  string
	ToolInput map[string]any
}

// Response 是用户对一个待决调用的完整答复。
type Response struct {
	Approved bool
	Always   bool
	Global   bool
}

type pendingDecision struct {
	sessionID      string
	conversationID string
	approval       PendingApproval
	ch             chan Response
}

var pendingRequests sync.Map

func WaitForDecision(requestID, sessionID, conversationID, toolName string, toolInput map[string]any) <-chan Response {
	ch := make(chan Response, 1)
	pendingRequests.Store(requestID, pendingDecision{
		sessionID:      sessionID,
		conversationID: conversationID,
		approval: PendingApproval{
			ToolName:  toolName,
			ToolInput: cloneInput(toolInput),
		},
		ch: ch,
	})
	return ch
}

// HandleResponse 校验会话绑定并唤醒等待方。Always/Global 随答复一起传递，
// 由工具循环在继续执行前安装规则，避免“先执行、后记规则”的竞态。
func HandleResponse(requestID, sessionID, conversationID string, approved, always, global bool) (bool, PendingApproval) {
	v, ok := pendingRequests.LoadAndDelete(requestID)
	if !ok {
		return false, PendingApproval{}
	}

	decision := v.(pendingDecision)
	if decision.sessionID != sessionID || decision.conversationID != conversationID {
		pendingRequests.Store(requestID, decision)
		return false, PendingApproval{}
	}

	decision.ch <- Response{Approved: approved, Always: always && approved, Global: global && always && approved}
	return true, decision.approval
}

func CancelDecision(requestID string) {
	pendingRequests.Delete(requestID)
}

func cloneInput(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	clone := make(map[string]any, len(input))
	for key, value := range input {
		clone[key] = value
	}
	return clone
}
