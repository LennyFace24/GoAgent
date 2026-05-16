package permission

import "sync"

// pendingRequests 存储等待用户响应的权限请求
// key: requestID (uuid), value: chan bool
var pendingRequests sync.Map

// WaitForDecision 创建一个等待 channel，阻塞直到用户响应或超时
func WaitForDecision(requestID string) <-chan bool {
	ch := make(chan bool, 1)
	pendingRequests.Store(requestID, ch)
	return ch
}

// HandleResponse 由 handler 调用，将用户决策写入对应的 channel
func HandleResponse(requestID string, approved bool) {
	if v, ok := pendingRequests.LoadAndDelete(requestID); ok {
		v.(chan bool) <- approved
	}
}

// CancelDecision 取消等待（超时或上下文取消时调用）
func CancelDecision(requestID string) {
	pendingRequests.Delete(requestID)
}
