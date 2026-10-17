// Package pipeline 实现消息预处理管线。
//
// 目标流程（对齐规划文档 docs/plan/message-pipeline.md）：
//
//	1. CommandCheck    —— 命令命中已在 handler 层完成（/api/commands/execute 短路），
//	                      本管线接收的是确认非命令的普通消息，故不重复检查。
//	2. EventTrigger    —— 事件总线：pre_process / pre_send 等 hook 链（插件注入点）
//	3. SkillExpand     —— 触发词匹配 skill，注入 skill 全文到上下文
//	4. ModelPrep       —— 模型实例与认证准备
//	5. CompactCheck    —— 历史预算检查 → MicroCompact / CompactFunc
//	6. ContextBuild    —— ContextManager 组装 slots + BuildMessages
//	7. Send            —— 由 AgentService.runLoop 消费 Messages 启动流式
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
