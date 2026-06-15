package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"log"

	"github.com/cloudwego/eino/schema"
)

type ConversationMeta struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type ConversationStore struct {
	dir string
}

type messageLine struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	ToolCalls  []toolCallLine `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	ToolName   string         `json:"tool_name,omitempty"`
}

type toolCallLine struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
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
	return &ConversationStore{dir: dir}, nil
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
	if err := os.MkdirAll(s.sessionDir(sessionID), 0755); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}

	id := fmt.Sprintf("conv_%d", time.Now().UnixMilli())
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
	dir := s.sessionDir(sessionID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read session dir: %w", err)
	}

	var metas []*ConversationMeta
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

// LoadHistory 加载指定对话的历史消息
func (s *ConversationStore) LoadHistory(sessionID, conversationID string) ([]*schema.Message, error) {
	f, err := os.Open(s.filePath(sessionID, conversationID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open history: %w", err)
	}
	defer f.Close()

	var msgs []*schema.Message
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var line messageLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		};
		switch line.Role {
		case "user":
			msgs = append(msgs, schema.UserMessage(line.Content))
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
			msgs = append(msgs, schema.AssistantMessage(line.Content, toolCalls))
		case "tool":
			msgs = append(msgs, schema.ToolMessage(line.Content, line.ToolCallID, schema.WithToolName(line.ToolName)))
		}
	}
	log.Printf("加载对话历史：会话 %s, 对话 %s, 消息数 %d", sessionID, conversationID, len(msgs))
	return msgs, scanner.Err()
}

// SaveMessages 保存消息到指定对话
func (s *ConversationStore) SaveMessages(sessionID, conversationID string, msgs ...*schema.Message) error {
	if err := os.MkdirAll(s.sessionDir(sessionID), 0755); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}

	f, err := os.OpenFile(s.filePath(sessionID, conversationID), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open history for write: %w", err)
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	for _, msg := range msgs {
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
		if msg.ToolCallID != "" {
			line.ToolCallID = msg.ToolCallID
		}
		if msg.ToolName != "" {
			line.ToolName = msg.ToolName
		}
		if err := encoder.Encode(line); err != nil {
			return fmt.Errorf("write message: %w", err)
		}
	}

	// 更新 meta 的 UpdatedAt
	meta, err := s.loadMeta(sessionID, conversationID)
	if err == nil {
		meta.UpdatedAt = time.Now().Format(time.RFC3339)
		_ = s.saveMeta(sessionID, meta)
	}

	return nil
}

// DeleteConversation 删除对话及其元数据
func (s *ConversationStore) DeleteConversation(sessionID, conversationID string) error {
	os.Remove(s.filePath(sessionID, conversationID))
	os.Remove(s.metaPath(sessionID, conversationID))
	return nil
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
