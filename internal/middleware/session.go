package middleware

import (
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SessionIDMiddleware 会话ID中间件
func SessionIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)

		if session.Get("session_id") == nil {
			session.Set("session_id", uuid.New().String())
			session.Save()
		}
		c.Next()
	}
}
