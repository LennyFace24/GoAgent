package middleware

import (
	"net/url"

	"github.com/gin-gonic/gin"
)

// isLoopbackOrigin 判断请求来源是否为本机回环地址（localhost / 127.0.0.1 / ::1）。
// 本地客户端定位下，仅放行本机来源即可挡住一切远程站点的跨站请求。
func isLoopbackOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

// CORS 跨域中间件：仅放行本机回环来源。
// 携带凭证（Cookie）的跨域请求要求 Allow-Origin 精确回写、Allow-Headers 显式枚举，
// 因此不再使用通配 *。非本机来源不写任何 CORS 头，由浏览器同源策略拦截。
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if isLoopbackOrigin(origin) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Accept")
			c.Header("Vary", "Origin")
		}
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
