package contexttool

import "context"

type contextKey string

const (
	// SessionIDKey / ConversationIDKey 用于在请求、命令和工具调用之间传递会话标识。
	SessionIDKey      contextKey = "session_id"
	ConversationIDKey contextKey = "conversation_id"
)

// WithScope 把会话标识写入 context。
func WithScope(ctx context.Context, sessionID, conversationID string) context.Context {
	ctx = context.WithValue(ctx, SessionIDKey, sessionID)
	return context.WithValue(ctx, ConversationIDKey, conversationID)
}

// ScopeFromContext 从 context 读取会话状态键。
func ScopeFromContext(ctx context.Context) (string, bool) {
	sessionID, _ := ctx.Value(SessionIDKey).(string)
	conversationID, _ := ctx.Value(ConversationIDKey).(string)
	if sessionID == "" || conversationID == "" {
		return "", false
	}
	return ScopeKey(sessionID, conversationID), true
}
