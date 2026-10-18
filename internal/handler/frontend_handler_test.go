package handler

import (
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func frontendFixture(t *testing.T) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "assets", "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"index.html":             "<!doctype html><title>GoAgent frontend</title>",
		"assets/app.js":          "window.goAgentReady=true;",
		"assets/app.css":         "body{color:green}",
		"assets/nested/chunk.js": "export const value=7;",
		"assets/.private":        "do-not-serve-hidden-file",
		"config.yaml":            "do-not-serve-root-config",
	} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	router := gin.New()
	if err := SetupFrontend(router, root); err != nil {
		t.Fatal(err)
	}
	return router, root
}

func frontendRequest(router http.Handler, method, path string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(method, path, nil))
	return response
}

func TestFrontendServesOnlyPublicFiles(t *testing.T) {
	router, _ := frontendFixture(t)
	for _, path := range []string{"/", "/index.html"} {
		response := frontendRequest(router, http.MethodGet, path)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "GoAgent frontend") || !strings.HasPrefix(response.Header().Get("Content-Type"), "text/html") {
			t.Fatalf("GET %s = %d %s %q", path, response.Code, response.Header().Get("Content-Type"), response.Body.String())
		}
	}
	for path, want := range map[string]string{
		"/assets/app.js":          "window.goAgentReady=true;",
		"/assets/app.css":         "body{color:green}",
		"/assets/nested/chunk.js": "export const value=7;",
	} {
		response := frontendRequest(router, http.MethodGet, path)
		if response.Code != http.StatusOK || response.Body.String() != want {
			t.Fatalf("GET %s = %d %q", path, response.Code, response.Body.String())
		}
		if response.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("GET %s lacks MIME sniffing protection", path)
		}
	}
	head := frontendRequest(router, http.MethodHead, "/assets/app.js")
	if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("Content-Length") == "" {
		t.Fatalf("HEAD asset = %d, body=%q, headers=%v", head.Code, head.Body.String(), head.Header())
	}
}

func TestFrontendDoesNotShadowAPIOrFallbackForMissingResources(t *testing.T) {
	router, _ := frontendFixture(t)
	// Also exercise registering API routes after frontend routes: no catch-all
	// static route may claim their namespace.
	router.GET("/api/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	router.POST("/api/chat_stream", func(c *gin.Context) { c.String(200, "chat API") })
	router.GET("/context", func(c *gin.Context) { c.String(200, "context API") })
	for _, tc := range []struct{ method, path, want string }{
		{http.MethodGet, "/api/health", `{"status":"ok"}`},
		{http.MethodPost, "/api/chat_stream", "chat API"},
		{http.MethodGet, "/context", "context API"},
	} {
		response := frontendRequest(router, tc.method, tc.path)
		if response.Code != http.StatusOK || response.Body.String() != tc.want {
			t.Fatalf("%s %s changed by frontend hosting: %d %q", tc.method, tc.path, response.Code, response.Body.String())
		}
	}
	for _, path := range []string{"/api/missing", "/assets/missing.js", "/assets/", "/assets/nested/", "/unknown-page", "/config.yaml", "/model.json", "/src/main.ts"} {
		response := frontendRequest(router, http.MethodGet, path)
		if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "GoAgent frontend") {
			t.Fatalf("GET %s must not return a page or directory listing: %d %q", path, response.Code, response.Body.String())
		}
	}
	if response := frontendRequest(router, http.MethodPost, "/"); response.Code != http.StatusNotFound {
		t.Fatalf("POST / served frontend: %d", response.Code)
	}
}

func TestFrontendRejectsTraversalAndHiddenAssets(t *testing.T) {
	router, root := frontendFixture(t)
	secret := filepath.Join(filepath.Dir(root), "secret-"+filepath.Base(root)+".txt")
	if err := os.WriteFile(secret, []byte("outside-secret"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(secret) })
	for _, path := range []string{
		"/assets/../config.yaml", "/assets/%2e%2e/config.yaml",
		"/assets/nested/../../config.yaml", "/assets/%2e%2e%5cconfig.yaml",
		"/assets/C:%5cWindows%5cwin.ini", "/assets/app.js:private", "/assets/.private",
	} {
		response := frontendRequest(router, http.MethodGet, path)
		if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "do-not-serve") {
			t.Fatalf("unsafe GET %s = %d %q", path, response.Code, response.Body.String())
		}
	}
	if err := os.Symlink(secret, filepath.Join(root, "assets", "external.txt")); err != nil {
		t.Logf("symlink escape check unavailable on this host: %v", err)
		return
	}
	response := frontendRequest(router, http.MethodGet, "/assets/external.txt")
	if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "outside-secret") {
		t.Fatalf("symlink escaped frontend root: %d %q", response.Code, response.Body.String())
	}
}

func TestFrontendMissingBuildDoesNotRegisterRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	err := SetupFrontend(router, filepath.Join(t.TempDir(), "dist"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing build should preserve filesystem error: %v", err)
	}
	router.GET("/api/health", func(c *gin.Context) { c.String(200, "API only") })
	response := frontendRequest(router, http.MethodGet, "/api/health")
	if response.Code != http.StatusOK || response.Body.String() != "API only" {
		t.Fatalf("API-only mode broken: %d %q", response.Code, response.Body.String())
	}
	if response := frontendRequest(router, http.MethodGet, "/"); response.Code != http.StatusNotFound {
		t.Fatalf("missing build registered a partial frontend: %d", response.Code)
	}
}
