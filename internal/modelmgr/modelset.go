package modelmgr

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
)

// ModelSet 一个服务实例的模型状态：当前激活模型 + 惰性构建的模型实例。
//
// 设计要点（每服务 Model 对象，非全局唯一变量）：
//   - Registry 可共享：只读的 model.json 注册表，多服务可共用
//   - ModelSet 每服务独立：持有 currentID + 构建出的 ChatModel/ToolModel，
//     每个服务使用自己的 ModelSet，互不干扰
//   - Switch 重建：切换模型时重建 ChatModel/ToolModel（重绑工具），
//     在途请求持读锁继续用旧模型，写锁切换
type ModelSet struct {
	registry  *Registry
	tools     []tool.BaseTool
	mu        sync.RWMutex
	currentID string
	chatModel model.BaseChatModel
	toolModel model.ToolCallingChatModel
}

// NewModelSet 创建服务级模型集合，初始使用 registry 的默认模型
func NewModelSet(registry *Registry, tools []tool.BaseTool) *ModelSet {
	return &ModelSet{
		registry:  registry,
		tools:     tools,
		currentID: registry.DefaultID(),
	}
}

// CurrentID 返回当前激活模型 id
func (ms *ModelSet) CurrentID() string {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.currentID
}

// Current 返回当前激活模型配置（不构建）
func (ms *ModelSet) Current() Model {
	id := ms.CurrentID()
	m, _ := ms.registry.Get(id)
	return m
}

// ListModels 返回可用模型列表（供前端展示）
func (ms *ModelSet) ListModels() []Model {
	return ms.registry.List()
}

// ChatModel 返回惰性构建的 ChatModel（无工具绑定）
func (ms *ModelSet) ChatModel(ctx context.Context) (model.BaseChatModel, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	if ms.chatModel != nil {
		return ms.chatModel, nil
	}
	m, ok := ms.registry.Get(ms.currentID)
	if !ok {
		return nil, fmt.Errorf("模型 %q 不存在", ms.currentID)
	}
	cm, err := m.BuildChatModel(ctx)
	if err != nil {
		return nil, err
	}
	ms.chatModel = cm
	return cm, nil
}

// ToolModel 返回惰性构建的带工具模型
func (ms *ModelSet) ToolModel(ctx context.Context) (model.ToolCallingChatModel, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	if ms.toolModel != nil {
		return ms.toolModel, nil
	}
	m, ok := ms.registry.Get(ms.currentID)
	if !ok {
		return nil, fmt.Errorf("模型 %q 不存在", ms.currentID)
	}
	tm, err := m.BuildToolCallingModel(ctx, ms.tools)
	if err != nil {
		return nil, err
	}
	ms.toolModel = tm
	return tm, nil
}

// Switch 切换模型。id 不存在返回错误；成功时重建 ChatModel/ToolModel。
func (ms *ModelSet) Switch(ctx context.Context, id string) error {
	m, ok := ms.registry.Get(id)
	if !ok {
		return fmt.Errorf("模型 %q 不存在（可用: %s）", id, ms.registry.IDs())
	}

	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.currentID = m.ID

	// 惰性缓存失效：下次取用时重建
	ms.chatModel = nil
	ms.toolModel = nil
	_ = ctx
	return nil
}
