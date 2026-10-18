package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LennyFace24/MiniAgent/internal/modelmgr"
	"github.com/LennyFace24/MiniAgent/internal/permission"
	"github.com/LennyFace24/MiniAgent/internal/pipeline"
	"github.com/LennyFace24/MiniAgent/internal/skills"
	"github.com/LennyFace24/MiniAgent/internal/store"
	contexttool "github.com/LennyFace24/MiniAgent/internal/tools/context_tool"
	extratool "github.com/LennyFace24/MiniAgent/internal/tools/extra_tool"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

type lifecycleRequest struct {
	Model    string `json:"model"`
	Stream   bool   `json:"stream"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	Tools      []any  `json:"tools"`
	ToolChoice string `json:"tool_choice"`
}

func writeLifecycleFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func lifecycleService(t *testing.T, endpoint string, registered []tool.BaseTool, window int) *AgentService {
	t.Helper()
	t.Chdir(t.TempDir())
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	writeLifecycleFile(t, "internal/prompt/system_prompt.md", "lifecycle-system-marker")
	writeLifecycleFile(t, "internal/tools/context_tool/compact_rule.md", "Summarize old messages. Return only the summary.")
	writeLifecycleFile(t, "permission.json", `{"permissions":{"allow":["inspect"],"ask":[],"deny":[]}}`)
	models := []modelmgr.Model{
		{ID: "first", BaseURL: endpoint + "/v1", APIKey: "fixture", ContextWindow: window, MaxCompletionTokens: 128, SafetyMarginTokens: 64},
		{ID: "second", BaseURL: endpoint + "/v1", APIKey: "fixture", ContextWindow: window * 2, MaxCompletionTokens: 128, SafetyMarginTokens: 64},
	}
	data, err := json.Marshal(map[string]any{"default": "first", "models": models})
	if err != nil {
		t.Fatal(err)
	}
	writeLifecycleFile(t, "model.json", string(data))
	registry, err := modelmgr.Load("model.json")
	if err != nil {
		t.Fatal(err)
	}
	conversations, err := store.NewConversationStore("data/conversations")
	if err != nil {
		t.Fatal(err)
	}
	return &AgentService{
		store: conversations, models: modelmgr.NewModelSet(registry, registered), tools: registered,
		compactTrigger: &contexttool.CompactTrigger{}, perms: permission.NewPermissionManager(permission.ModeDefault),
		skillReg: skills.NewSkillRegistry(),
	}
}

func respondLifecycleStream(w http.ResponseWriter, delta map[string]any) {
	w.Header().Set("Content-Type", "text/event-stream")
	data, _ := json.Marshal(map[string]any{
		"id": "lifecycle", "object": "chat.completion.chunk", "created": 1,
		"choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": "stop"}},
	})
	_, _ = fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", data)
}

func drainLifecycle(t *testing.T, iterator *adk.AsyncIterator[*adk.AgentEvent], events <-chan ToolEvent) []ToolEvent {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		for event, ok := iterator.Next(); ok; event, ok = iterator.Next() {
			if event.Err != nil {
				done <- event.Err
				return
			}
		}
		done <- nil
	}()
	var collected []ToolEvent
	for event := range events {
		collected = append(collected, event)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	return collected
}

func TestRequestKeepsPreparedModelAfterSwitch(t *testing.T) {
	var mu sync.Mutex
	var modelIDs []string
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request lifecycleRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			http.Error(w, err.Error(), 400)
			return
		}
		mu.Lock()
		modelIDs = append(modelIDs, request.Model)
		mu.Unlock()
		if calls.Add(1) == 1 {
			respondLifecycleStream(w, map[string]any{"tool_calls": []any{map[string]any{
				"index": 0, "id": "inspect-call", "type": "function",
				"function": map[string]any{"name": "inspect", "arguments": "{}"},
			}}})
			return
		}
		respondLifecycleStream(w, map[string]any{"content": "answer from " + request.Model})
	}))
	defer provider.Close()
	var svc *AgentService
	inspect, err := utils.InferTool("inspect", "Inspect local state", func(ctx context.Context, _ struct{}) (string, error) {
		if err := svc.SwitchModel(ctx, "second"); err != nil {
			return "", err
		}
		return "inspection completed", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	svc = lifecycleService(t, provider.URL, []tool.BaseTool{inspect}, 20000)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	iterator, events, err := svc.Stream(ctx, "chat", "session", "conversation", "Inspect and answer")
	if err != nil {
		t.Fatal(err)
	}
	drainLifecycle(t, iterator, events)
	iterator, events, err = svc.Stream(ctx, "chat", "session", "next-conversation", "Answer directly")
	if err != nil {
		t.Fatal(err)
	}
	drainLifecycle(t, iterator, events)
	mu.Lock()
	defer mu.Unlock()
	if want := []string{"first", "first", "second"}; !reflect.DeepEqual(modelIDs, want) {
		t.Fatalf("model changed inside an in-flight request: got %v, want %v", modelIDs, want)
	}
	if got := contexttool.GetContextState().MaxTokens(); got != 40000-128-64 {
		t.Fatalf("new request retained the previous model budget: %d", got)
	}
}

func TestStreamSummarizesOldHistoryBeforeSendingConversation(t *testing.T) {
	var summaries atomic.Int32
	var streams atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request lifecycleRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			http.Error(w, err.Error(), 400)
			return
		}
		if !request.Stream {
			summaries.Add(1)
			if len(request.Tools) != 0 {
				t.Error("summary request unexpectedly bound execution tools")
			}
			if !strings.Contains(request.Messages[len(request.Messages)-1].Content, "old-marker-0") {
				t.Error("compaction lost the first historical user message")
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"id":"summary","object":"chat.completion","created":1,"choices":[{"index":0,"message":{"role":"assistant","content":"condensed-history-marker"},"finish_reason":"stop"}]}`)
			return
		}
		streams.Add(1)
		joined := ""
		for _, message := range request.Messages {
			joined += message.Content
		}
		if !strings.Contains(joined, "condensed-history-marker") || !strings.Contains(joined, "lifecycle-system-marker") || strings.Contains(joined, "old-marker-0") {
			t.Error("conversation request did not preserve system prompt and condensed history")
		}
		if request.Messages[len(request.Messages)-1].Content != "current-user-marker" {
			t.Error("compaction lost the current user message")
		}
		respondLifecycleStream(w, map[string]any{"content": "summarized answer"})
	}))
	defer provider.Close()
	statusTool, err := contexttool.NewContextStatusTool()
	if err != nil {
		t.Fatal(err)
	}
	svc := lifecycleService(t, provider.URL, []tool.BaseTool{statusTool}, 4000)
	var history []*schema.Message
	for i := range 10 {
		content := fmt.Sprintf("old-marker-%d", i)
		if i < 4 {
			content += " " + strings.Repeat("history ", 1500)
		}
		history = append(history, schema.UserMessage(content), schema.AssistantMessage("previous reply", nil))
	}
	if err := svc.store.SaveMessages("session", "conversation", history...); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	iterator, events, err := svc.Stream(ctx, "chat", "session", "conversation", "current-user-marker")
	if err != nil {
		t.Fatal(err)
	}
	drainLifecycle(t, iterator, events)
	if summaries.Load() != 1 || streams.Load() != 1 {
		t.Fatalf("precheck/loop repeated compaction: summaries=%d streams=%d", summaries.Load(), streams.Load())
	}
}

func TestToolRoundLimitForcesFinalStreamingAnswer(t *testing.T) {
	var executions atomic.Int32
	var requests atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request lifecycleRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			http.Error(w, err.Error(), 400)
			return
		}
		requests.Add(1)
		if request.ToolChoice == "none" {
			respondLifecycleStream(w, map[string]any{"content": "final answer after tool limit"})
			return
		}
		respondLifecycleStream(w, map[string]any{"tool_calls": []any{map[string]any{
			"index": 0, "id": fmt.Sprintf("inspect-%d", requests.Load()), "type": "function",
			"function": map[string]any{"name": "inspect", "arguments": "{}"},
		}}})
	}))
	defer provider.Close()
	inspect, err := utils.InferTool("inspect", "Inspect", func(context.Context, struct{}) (string, error) {
		executions.Add(1)
		return "inspection result", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	svc := lifecycleService(t, provider.URL, []tool.BaseTool{inspect}, 20000)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	iterator, events, err := svc.Stream(ctx, "chat", "session", "conversation", "Inspect until finished")
	if err != nil {
		t.Fatal(err)
	}
	collected := drainLifecycle(t, iterator, events)
	var reply strings.Builder
	for _, event := range collected {
		if event.Type == "delta" {
			reply.WriteString(event.Content)
		}
		if event.Type == "tool_result" && strings.Contains(event.Result, "待办事项") {
			t.Error("request without todo tools advertised nonexistent todo capabilities")
		}
	}
	if executions.Load() != 20 || requests.Load() != 21 || reply.String() != "final answer after tool limit" {
		t.Fatalf("tool limit failed: executions=%d requests=%d reply=%q", executions.Load(), requests.Load(), reply.String())
	}
}

func TestRepeatedDenialsDoNotAdvertisePlanMode(t *testing.T) {
	var requests atomic.Int32
	var executions atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) <= 3 {
			respondLifecycleStream(w, map[string]any{"tool_calls": []any{map[string]any{
				"index": 0, "id": fmt.Sprintf("denied-%d", requests.Load()), "type": "function",
				"function": map[string]any{"name": "inspect", "arguments": "{}"},
			}}})
			return
		}
		respondLifecycleStream(w, map[string]any{"content": "answer without execution"})
	}))
	defer provider.Close()
	inspect, err := utils.InferTool("inspect", "Inspect", func(context.Context, struct{}) (string, error) {
		executions.Add(1)
		return "must not execute", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	svc := lifecycleService(t, provider.URL, []tool.BaseTool{inspect}, 20000)
	svc.perms.AddRule(permission.Rule{Tool: "inspect", Behavior: permission.BehaviorDeny})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	iterator, events, err := svc.Stream(ctx, "chat", "session", "conversation", "Inspect")
	if err != nil {
		t.Fatal(err)
	}
	collected := drainLifecycle(t, iterator, events)
	var results []string
	for _, event := range collected {
		if event.Type == "tool_result" {
			results = append(results, event.Result)
		}
	}
	if executions.Load() != 0 || len(results) != 3 || !strings.Contains(results[2], "确认允许执行的范围") || strings.Contains(results[2], "plan 模式") {
		t.Fatalf("denial policy changed: executions=%d results=%v", executions.Load(), results)
	}
}

func TestTodoReminderUsesRegisteredFileTools(t *testing.T) {
	var requests atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) <= 5 {
			respondLifecycleStream(w, map[string]any{"tool_calls": []any{map[string]any{
				"index": 0, "id": fmt.Sprintf("inspect-%d", requests.Load()), "type": "function",
				"function": map[string]any{"name": "inspect", "arguments": "{}"},
			}}})
			return
		}
		respondLifecycleStream(w, map[string]any{"content": "finished"})
	}))
	defer provider.Close()
	inspect, err := utils.InferTool("inspect", "Inspect", func(context.Context, struct{}) (string, error) { return "inspection result", nil })
	if err != nil {
		t.Fatal(err)
	}
	reader, err := extratool.NewTodoReader()
	if err != nil {
		t.Fatal(err)
	}
	writer, err := extratool.NewTodoWriter()
	if err != nil {
		t.Fatal(err)
	}
	svc := lifecycleService(t, provider.URL, []tool.BaseTool{inspect, reader, writer}, 20000)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	iterator, events, err := svc.Stream(ctx, "chat", "session", "conversation", "Inspect")
	if err != nil {
		t.Fatal(err)
	}
	collected := drainLifecycle(t, iterator, events)
	reminders := 0
	for _, event := range collected {
		if event.Type == "tool_result" && strings.Contains(event.Result, "read_todo 和 write_todo") {
			reminders++
		}
	}
	if reminders != 1 {
		t.Fatalf("five non-todo rounds should trigger one reminder, got %d", reminders)
	}
	if _, err := writer.InvokableRun(ctx, `{"path":"todo.md","content":"# Task\n- pending: inspect"}`); err != nil {
		t.Fatal(err)
	}
	text, err := reader.InvokableRun(ctx, `{"path":"todo.md"}`)
	if err != nil || !strings.Contains(text, "pending: inspect") {
		t.Fatalf("registered todo tools did not persist/reload task: %q %v", text, err)
	}
}

func TestFullCompactionRunsInsidePipeline(t *testing.T) {
	var summaries atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request lifecycleRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		if request.Stream {
			t.Error("pipeline started the dialogue loop during preparation")
		}
		if !strings.Contains(request.Messages[len(request.Messages)-1].Content, "precheck-oldest") {
			t.Error("precheck failed to summarize the oldest user message")
		}
		summaries.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id":"summary","choices":[{"index":0,"message":{"role":"assistant","content":"precheck-summary"},"finish_reason":"stop"}]}`)
	}))
	defer provider.Close()
	svc := lifecycleService(t, provider.URL, nil, 4000)
	history := []*schema.Message{schema.UserMessage("precheck-oldest " + strings.Repeat("history ", 5000)), schema.AssistantMessage("old reply", nil)}
	for i := range 8 {
		history = append(history, schema.UserMessage(fmt.Sprintf("recent-%d", i)), schema.AssistantMessage("reply", nil))
	}
	mc := &pipeline.MessageContext{Raw: "current", Mode: "chat", SessionID: "session", ConversationID: "conversation", History: history}
	pl := pipeline.New(
		pipeline.NewModelPrepStage(svc.models.Snapshot),
		pipeline.NewContextBuildStage(nil),
		pipeline.NewCompactCheckStage(contexttool.CompactFunc, nil),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := pl.Run(ctx, mc); err != nil {
		t.Fatal(err)
	}
	if summaries.Load() != 1 || mc.Messages[0].Role != schema.System || mc.Messages[1].Content != "precheck-summary" || mc.Messages[len(mc.Messages)-1].Content != "current" {
		t.Fatalf("full compaction was not completed by Pipeline: summaries=%d messages=%v", summaries.Load(), mc.Messages)
	}
}
