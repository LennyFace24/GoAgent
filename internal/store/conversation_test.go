package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func newTestStore(t *testing.T) *ConversationStore {
	t.Helper()
	s, err := NewConversationStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewConversationStore: %v", err)
	}
	return s
}

func TestSaveAndLoadHistoryRoundTrip(t *testing.T) {
	s := newTestStore(t)
	const sessionID = "sess1"
	meta, err := s.CreateConversation(sessionID, "测试对话")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	convID := meta.ID

	// 保存一条用户消息 + 一条助手消息
	if err := s.SaveMessages(sessionID, convID,
		schema.UserMessage("你好"),
		schema.AssistantMessage("你好，有什么可以帮你？", nil),
	); err != nil {
		t.Fatalf("SaveMessages: %v", err)
	}

	msgs, err := s.LoadHistory(sessionID, convID)
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("len(msgs) = %d, want 2", len(msgs))
	}
	if msgs[0].Role != schema.User || msgs[0].Content != "你好" {
		t.Fatalf("msgs[0] = %+v, want user/你好", msgs[0])
	}
	if msgs[1].Role != schema.Assistant || msgs[1].Content != "你好，有什么可以帮你？" {
		t.Fatalf("msgs[1] = %+v, want assistant reply", msgs[1])
	}
}

func TestLoadHistoryEmptyWhenMissing(t *testing.T) {
	s := newTestStore(t)
	msgs, err := s.LoadHistory("sess", "no-such-conv")
	if err != nil {
		t.Fatalf("LoadHistory on missing conv: %v", err)
	}
	if msgs != nil && len(msgs) != 0 {
		t.Fatalf("expected nil/empty, got %d msgs", len(msgs))
	}
}

// TestConcurrentSaveNoDataLoss 验证并发追加写同一会话不会丢消息（mutex 保护的回归测试）。
func TestConcurrentSaveNoDataLoss(t *testing.T) {
	s := newTestStore(t)
	const sessionID = "sess"
	meta, err := s.CreateConversation(sessionID, "并发")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	convID := meta.ID

	const goroutines = 20
	const perGoroutine = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				if err := s.SaveMessages(sessionID, convID,
					schema.UserMessage(fmt.Sprintf("g%d-i%d", g, i)),
				); err != nil {
					t.Errorf("SaveMessages: %v", err)
				}
			}
		}(g)
	}
	wg.Wait()

	msgs, err := s.LoadHistory(sessionID, convID)
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	want := goroutines * perGoroutine
	if len(msgs) != want {
		t.Fatalf("len(msgs) = %d, want %d (并发写入丢失数据)", len(msgs), want)
	}
}

func TestListConversationsReturnsAllCreated(t *testing.T) {
	s := newTestStore(t)
	const sessionID = "sess"
	m1, err := s.CreateConversation(sessionID, "c1")
	if err != nil {
		t.Fatalf("CreateConversation c1: %v", err)
	}
	m2, err := s.CreateConversation(sessionID, "c2")
	if err != nil {
		t.Fatalf("CreateConversation c2: %v", err)
	}

	list, err := s.ListConversations(sessionID)
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len(list) = %d, want 2", len(list))
	}
	// UpdatedAt 为 RFC3339 秒级精度，同秒内创建顺序不保证，仅校验两者都在列表中
	got := map[string]bool{}
	for _, m := range list {
		got[m.ID] = true
	}
	if !got[m1.ID] || !got[m2.ID] {
		t.Fatalf("list 缺少创建的会话: got=%v want %s,%s", got, m1.ID, m2.ID)
	}
}

func TestRenameConversationUpdatesTitleAndTimestamp(t *testing.T) {
	s := newTestStore(t)
	const sessionID = "sess"
	meta, err := s.CreateConversation(sessionID, "旧名称")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	renamed, err := s.RenameConversation(sessionID, meta.ID, "新名称")
	if err != nil {
		t.Fatalf("RenameConversation: %v", err)
	}
	if renamed.Title != "新名称" || renamed.UpdatedAt == "" {
		t.Fatalf("rename did not update title: %#v", renamed)
	}

	loaded, err := s.ListConversations(sessionID)
	if err != nil || len(loaded) != 1 || loaded[0].Title != "新名称" {
		t.Fatalf("renamed title not persisted: %#v (%v)", loaded, err)
	}

	if _, err := s.RenameConversation(sessionID, "missing", "x"); !errors.Is(err, ErrConversationNotFound) {
		t.Fatalf("missing conversation error = %v, want ErrConversationNotFound", err)
	}
}

func TestDeleteConversationRemovesHistory(t *testing.T) {
	s := newTestStore(t)
	const sessionID = "sess"
	meta, _ := s.CreateConversation(sessionID, "c")
	convID := meta.ID
	_ = s.SaveMessages(sessionID, convID, schema.UserMessage("x"))

	if err := s.DeleteConversation(sessionID, convID); err != nil {
		t.Fatalf("DeleteConversation: %v", err)
	}
	msgs, _ := s.LoadHistory(sessionID, convID)
	if len(msgs) != 0 {
		t.Fatalf("after delete, len(msgs) = %d, want 0", len(msgs))
	}
	if err := s.DeleteConversation(sessionID, convID); err != nil {
		t.Fatalf("delete missing conversation should be idempotent: %v", err)
	}
}

func TestDeleteConversationReportsRemovalErrors(t *testing.T) {
	s := newTestStore(t)
	const sessionID = "sess"
	meta, err := s.CreateConversation(sessionID, "c")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveMessages(sessionID, meta.ID, schema.UserMessage("x")); err != nil {
		t.Fatal(err)
	}

	// 用非空目录占住历史文件路径，制造跨平台可复现的删除失败。
	historyPath := s.filePath(sessionID, meta.ID)
	if err := os.Remove(historyPath); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(historyPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(historyPath, "block"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	err = s.DeleteConversation(sessionID, meta.ID)
	if err == nil || !strings.Contains(err.Error(), filepath.Base(historyPath)) {
		t.Fatalf("delete error = %v, want wrapped history path error", err)
	}
}

func TestLoadHistorySupportsLinesLargerThanScannerDefault(t *testing.T) {
	s := newTestStore(t)
	const sessionID = "sess"
	meta, err := s.CreateConversation(sessionID, "long")
	if err != nil {
		t.Fatal(err)
	}
	longContent := strings.Repeat("工具输出", 40_000) // 约 480KB，远超 64KB 默认上限
	if err := s.SaveMessages(sessionID, meta.ID, schema.ToolMessage(longContent, "call-1", schema.WithToolName("bash"))); err != nil {
		t.Fatal(err)
	}

	msgs, err := s.LoadHistory(sessionID, meta.ID)
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("long line load returned %d messages, want 1", len(msgs))
	}
	if msgs[0].Content != longContent {
		t.Fatalf("long line truncated: got %d bytes, want %d", len(msgs[0].Content), len(longContent))
	}
}

func TestLoadHistoryAppliesCompactionAndKeepsLaterMessages(t *testing.T) {
	s := newTestStore(t)
	const sessionID = "sess"
	meta, err := s.CreateConversation(sessionID, "compact")
	if err != nil {
		t.Fatal(err)
	}
	convID := meta.ID
	if err := s.SaveMessages(sessionID, convID,
		schema.UserMessage("old-user"),
		schema.AssistantMessage("old-reply", nil),
		schema.UserMessage("recent-user"),
		schema.AssistantMessage("recent-reply", nil),
	); err != nil {
		t.Fatal(err)
	}

	if err := s.SaveCompaction(sessionID, convID, "summary-marker", "3"); err != nil {
		t.Fatalf("SaveCompaction: %v", err)
	}

	msgs, err := s.LoadHistory(sessionID, convID)
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(msgs) != 3 || msgs[0].Content != "summary-marker" || msgs[1].Content != "recent-user" || msgs[2].Content != "recent-reply" {
		t.Fatalf("compaction boundary not applied: %v", msgs)
	}

	raw, err := os.ReadFile(s.filePath(sessionID, convID))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 5 {
		t.Fatalf("append-only history has %d lines, want 5", len(lines))
	}
	if !strings.Contains(lines[0], `"old-user"`) {
		t.Fatalf("old message was removed from history: %s", lines[0])
	}
	var record struct {
		Role        string `json:"role"`
		Summary     string `json:"summary"`
		FirstKeptID string `json:"first_kept_id"`
	}
	if err := json.Unmarshal([]byte(lines[4]), &record); err != nil {
		t.Fatal(err)
	}
	if record.Role != "compaction" || record.Summary != "summary-marker" || record.FirstKeptID != "3" {
		t.Fatalf("compaction record = %+v", record)
	}

	// 压缩后的新消息继续追加，加载时仍保留摘要与顺序。
	if err := s.SaveMessages(sessionID, convID, schema.UserMessage("after-compaction")); err != nil {
		t.Fatal(err)
	}
	msgs, err = s.LoadHistory(sessionID, convID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 4 || msgs[3].Content != "after-compaction" {
		t.Fatalf("messages after compaction were lost: %v", msgs)
	}
}

func TestLoadHistorySupportsLegacyCompactionAtFileStart(t *testing.T) {
	s := newTestStore(t)
	const sessionID = "sess"
	meta, err := s.CreateConversation(sessionID, "legacy-compact")
	if err != nil {
		t.Fatal(err)
	}
	convID := meta.ID
	path := s.filePath(sessionID, convID)
	if err := os.MkdirAll(s.sessionDir(sessionID), 0755); err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		`{"role":"compaction","summary":"legacy-summary","first_kept_id":"2"}`,
		`{"id":"1","role":"user","content":"kept-message-1"}`,
		`{"id":"2","role":"user","content":"kept-message-2"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	msgs, err := s.LoadHistory(sessionID, convID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 3 || msgs[0].Content != "legacy-summary" || msgs[1].Content != "kept-message-1" || msgs[2].Content != "kept-message-2" {
		t.Fatalf("legacy compaction was not loaded: %v", msgs)
	}
}
