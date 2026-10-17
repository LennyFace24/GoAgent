package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	mcpclient "github.com/LennyFace24/MiniAgent/internal/mcp"
	"github.com/LennyFace24/MiniAgent/internal/modelmgr"
	"github.com/LennyFace24/MiniAgent/internal/permission"
	contexttool "github.com/LennyFace24/MiniAgent/internal/tools/context_tool"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPApprovalPreservesProtocolFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var calls atomic.Int32
	remote := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "approval-regression", Version: "1"}, nil)
	remote.AddTool(&sdkmcp.Tool{Name: "read_record", InputSchema: json.RawMessage(`{"type":"object"}`)}, func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		calls.Add(1)
		return nil, fmt.Errorf("record access rejected by MCP protocol")
	})
	remoteHTTP := httptest.NewServer(sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server { return remote }, nil))
	defer remoteHTTP.Close()
	manager := mcpclient.NewMCPManager()
	defer manager.Close()
	manager.SetConfig(&mcpclient.MCPConfig{Servers: map[string]*mcpclient.MCPServerConfig{
		"records": {Enabled: true, Transport: mcpclient.TransportStreamableHTTP, URL: remoteHTTP.URL},
	}})
	if err := manager.ConnectAll(ctx); err != nil {
		t.Fatal(err)
	}
	remoteTools, err := manager.Tools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	info, err := remoteTools[0].Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	toolName := info.Name

	for _, approved := range []bool{false, true} {
		t.Run(fmt.Sprintf("approved=%t", approved), func(t *testing.T) {
			modelResult := make(chan string, 1)
			modelHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					Messages []struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					} `json:"messages"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					http.Error(w, err.Error(), 400)
					return
				}
				var delta map[string]any
				finishReason := "tool_calls"
				last := request.Messages[len(request.Messages)-1]
				if last.Role == "tool" {
					modelResult <- last.Content
					delta = map[string]any{"content": "MCP result received"}
					finishReason = "stop"
				} else {
					delta = map[string]any{"tool_calls": []any{map[string]any{
						"index": 0, "id": "approval-call", "type": "function",
						"function": map[string]any{"name": toolName, "arguments": "{}"},
					}}}
				}
				w.Header().Set("Content-Type", "text/event-stream")
				chunk := map[string]any{"id": "approval-model", "object": "chat.completion.chunk", "created": 1, "model": "approval", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finishReason}}}
				data, _ := json.Marshal(chunk)
				_, _ = fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", data)
			}))
			defer modelHTTP.Close()
			modelConfig := map[string]any{"default": "approval", "models": []modelmgr.Model{{ID: "approval", BaseURL: modelHTTP.URL + "/v1", APIKey: "fixture-only", ContextWindow: 200000, MaxCompletionTokens: 1024}}}
			data, err := json.Marshal(modelConfig)
			if err != nil {
				t.Fatal(err)
			}
			modelPath := filepath.Join(t.TempDir(), "model.json")
			if err := os.WriteFile(modelPath, data, 0600); err != nil {
				t.Fatal(err)
			}
			registry, err := modelmgr.Load(modelPath)
			if err != nil {
				t.Fatal(err)
			}
			perms := permission.NewPermissionManager(permission.ModeDefault)
			perms.AddRule(permission.Rule{Tool: toolName, Behavior: permission.BehaviorAsk})
			svc := &AgentService{models: modelmgr.NewModelSet(registry, remoteTools), tools: remoteTools, perms: perms, compactTrigger: &contexttool.CompactTrigger{}}
			iterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
			agentDone := make(chan struct{})
			go func() {
				defer close(agentDone)
				for event, ok := iterator.Next(); ok; event, ok = iterator.Next() {
					if event.Err != nil {
						t.Errorf("agent error: %v", event.Err)
					}
				}
			}()
			toolEvents := make(chan ToolEvent)
			go svc.runLoop(ctx, "chat", []*schema.Message{schema.UserMessage("Read the record")}, generator, toolEvents, "approval-session", "approval-conversation", contexttool.NewContextState(200000), contexttool.NewContextState(200000))
			result := ""
			permissionSeen := false
		loop:
			for {
				select {
				case event, ok := <-toolEvents:
					if !ok {
						break loop
					}
					if event.Type == "permission_request" {
						permissionSeen = true
						if calls.Load() != 0 {
							t.Error("MCP call ran before approval")
						}
						if handled, _ := permission.HandleResponse(event.RequestID, "approval-session", "approval-conversation", approved); !handled {
							t.Error("permission request was published before its waiter was registered")
						}
					}
					if event.Type == "tool_result" {
						result = event.Result
					}
				case <-ctx.Done():
					t.Fatal("agent did not finish:", ctx.Err())
				}
			}
			<-agentDone
			if !permissionSeen {
				t.Fatal("MCP tool bypassed permission approval")
			}
			if approved {
				if calls.Load() != 1 || !strings.Contains(result, "record access rejected by MCP protocol") {
					t.Fatalf("approved MCP failure lost: calls=%d result=%q", calls.Load(), result)
				}
			} else if calls.Load() != 0 || !strings.Contains(result, "Permission denied") {
				t.Fatalf("denied MCP tool executed: calls=%d result=%q", calls.Load(), result)
			}
			select {
			case received := <-modelResult:
				if received != result {
					t.Fatalf("model received %q, SSE result was %q", received, result)
				}
			default:
				t.Fatal("MCP tool result did not reach the next model turn")
			}
		})
	}
}
