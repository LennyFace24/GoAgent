package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LennyFace24/MiniAgent/internal/config"
	"github.com/LennyFace24/MiniAgent/internal/middleware"
	"github.com/LennyFace24/MiniAgent/internal/modelmgr"
	"github.com/LennyFace24/MiniAgent/internal/service"
	"github.com/LennyFace24/MiniAgent/internal/store"
	"github.com/LennyFace24/MiniAgent/internal/tools"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func TestConcurrentHTTPStreamsKeepValidSSEAndPersistReplies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if _, err := config.LoadConfig("../../config.yaml"); err != nil {
		t.Fatal(err)
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "data: {\"id\":\"stream\",\"object\":\"chat.completion.chunk\",\"created\":1,\"choices\":[{\"index\":0,\"delta\":{\"content\":\"complete-reply-marker\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer provider.Close()
	modelPath := filepath.Join(t.TempDir(), "model.json")
	modelJSON := fmt.Sprintf(`{"default":"fixture","models":[{"id":"fixture","base_url":%q,"api_key":"local-only","context_window":200000,"max_completion_tokens":1024}]}`, provider.URL+"/v1")
	if err := os.WriteFile(modelPath, []byte(modelJSON), 0600); err != nil {
		t.Fatal(err)
	}
	registry, err := modelmgr.Load(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	conversations, err := store.NewConversationStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	registered, err := tools.NewToolHandler(nil, nil, registry.Default)
	if err != nil {
		t.Fatal(err)
	}
	agent := service.NewAgentService(registered, conversations, registry)
	handler := NewAgentHandler(agent)
	router := gin.New()
	cookieStore := cookie.NewStore([]byte("isolated-http-stream-test"))
	router.Use(sessions.Sessions("goagent_session", cookieStore), middleware.SessionIDMiddleware())
	router.POST("/api/chat_stream", setAgentMode("chat"), handler.Stream)
	router.GET("/api/session", func(c *gin.Context) {
		c.JSON(200, gin.H{"id": sessions.Default(c).Get("session_id")})
	})
	server := httptest.NewServer(router)
	defer server.Close()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	resp, err := client.Get(server.URL + "/api/session")
	if err != nil {
		t.Fatal(err)
	}
	var session struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	var wg sync.WaitGroup
	for i := range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conversation := fmt.Sprintf("stream-%d", i)
			body := fmt.Sprintf(`{"message":"reply","conversation_id":%q}`, conversation)
			resp, err := client.Post(server.URL+"/api/chat_stream", "application/json", strings.NewReader(body))
			if err != nil {
				t.Error("invalid HTTP stream response:", err)
				return
			}
			data, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil || resp.StatusCode != 200 || !strings.Contains(string(data), "event: delta\n") || !strings.Contains(string(data), "event: done\n") {
				t.Errorf("incomplete SSE: status=%d error=%v body=%q", resp.StatusCode, err, data)
				return
			}
			history, err := conversations.LoadHistory(session.ID, conversation)
			if err != nil || len(history) != 2 || history[1].Content != "complete-reply-marker" {
				t.Errorf("reply not persisted: history=%v error=%v", history, err)
			}
		}()
	}
	wg.Wait()
}
