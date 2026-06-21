package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newCORSEngine() *gin.Engine {
	r := gin.New()
	r.Use(CORS())
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })
	return r
}

// 本机回环来源（任意端口）应被精确放行并允许携带凭证。
func TestCORS_LoopbackOriginAllowed(t *testing.T) {
	r := newCORSEngine()
	origins := []string{
		"http://localhost:5173",
		"http://127.0.0.1:3000",
		"http://localhost",
	}
	for _, origin := range origins {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.Header.Set("Origin", origin)
		r.ServeHTTP(w, req)

		if got := w.Header().Get("Access-Control-Allow-Origin"); got != origin {
			t.Errorf("origin %q: ACAO = %q，期望精确回写", origin, got)
		}
		if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
			t.Errorf("origin %q: ACAC = %q，期望 true", origin, got)
		}
	}
}

// 外部来源不得回写任何 CORS 头，避免反射放行。
func TestCORS_ExternalOriginRejected(t *testing.T) {
	r := newCORSEngine()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("Origin", "https://evil.com")
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("外部源不应回写 ACAO，实际 = %q", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("外部源不应允许凭证，实际 = %q", got)
	}
}

// 外部来源的预检请求返回 204，但不携带放行头，预检即失败。
func TestCORS_PreflightExternalOriginNotAllowed(t *testing.T) {
	r := newCORSEngine()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/ping", nil)
	req.Header.Set("Origin", "https://evil.com")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("OPTIONS 状态码 = %d，期望 204", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("外部源预检不应放行，ACAO = %q", got)
	}
}
