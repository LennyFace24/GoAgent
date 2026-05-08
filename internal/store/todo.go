package store

import (
	"sync"
)

type TodoItem struct {
	Content string `json:"content"` // 这一步要做什么
	Status  string `json:"status"` // "pending" | "in_progress" | "completed",
	ActiveForm string `json:"active_form"` // 当它正在进行中时，可以用更自然的进行时描述
}

type TodoStore struct {
	mu sync.Mutex
	todos map[string][]TodoItem // session_id -> 待办事项列表
}

// 添加待办事项
func NewTodoStore() *TodoStore {
	return &TodoStore{
		todos: make(map[string][]TodoItem),
	}
}

func (s *TodoStore) AddTodo(sessionID, content string, activeForm string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.todos[sessionID] = append(s.todos[sessionID], TodoItem{
		Content: content,
		Status: "pending",
		ActiveForm: activeForm,
	})
}
