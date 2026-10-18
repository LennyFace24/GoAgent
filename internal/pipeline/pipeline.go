// Package pipeline 实现消息预处理管线。
//
// 生产流程：SkillExpand → ModelPrep → ContextBuild → CompactCheck。
// 命令由单独端点处理；事件 Hook 层不实现，管线固定为这四个 Stage。
// 管线只负责请求准备；AgentService.Stream 在其成功返回后启动独立 runLoop。
package pipeline

import (
	"context"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	ctxmgr "github.com/LennyFace24/MiniAgent/internal/context"
	"github.com/LennyFace24/MiniAgent/internal/skills"
	contexttool "github.com/LennyFace24/MiniAgent/internal/tools/context_tool"
)

// MessageContext 贯穿管线的请求上下文
type MessageContext struct {
	Raw            string // 原始用户消息
	Mode           string // chat | aiops
	SessionID      string
	ConversationID string

	// 阶段 3 填充：匹配到的 skill
	Skill *skills.SkillDocument

	// 阶段 4 填充：模型与认证
	ChatModel model.BaseChatModel
	ToolModel model.ToolCallingChatModel

	// 阶段 5/6 填充：上下文状态、上下文管理器与最终消息
	CtxState *contexttool.ContextState
	Manager  *ctxmgr.ContextManager
	History  []*schema.Message
	Messages []*schema.Message

	// PersistCompaction 由 AgentService 注入，把一次完整压缩的结果写入对话历史。
	PersistCompaction func(*contexttool.CompactionResult) error
}

// Stage 管线阶段
type Stage interface {
	Name() string
	Process(ctx context.Context, mc *MessageContext) error
}

// Pipeline 顺序执行阶段
type Pipeline struct {
	stages []Stage
}

// New 创建管线
func New(stages ...Stage) *Pipeline {
	return &Pipeline{stages: stages}
}

// Run 顺序执行所有阶段
func (p *Pipeline) Run(ctx context.Context, mc *MessageContext) error {
	for _, s := range p.stages {
		if err := s.Process(ctx, mc); err != nil {
			return err
		}
	}
	return nil
}
