package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cloudwego/eino/schema"
)

type ConversationStore struct {
	dir string
}

type messageLine struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func NewConversationStore(dir string) (*ConversationStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create conversations dir: %w", err)
	}
	return &ConversationStore{dir: dir}, nil
}

func (s *ConversationStore) filePath(sessionID string) string {
	return filepath.Join(s.dir, sessionID+".jsonl")
}

// LoadHistory 加载指定sessionID的对话历史，返回消息列表
func (s *ConversationStore) LoadHistory(sessionID string) ([]*schema.Message, error) {
	// 打开文件
	f, err := os.Open(s.filePath(sessionID))
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
			continue // 跳过损坏的行
		}
		switch line.Role {
		case "user":
			msgs = append(msgs, schema.UserMessage(line.Content))
		case "assistant":
			msgs = append(msgs, schema.AssistantMessage(line.Content, nil))
		}
	}
	return msgs, scanner.Err()
}

// SaveMessages 将新的消息追加保存到指定sessionID的历史文件中
func (s *ConversationStore) SaveMessages(sessionID string, msgs ...*schema.Message) error {
	f, err := os.OpenFile(s.filePath(sessionID), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open history for write: %w", err)
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	for _, msg := range msgs {
		line := messageLine{Role: string(msg.Role), Content: msg.Content}
		if err := encoder.Encode(line); err != nil {
			return fmt.Errorf("write message: %w", err)
		}
	}
	return nil
}