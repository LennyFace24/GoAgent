package commands

import (
	"sync"
)

// ---------- 命令注册中心 ----------

// Registry 命令注册中心
type Registry struct {
	mu       sync.RWMutex
	commands []Command
}

var (
	globalRegistry *Registry
	once           sync.Once
)

// GetRegistry 获取全局命令注册中心
func GetRegistry() *Registry {
	once.Do(func() {
		globalRegistry = &Registry{}
		registerBuiltinCommands(globalRegistry)
	})
	return globalRegistry
}

// Register 注册命令
func (r *Registry) Register(cmd Command) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.commands = append(r.commands, cmd)
}

// List 获取所有命令
func (r *Registry) List() []Command {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Command, len(r.commands))
	copy(result, r.commands)
	return result
}

// Find 根据名称查找命令
func (r *Registry) Find(name string) (Command, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, cmd := range r.commands {
		if cmd.Name == name {
			return cmd, true
		}
	}
	return Command{}, false
}
