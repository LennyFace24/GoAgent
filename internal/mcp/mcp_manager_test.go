package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/tool"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPHTTPTransportsSurviveStartupAndDiscoverPages(t *testing.T) {
	for _, transport := range []MCPTransport{TransportSSE, TransportStreamableHTTP} {
		t.Run(string(transport), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			t.Setenv("MCP_TEST_HEADER", "local-secret")
			server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "transport-regression", Version: "1"}, &sdkmcp.ServerOptions{PageSize: 1})
			for _, name := range []string{"first", "second"} {
				server.AddTool(&sdkmcp.Tool{Name: name, InputSchema: json.RawMessage(`{"type":"object"}`)}, func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
					return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "authenticated call completed"}}}, nil
				})
			}
			var handler http.Handler
			getServer := func(*http.Request) *sdkmcp.Server { return server }
			if transport == TransportSSE {
				handler = sdkmcp.NewSSEHandler(getServer, nil)
			} else {
				handler = sdkmcp.NewStreamableHTTPHandler(getServer, nil)
			}
			httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer local-secret" {
					http.Error(w, "authentication header missing", http.StatusUnauthorized)
					return
				}
				handler.ServeHTTP(w, r)
			}))
			defer httpServer.Close()
			manager := NewMCPManager()
			defer manager.Close()
			manager.SetConfig(&MCPConfig{Servers: map[string]*MCPServerConfig{
				"remote": {Enabled: true, Transport: transport, URL: httpServer.URL, Headers: map[string]string{"Authorization": "Bearer ${MCP_TEST_HEADER}"}},
			}})
			if err := manager.ConnectAll(ctx); err != nil {
				t.Fatal(err)
			}
			cancel()
			callCtx, cancelCall := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancelCall()
			tools, err := manager.Tools(callCtx)
			if err != nil {
				t.Fatal("startup cancellation closed the established session:", err)
			}
			var names []string
			for _, remoteTool := range tools {
				info, err := remoteTool.Info(callCtx)
				if err != nil {
					t.Fatal(err)
				}
				names = append(names, info.Name)
			}
			if want := []string{"mcp__remote__first", "mcp__remote__second"}; !reflect.DeepEqual(names, want) {
				t.Fatalf("paginated tools = %v, want %v", names, want)
			}
			output, err := tools[1].(tool.InvokableTool).InvokableRun(callCtx, "{}")
			if err != nil || output != "authenticated call completed" {
				t.Fatalf("post-startup authenticated call = %q: %v", output, err)
			}
			if err := manager.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err := tools[0].(tool.InvokableTool).InvokableRun(callCtx, "{}"); err == nil {
				t.Fatal("closed MCP manager still allowed a call")
			}
		})
	}
}

func TestMCPConfiguredHeadersNeverFollowCrossOriginRedirect(t *testing.T) {
	var requests atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer destination.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	client := newHTTPClient(source.URL, map[string]string{"Authorization": "Bearer private"})
	response, err := client.Get(source.URL)
	if response != nil {
		response.Body.Close()
	}
	if err == nil || requests.Load() != 0 {
		t.Fatalf("configured headers crossed origin: requests=%d error=%v", requests.Load(), err)
	}
}

func TestMCPPartialConnectionFailureIsReportedAndClosed(t *testing.T) {
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "partial", Version: "1"}, nil)
	server.AddTool(&sdkmcp.Tool{Name: "read", InputSchema: json.RawMessage(`{"type":"object"}`)}, func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "read"}}}, nil
	})
	httpServer := httptest.NewServer(sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server { return server }, nil))
	defer httpServer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	manager := NewMCPManager()
	defer manager.Close()
	manager.SetConfig(&MCPConfig{Servers: map[string]*MCPServerConfig{
		"working": {Enabled: true, Transport: TransportStreamableHTTP, URL: httpServer.URL},
		"broken":  {Enabled: true, Transport: TransportStdio},
	}})
	if err := manager.ConnectAll(ctx); err == nil {
		t.Fatal("partial failure was not reported")
	}
	if _, ok := manager.GetSession("working"); !ok {
		t.Fatal("successful connection disappeared after peer failed")
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	if err := manager.ConnectAll(ctx); err == nil {
		t.Fatal("closed manager reopened sessions")
	}
	if len(manager.GetAllSessions()) != 0 {
		t.Fatal("closed manager retained sessions")
	}
}
