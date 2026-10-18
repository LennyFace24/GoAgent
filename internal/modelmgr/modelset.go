package modelmgr

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ModelSet 一个服务实例的模型状态：当前激活模型 + 惰性构建的模型实例。
//
// 设计要点（每服务 Model 对象，非全局唯一变量）：
//   - Registry 可共享：只读的 model.json 注册表，多服务可共用
//   - ModelSet 每服务独立：持有 currentID + 构建出的 ChatModel/ToolModel，
//     每个服务使用自己的 ModelSet，互不干扰
//   - Snapshot 同时选定对话模型、摘要模型和预算；Switch 只影响之后的快照。
type ModelSet struct {
	registry  *Registry
	tools     []tool.BaseTool
	mu        sync.RWMutex
	currentID string
	chatModel model.ToolCallingChatModel
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

// ModelSnapshot 固定一次请求使用的模型实例及其配置。
type ModelSnapshot struct {
	Config    Model
	ChatModel model.BaseChatModel
	ToolModel model.ToolCallingChatModel
}

// Snapshot 在模型选择锁的保护下准备两个模型，避免并发的 Switch 把新预算
// 或新摘要模型混入正在执行的请求。
func (ms *ModelSet) Snapshot(ctx context.Context) (ModelSnapshot, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	m, ok := ms.registry.Get(ms.currentID)
	if !ok {
		return ModelSnapshot{}, fmt.Errorf("模型 %q 不存在", ms.currentID)
	}
	if ms.chatModel == nil {
		cm, err := m.BuildChatModel(ctx)
		if err != nil {
			return ModelSnapshot{}, err
		}
		ms.chatModel = cm
	}
	if ms.toolModel == nil {
		infos := make([]*schema.ToolInfo, 0, len(ms.tools))
		for _, registered := range ms.tools {
			info, err := registered.Info(ctx)
			if err != nil {
				return ModelSnapshot{}, fmt.Errorf("获取工具信息失败: %w", err)
			}
			infos = append(infos, info)
		}
		ms.toolModel = ms.chatModel
		if len(infos) > 0 {
			bound, err := ms.chatModel.WithTools(infos)
			if err != nil {
				ms.toolModel = nil
				return ModelSnapshot{}, err
			}
			ms.toolModel = bound
		}
	}
	return ModelSnapshot{Config: m, ChatModel: ms.chatModel, ToolModel: ms.toolModel}, nil
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
