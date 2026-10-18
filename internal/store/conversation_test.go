package store

import (
	"errors"
	"fmt"
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
}
