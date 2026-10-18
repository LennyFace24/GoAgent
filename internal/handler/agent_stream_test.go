package handler

import (
	"bufio"
	"context"
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
	"github.com/LennyFace24/MiniAgent/internal/permission"
	"github.com/LennyFace24/MiniAgent/internal/service"
	"github.com/LennyFace24/MiniAgent/internal/store"
	"github.com/LennyFace24/MiniAgent/internal/tools"
	"github.com/cloudwego/eino/schema"
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
			var displayed strings.Builder
			for _, frame := range strings.Split(string(data), "\n\n") {
				if !strings.HasPrefix(frame, "event: delta\n") && !strings.HasPrefix(frame, "event: message\n") {
					continue
				}
				_, payload, _ := strings.Cut(frame, "\ndata: ")
				var message struct{ Content string }
				if err := json.Unmarshal([]byte(payload), &message); err != nil {
					t.Error(err)
					return
				}
				displayed.WriteString(message.Content)
			}
			if displayed.String() != "complete-reply-marker" {
				t.Errorf("displayed reply duplicated or truncated: %q", displayed.String())
			}
			history, err := conversations.LoadHistory(session.ID, conversation)
			if err != nil || len(history) != 2 || history[1].Content != "complete-reply-marker" {
				t.Errorf("reply not persisted: history=%v error=%v", history, err)
			}
		}()
	}
	wg.Wait()
}

type cancelOnHeaderWriter struct {
	gin.ResponseWriter
	once   sync.Once
	before func()
}

func (w *cancelOnHeaderWriter) Header() http.Header {
	w.once.Do(w.before)
	return w.ResponseWriter.Header()
}

func TestStoppedReplyIsSavedAndIncludedInNextRequest(t *testing.T) {
	for _, beforeEvents := range []bool{true, false} {
		name := "after_visible_delta"
		if beforeEvents {
			name = "before_event_consumption"
		}
		t.Run(name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			if _, err := config.LoadConfig("../../config.yaml"); err != nil {
				t.Fatal(err)
			}
			firstChunk := make(chan struct{})
			nextContext := make(chan []*schema.Message, 1)
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct{ Messages []*schema.Message }
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				if request.Messages[len(request.Messages)-1].Content == "continue" {
					nextContext <- request.Messages
					fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"continued\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
					return
				}
				if beforeEvents {
					if request.Messages[len(request.Messages)-1].Role == schema.Tool {
						// Reaching the next model turn proves earlier deltas were enqueued.
						close(firstChunk)
						<-r.Context().Done()
						return
					}
					fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"partial-reply\"}}]}\n\ndata: {\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"denied-call\",\"type\":\"function\",\"function\":{\"name\":\"context_status\",\"arguments\":\"{}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n")
					return
				}
				fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"partial-\"}}]}\n\ndata: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"reply\"}}]}\n\n")
				w.(http.Flusher).Flush()
				<-r.Context().Done()
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
			agent.PermissionManager().AddRule("", permission.Rule{Tool: "context_status", Behavior: permission.BehaviorDeny})
			handler := NewAgentHandler(agent)
			router := gin.New()
			router.Use(sessions.Sessions("goagent_session", cookie.NewStore([]byte("isolated-stopped-reply-test"))), middleware.SessionIDMiddleware())
			completed := make(chan struct{}, 2)
			router.POST("/api/chat_stream", setAgentMode("chat"), func(c *gin.Context) {
				defer func() { completed <- struct{}{} }()
				if c.Query("cancel_before_events") == "true" {
					ctx, cancel := context.WithCancel(c.Request.Context())
					defer cancel()
					c.Request = c.Request.WithContext(ctx)
					// Interrupt after generation but before the Handler starts its consumers.
					c.Writer = &cancelOnHeaderWriter{ResponseWriter: c.Writer, before: func() {
						select {
						case <-firstChunk:
						case <-ctx.Done():
						}
						cancel()
					}}
				}
				handler.Stream(c)
			})
			router.GET("/api/session", func(c *gin.Context) {
				c.JSON(200, gin.H{"id": sessions.Default(c).Get("session_id")})
			})
			server := httptest.NewServer(router)
			defer server.Close()
			jar, err := cookiejar.New(nil)
			if err != nil {
				t.Fatal(err)
			}
			client := &http.Client{Jar: jar, Timeout: 5 * time.Second}
			resp, err := client.Get(server.URL + "/api/session")
			if err != nil {
				t.Fatal(err)
			}
			var session struct{ ID string }
			err = json.NewDecoder(resp.Body).Decode(&session)
			resp.Body.Close()
			if err != nil {
				t.Fatal(err)
			}
			previous := []*schema.Message{schema.UserMessage("previous question"), schema.AssistantMessage("previous answer", nil)}
			if err := conversations.SaveMessages(session.ID, "stopped", previous...); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			url := server.URL + "/api/chat_stream"
			if beforeEvents {
				url += "?cancel_before_events=true"
			}
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(`{"message":"reply","conversation_id":"stopped"}`))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err = client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			if beforeEvents {
				_, err = io.Copy(io.Discard, resp.Body)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				scanner := bufio.NewScanner(resp.Body)
				for scanner.Scan() && scanner.Text() != `data: {"content":"reply"}` {
				}
				if scanner.Err() != nil || scanner.Text() != `data: {"content":"reply"}` {
					t.Fatalf("partial reply not delivered: %q %v", scanner.Text(), scanner.Err())
				}
			}
			cancel()
			resp.Body.Close()
			select {
			case <-completed:
			case <-time.After(5 * time.Second):
				t.Fatal("interrupted Handler did not finish saving")
			}
			history, err := conversations.LoadHistory(session.ID, "stopped")
			wantCount := 4
			if beforeEvents {
				wantCount += 2
			}
			if err != nil || len(history) != wantCount {
				t.Fatalf("interrupted reply lost: history=%v error=%v", history, err)
			}
			for i, message := range append(previous, schema.UserMessage("reply")) {
				if history[i].Role != message.Role || history[i].Content != message.Content {
					t.Fatalf("history[%d] = %s %q, want %s %q", i, history[i].Role, history[i].Content, message.Role, message.Content)
				}
			}
			if beforeEvents && (len(history[3].ToolCalls) != 1 || history[3].ToolCalls[0].ID != "denied-call" || history[4].Role != schema.Tool || history[4].ToolCallID != "denied-call") {
				t.Fatalf("tool call/result pair lost during cancellation: %v", history)
			}
			if last := history[len(history)-1]; last.Role != schema.Assistant || last.Content != "partial-reply\n\n[用户在此处停止回答]" {
				t.Fatalf("partial reply = %s %q", last.Role, last.Content)
			}
			resp, err = client.Post(server.URL+"/api/chat_stream", "application/json", strings.NewReader(`{"message":"continue","conversation_id":"stopped"}`))
			if err != nil {
				t.Fatal(err)
			}
			_, err = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if err != nil {
				t.Fatal(err)
			}
			select {
			case messages := <-nextContext:
				want := append(history, schema.UserMessage("continue"))
				if len(messages) < len(want) {
					t.Fatalf("next request omitted conversation history: %v", messages)
				}
				messages = messages[len(messages)-len(want):]
				for i, message := range want {
					if messages[i].Role != message.Role || messages[i].Content != message.Content {
						t.Fatalf("model context[%d] = %s %q, want %s %q", i, messages[i].Role, messages[i].Content, message.Role, message.Content)
					}
				}
			case <-time.After(5 * time.Second):
				t.Fatal("next request did not reach the model")
			}
		})
	}
}
