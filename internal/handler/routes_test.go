package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LennyFace24/MiniAgent/internal/config"
	"github.com/LennyFace24/MiniAgent/internal/metrics"
	"github.com/LennyFace24/MiniAgent/internal/middleware"
	"github.com/LennyFace24/MiniAgent/internal/modelmgr"
	"github.com/LennyFace24/MiniAgent/internal/service"
	"github.com/LennyFace24/MiniAgent/internal/store"
	"github.com/LennyFace24/MiniAgent/internal/tools"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// newTestRouter 构建注入依赖的测试路由：
// - t.TempDir() 建 ConversationStore（不碰 data/conversations）
// - cookie store + sessions + SessionIDMiddleware + CORS（复刻 main.go 装配）
// - 真实 ModelSet（model.json 加载），/chat_stream 与 /ai_ops 只走参数校验不触发 LLM
func newTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	if _, err := config.LoadConfig("../../config.yaml"); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	convStore, err := store.NewConversationStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewConversationStore: %v", err)
	}

	fileService := service.NewFileService()
	collector := metrics.NewCollector()

	registry, err := modelmgr.Load("../../model.json")
	if err != nil {
		t.Fatalf("Load model.json: %v", err)
	}

	toolsHandler, err := tools.NewToolHandler(fileService, collector, func() modelmgr.Model {
		return registry.Default()
	})
	if err != nil {
		t.Fatalf("NewToolHandler: %v", err)
	}
	agentService := service.NewAgentService(toolsHandler, convStore, registry)
	hs := NewHandlers(convStore, agentService, fileService, collector)

	r := gin.New()
	r.Use(middleware.CORS())
	storeSess := cookie.NewStore([]byte("test-secret"))
	storeSess.Options(sessions.Options{Path: "/", MaxAge: 86400 * 7, HttpOnly: true})
	r.Use(sessions.Sessions("goagent_session", storeSess))
	r.Use(middleware.SessionIDMiddleware())
	SetupRoutesWith(r, hs)
	return r
}

// doRequest 执行请求并返回 recorder
func doRequest(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	return doRequestWithCookie(r, method, path, body, "")
}

// doRequestWithCookie 携带指定 cookie（复用 session）
func doRequestWithCookie(r *gin.Engine, method, path, body, cookie string) *httptest.ResponseRecorder {
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// 路由表测试：所有端点 method+path 命中且不 404
func TestRouteTable(t *testing.T) {
	r := newTestRouter(t)

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/commands"},
		{http.MethodGet, "/api/model"},
		{http.MethodPost, "/api/model"},
		{http.MethodGet, "/api/context/status"},
		{http.MethodGet, "/api/metrics"},
		{http.MethodGet, "/api/conversations"},
		{http.MethodPost, "/api/conversation"},
		{http.MethodGet, "/api/conversation/default"},
		{http.MethodDelete, "/api/conversation/default"},
		{http.MethodPost, "/api/commands/execute"},
		{http.MethodPost, "/api/upload_file"},
		{http.MethodPost, "/api/search"},
		{http.MethodPost, "/api/chat_stream"},
		{http.MethodPost, "/api/ai_ops"},
		{http.MethodPost, "/api/permission/response"},
		{http.MethodGet, "/context"},
	}

	for _, tc := range cases {
		w := doRequest(r, tc.method, tc.path, "")
		if w.Code == http.StatusNotFound {
			t.Errorf("%s %s → 404（路由未注册）", tc.method, tc.path)
		}
	}
}

// setAgentMode 中间件注入验证
func TestSetAgentModeInjection(t *testing.T) {
	captured := map[string]string{}
	r2 := gin.New()
	r2.Use(func(c *gin.Context) {
		c.Set("agent_mode", "chat")
		c.Next()
	})
	r2.Use(func(c *gin.Context) {
		if v, ok := c.Get("agent_mode"); ok {
			captured["mode"] = v.(string)
		}
		c.Next()
	})
	r2.POST("/api/chat_stream", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	doRequest(r2, http.MethodPost, "/api/chat_stream", `{"message":"hi"}`)
	if captured["mode"] != "chat" {
		t.Fatalf("agent_mode = %q, want chat", captured["mode"])
	}
}

func TestGetCommandsReturnsBuiltins(t *testing.T) {
	r := newTestRouter(t)
	w := doRequest(r, http.MethodGet, "/api/commands", "")
	if w.Code != 200 {
		t.Fatalf("GET /api/commands → %d", w.Code)
	}
	var resp struct {
		Commands []struct {
			Name       string `json:"name"`
			ArgsPolicy string `json:"args_policy"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	names := map[string]bool{}
	for _, c := range resp.Commands {
		names[c.Name] = true
	}
	for _, want := range []string{"help", "clear", "compact", "context", "model"} {
		if !names[want] {
			t.Errorf("命令缺失: %q", want)
		}
	}
	for _, c := range resp.Commands {
		if c.Name == "compact" && c.ArgsPolicy == "" {
			t.Errorf("compact 缺 args_policy")
		}
	}
}

func TestGetModel(t *testing.T) {
	r := newTestRouter(t)
	w := doRequest(r, http.MethodGet, "/api/model", "")
	if w.Code != 200 {
		t.Fatalf("GET /api/model → %d", w.Code)
	}
	var resp struct {
		Models  []struct {
			ID string `json:"id"`
		} `json:"models"`
		Current string `json:"current"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Models) == 0 {
		t.Fatalf("models 为空")
	}
	if resp.Current == "" {
		t.Fatalf("current 为空")
	}
}

func TestSwitchModelUnknownID(t *testing.T) {
	r := newTestRouter(t)
	w := doRequest(r, http.MethodPost, "/api/model", `{"id":"nonexistent"}`)
	if w.Code != 400 {
		t.Fatalf("切换不存在模型 → %d, want 400", w.Code)
	}
}

func TestGetContextStatus(t *testing.T) {
	r := newTestRouter(t)
	for _, path := range []string{"/api/context/status", "/context"} {
		w := doRequest(r, http.MethodGet, path, "")
		if w.Code != 200 {
			t.Fatalf("GET %s → %d", path, w.Code)
		}
		var resp struct {
			CurrentTokens int64   `json:"current_tokens"`
			MaxTokens     int64   `json:"max_tokens"`
			Percentage    float64 `json:"percentage"`
			MessageCount  int     `json:"message_count"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal %s: %v", path, err)
		}
		if resp.MaxTokens <= 0 {
			t.Errorf("%s: max_tokens = %d, want > 0", path, resp.MaxTokens)
		}
	}
}

func TestConversationCRUD(t *testing.T) {
	r := newTestRouter(t)

	w := doRequest(r, http.MethodPost, "/api/conversation", `{"title":"测试对话"}`)
	if w.Code != 200 {
		t.Fatalf("POST /api/conversation → %d", w.Code)
	}
	var created struct {
		Conversation struct {
			ID string `json:"id"`
		} `json:"conversation"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	id := created.Conversation.ID
	if id == "" {
		t.Fatal("conversation id 为空")
	}

	w = doRequest(r, http.MethodGet, "/api/conversations", "")
	if w.Code != 200 {
		t.Fatalf("GET /api/conversations → %d", w.Code)
	}

	w = doRequest(r, http.MethodGet, "/api/conversation/"+id, "")
	if w.Code != 200 {
		t.Fatalf("GET /api/conversation/%s → %d", id, w.Code)
	}

	w = doRequest(r, http.MethodDelete, "/api/conversation/"+id, "")
	if w.Code != 200 {
		t.Fatalf("DELETE /api/conversation/%s → %d", id, w.Code)
	}
}

func TestChatStreamEmptyMessageRejected(t *testing.T) {
	r := newTestRouter(t)
	w := doRequest(r, http.MethodPost, "/api/chat_stream", `{"message":""}`)
	if w.Code != 400 {
		t.Fatalf("空 message → %d, want 400", w.Code)
	}
}

func TestAIOpsEmptyMessageRejected(t *testing.T) {
	r := newTestRouter(t)
	w := doRequest(r, http.MethodPost, "/api/ai_ops", `{"message":""}`)
	if w.Code != 400 {
		t.Fatalf("空 message → %d, want 400", w.Code)
	}
}

func TestPermissionResponseUnknownID(t *testing.T) {
	r := newTestRouter(t)
	w := doRequest(r, http.MethodPost, "/api/permission/response", `{"id":"nonexistent","conversation_id":"default","approved":true}`)
	if w.Code != 404 {
		t.Fatalf("未知 request id → %d, want 404", w.Code)
	}
}

func TestMetricsEndpoint(t *testing.T) {
	r := newTestRouter(t)
	w := doRequest(r, http.MethodGet, "/api/metrics", "")
	if w.Code != 200 {
		t.Fatalf("GET /api/metrics → %d", w.Code)
	}
}

func TestUploadFileNoFileRejected(t *testing.T) {
	r := newTestRouter(t)
	w := doRequest(r, http.MethodPost, "/api/upload_file", "")
	if w.Code != 400 {
		t.Fatalf("无文件上传 → %d, want 400", w.Code)
	}
}

func TestSearchMissingQueryRejected(t *testing.T) {
	r := newTestRouter(t)
	w := doRequest(r, http.MethodPost, "/api/search", `{}`)
	if w.Code != 400 {
		t.Fatalf("缺 query → %d, want 400", w.Code)
	}
}

// 命令执行：未知命令 404
func TestExecuteUnknownCommand(t *testing.T) {
	r := newTestRouter(t)
	w := doRequest(r, http.MethodPost, "/api/commands/execute", `{"name":"nonexistent_cmd"}`)
	if w.Code != 404 {
		t.Fatalf("未知命令 → %d, want 404", w.Code)
	}
}

// 命令执行：builtin 命令落库 user 原文 + assistant 结果
func TestExecuteBuiltinCommandPersists(t *testing.T) {
	r := newTestRouter(t)

	w := doRequest(r, http.MethodPost, "/api/conversation", `{"title":"cmd"}`)
	cookie := w.Header().Get("Set-Cookie")
	if cookie == "" {
		t.Fatal("创建会话未返回 Set-Cookie")
	}
	var created struct {
		Conversation struct {
			ID string `json:"id"`
		} `json:"conversation"`
	}
	json.Unmarshal(w.Body.Bytes(), &created)
	id := created.Conversation.ID
	if id == "" {
		t.Fatal("conversation id 为空")
	}

	body := `{"name":"model","conversation_id":"` + id + `"}`
	w = doRequestWithCookie(r, http.MethodPost, "/api/commands/execute", body, cookie)
	if w.Code != 200 {
		t.Fatalf("execute model → %d, body=%s", w.Code, w.Body.String())
	}
	var exec struct {
		Type   string `json:"type"`
		Result string `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &exec); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if exec.Type != "builtin" {
		t.Fatalf("type = %q, want builtin", exec.Type)
	}

	w = doRequestWithCookie(r, http.MethodGet, "/api/conversation/"+id, "", cookie)
	var conv struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &conv); err != nil {
		t.Fatalf("unmarshal conv: %v", err)
	}
	if len(conv.Messages) < 2 {
		t.Fatalf("messages = %d, want ≥2（user 原文 + assistant 结果）", len(conv.Messages))
	}
	if conv.Messages[0].Role != "user" || !strings.Contains(conv.Messages[0].Content, "/model") {
		t.Errorf("第一条应为 user 命令原文, got %+v", conv.Messages[0])
	}
	if conv.Messages[1].Role != "assistant" || conv.Messages[1].Content == "" {
		t.Errorf("第二条应为 assistant 结果, got %+v", conv.Messages[1])
	}
}

// 会话隔离：不同 session 的 conversation 互不可见
func TestSessionIsolation(t *testing.T) {
	convStore, err := store.NewConversationStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sID := uuid.NewString()
	meta, err := convStore.CreateConversation(sID, "only")
	if err != nil {
		t.Fatal(err)
	}
	list, _ := convStore.ListConversations(sID)
	if len(list) != 1 || list[0].ID != meta.ID {
		t.Fatalf("session 列表 = %+v, want 仅自己的对话", list)
	}
}
