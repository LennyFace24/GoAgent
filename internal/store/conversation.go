package store

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

// ErrConversationNotFound 表示指定对话的元数据不存在。
var ErrConversationNotFound = errors.New("conversation not found")

const (
	compactionRole = "compaction"
	maxLineBytes   = 8 * 1024 * 1024
)

type ConversationMeta struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type ConversationStore struct {
	dir string
	mu  sync.Mutex // 保护同一会话文件的并发读写，避免 JSONL 行交错
	// sequences 缓存每个历史文件的下一条消息 ID，避免每次追加都扫描全文件。
	sequences map[string]uint64
}

type messageLine struct {
	ID          string         `json:"id,omitempty"`
	Role        string         `json:"role"`
	Content     string         `json:"content"`
	Summary     string         `json:"summary,omitempty"`
	FirstKeptID string         `json:"first_kept_id,omitempty"`
	ToolCalls   []toolCallLine `json:"tool_calls,omitempty"`
	ToolCallID  string         `json:"tool_call_id,omitempty"`
	ToolName    string         `json:"tool_name,omitempty"`
}

type toolCallLine struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function functionCallLine `json:"function"`
}

type functionCallLine struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

func NewConversationStore(dir string) (*ConversationStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create conversations dir: %w", err)
	}
	return &ConversationStore{dir: dir, sequences: make(map[string]uint64)}, nil
}

func (s *ConversationStore) sessionDir(sessionID string) string {
	return filepath.Join(s.dir, sessionID)
}

func (s *ConversationStore) filePath(sessionID, conversationID string) string {
	return filepath.Join(s.sessionDir(sessionID), conversationID+".jsonl")
}

func (s *ConversationStore) metaPath(sessionID, conversationID string) string {
	return filepath.Join(s.sessionDir(sessionID), conversationID+".meta.json")
}

// CreateConversation 创建新对话，返回 conversationID
func (s *ConversationStore) CreateConversation(sessionID, title string) (*ConversationMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.sessionDir(sessionID), 0755); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}

	// 用 uuid 避免同一毫秒内创建多个对话时 ID 冲突（此前 conv_<UnixMilli> 会碰撞覆盖）
	id := "conv_" + uuid.New().String()
	now := time.Now().Format(time.RFC3339)

	meta := &ConversationMeta{
		ID:        id,
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.saveMeta(sessionID, meta); err != nil {
		return nil, err
	}
	return meta, nil
}

// ListConversations 返回指定 session 的所有对话元数据，按更新时间倒序
func (s *ConversationStore) ListConversations(sessionID string) ([]*ConversationMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := s.sessionDir(sessionID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read session dir: %w", err)
	}

	var metas = make([]*ConversationMeta, 0)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		metaPath := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var meta ConversationMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			continue
		}
		metas = append(metas, &meta)
	}

	// 按更新时间倒序排列
	for i := 0; i < len(metas); i++ {
		for j := i + 1; j < len(metas); j++ {
			if metas[j].UpdatedAt > metas[i].UpdatedAt {
				metas[i], metas[j] = metas[j], metas[i]
			}
		}
	}

	return metas, nil
}

// LoadHistory 加载指定对话的历史消息。
// 文件保持 append-only；最新 compaction 之前的消息保留在磁盘，
// 但只返回摘要、first_kept_id 开始的消息，以及 compaction 之后的新消息。
func (s *ConversationStore) LoadHistory(sessionID, conversationID string) ([]*schema.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.Open(s.filePath(sessionID, conversationID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open history: %w", err)
	}
	defer f.Close()

	type historyEntry struct {
		id  string
		msg *schema.Message
	}
	var entries []historyEntry
	var summary, firstKeptID string
	latestCompactionAt := -1
	var messageIndex uint64
	scanner := bufio.NewScanner(f)
	// 工具输出可能远大于 Scanner 默认的 64KB 单行上限。
	scanner.Buffer(make([]byte, 64*1024), maxLineBytes)
	for scanner.Scan() {
		var line messageLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}
		if line.Role == compactionRole {
			summary = line.Summary
			firstKeptID = line.FirstKeptID
			latestCompactionAt = len(entries)
			continue
		}
		messageIndex++
		id := line.ID
		if id == "" {
			// 兼容旧文件：用消息在文件中的序号补一个稳定 ID。
			id = strconv.FormatUint(messageIndex, 10)
		}
		msg := decodeMessageLine(line)
		if msg == nil {
			continue
		}
		if msg.Extra == nil {
			msg.Extra = make(map[string]any, 1)
		}
		msg.Extra["history_id"] = id
		entries = append(entries, historyEntry{id: id, msg: msg})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan history: %w", err)
	}

	start := 0
	if latestCompactionAt >= 0 {
		start = latestCompactionAt
		// 旧版本曾把 compaction 写在文件开头并重写文件，此时 first_kept_id
		// 指向重写前被删除的消息，不能用于过滤；只认真正追加在消息之后的记录。
		if latestCompactionAt > 0 {
			for i := range entries {
				if entries[i].id == firstKeptID {
					start = i
					break
				}
			}
		}
	}

	msgs := make([]*schema.Message, 0, len(entries)-start+1)
	if latestCompactionAt >= 0 && summary != "" {
		msgs = append(msgs, schema.UserMessage(summary))
	}
	for i := start; i < len(entries); i++ {
		msgs = append(msgs, entries[i].msg)
	}
	log.Printf("加载对话历史：会话 %s, 对话 %s, 消息数 %d", sessionID, conversationID, len(msgs))
	return msgs, nil
}

func decodeMessageLine(line messageLine) *schema.Message {
	switch line.Role {
	case "user":
		return schema.UserMessage(line.Content)
	case "assistant":
		var toolCalls []schema.ToolCall
		for _, tc := range line.ToolCalls {
			toolCalls = append(toolCalls, schema.ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: schema.FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}
		return schema.AssistantMessage(line.Content, toolCalls)
	case "tool":
		return schema.ToolMessage(line.Content, line.ToolCallID, schema.WithToolName(line.ToolName))
	default:
		return nil
	}
}

func encodeMessageLine(msg *schema.Message) messageLine {
	line := messageLine{Role: string(msg.Role), Content: msg.Content}
	if len(msg.ToolCalls) > 0 {
		line.ToolCalls = make([]toolCallLine, len(msg.ToolCalls))
		for i, tc := range msg.ToolCalls {
			line.ToolCalls[i] = toolCallLine{
				ID:   tc.ID,
				Type: tc.Type,
				Function: functionCallLine{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		}
	}
	line.ToolCallID = msg.ToolCallID
	line.ToolName = msg.ToolName
	return line
}

// nextMessageIDLocked 返回该历史文件的下一条消息 ID。调用方必须持有 s.mu。
func (s *ConversationStore) nextMessageIDLocked(path string) string {
	next := s.sequences[path]
	if next == 0 {
		next = 1
		if f, err := os.Open(path); err == nil {
			var count uint64
			scanner := bufio.NewScanner(f)
			scanner.Buffer(make([]byte, 64*1024), maxLineBytes)
			for scanner.Scan() {
				var line messageLine
				if json.Unmarshal(scanner.Bytes(), &line) == nil && line.Role != compactionRole {
					count++
				}
			}
			_ = f.Close()
			next = count + 1
		}
	}
	s.sequences[path] = next + 1
	return strconv.FormatUint(next, 10)
}

// SaveMessages 保存消息到指定对话
func (s *ConversationStore) SaveMessages(sessionID, conversationID string, msgs ...*schema.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.sessionDir(sessionID), 0755); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}

	path := s.filePath(sessionID, conversationID)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open history for write: %w", err)
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	for _, msg := range msgs {
		line := encodeMessageLine(msg)
		line.ID = s.nextMessageIDLocked(path)
		if err := encoder.Encode(line); err != nil {
			return fmt.Errorf("write message: %w", err)
		}
	}

	// 更新 meta 的 UpdatedAt
	meta, err := s.loadMeta(sessionID, conversationID)
	if err == nil {
		meta.UpdatedAt = time.Now().Format(time.RFC3339)
		if err := s.saveMeta(sessionID, meta); err != nil {
			return fmt.Errorf("update conversation meta: %w", err)
		}
	}

	return nil
}

// SaveCompaction 追加一条压缩记录，不删除旧消息、不重写历史文件。
// 后续加载通过 first_kept_id 截断发送给模型的消息窗口。
func (s *ConversationStore) SaveCompaction(sessionID, conversationID, summary, firstKeptID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.sessionDir(sessionID), 0755); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}

	path := s.filePath(sessionID, conversationID)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open history for compaction: %w", err)
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(messageLine{
		Role:        compactionRole,
		Summary:     summary,
		FirstKeptID: firstKeptID,
	}); err != nil {
		return fmt.Errorf("write compaction record: %w", err)
	}
	log.Printf("对话 %s/%s 已追加压缩记录：保留窗口从 %s 开始", sessionID, conversationID, firstKeptID)
	return nil
}

// RenameConversation 更新指定对话的标题和更新时间。
func (s *ConversationStore) RenameConversation(sessionID, conversationID, title string) (*ConversationMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	meta, err := s.loadMeta(sessionID, conversationID)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrConversationNotFound
		}
		return nil, fmt.Errorf("load conversation meta: %w", err)
	}
	meta.Title = title
	meta.UpdatedAt = time.Now().Format(time.RFC3339)
	if err := s.saveMeta(sessionID, meta); err != nil {
		return nil, err
	}
	return meta, nil
}

// DeleteConversation 删除对话及其元数据。
// 文件已不存在视为成功；其余错误汇总返回，调用方据此报告失败。
func (s *ConversationStore) DeleteConversation(sessionID, conversationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var errs []error
	for _, path := range []string{
		s.filePath(sessionID, conversationID),
		s.metaPath(sessionID, conversationID),
	} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("remove %s: %w", filepath.Base(path), err))
		}
	}
	delete(s.sequences, s.filePath(sessionID, conversationID))
	return errors.Join(errs...)
}

func (s *ConversationStore) saveMeta(sessionID string, meta *ConversationMeta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}
	return os.WriteFile(s.metaPath(sessionID, meta.ID), data, 0644)
}

func (s *ConversationStore) loadMeta(sessionID, conversationID string) (*ConversationMeta, error) {
	data, err := os.ReadFile(s.metaPath(sessionID, conversationID))
	if err != nil {
		return nil, err
	}
	var meta ConversationMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}
