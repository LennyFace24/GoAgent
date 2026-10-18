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
// 若存在 compaction 记录，只返回最新摘要及其之后的原始消息。
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

	var msgs []*schema.Message
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
			// append-only 文件里 compaction 之后的记录才是当前有效历史。
			msgs = nil
			if line.Summary != "" {
				msgs = append(msgs, schema.UserMessage(line.Summary))
			}
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
		msgs = append(msgs, msg)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan history: %w", err)
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

// SaveCompaction 原子重写历史文件：写入最新摘要和仍在窗口内的原始消息。
// 这样压缩结果会落盘，文件也不会随对话无限增长。
func (s *ConversationStore) SaveCompaction(sessionID, conversationID, summary, firstKeptID string, kept []*schema.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := s.sessionDir(sessionID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}

	tmp, err := os.CreateTemp(dir, conversationID+".compact-*")
	if err != nil {
		return fmt.Errorf("create compaction temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	encoder := json.NewEncoder(tmp)
	if err := encoder.Encode(messageLine{
		Role:        compactionRole,
		Summary:     summary,
		FirstKeptID: firstKeptID,
	}); err != nil {
		tmp.Close()
		return fmt.Errorf("write compaction record: %w", err)
	}
	for i, msg := range kept {
		line := encodeMessageLine(msg)
		line.ID = strconv.Itoa(i + 1)
		if err := encoder.Encode(line); err != nil {
			tmp.Close()
			return fmt.Errorf("write kept message: %w", err)
		}
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync compaction file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close compaction file: %w", err)
	}

	path := s.filePath(sessionID, conversationID)
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace history with compaction: %w", err)
	}
	s.sequences[path] = uint64(len(kept) + 1)
	log.Printf("对话 %s/%s 已压缩：保留 %d 条原始消息", sessionID, conversationID, len(kept))
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
