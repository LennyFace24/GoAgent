package context

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LennyFace24/MiniAgent/internal/prompt"
	"github.com/LennyFace24/MiniAgent/internal/skills"
	contexttool "github.com/LennyFace24/MiniAgent/internal/tools/context_tool"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// Slot 代表上下文的一个模块
type Slot struct {
	Tag     string // XML 标签名
	Content string // 模块内容
	Order   int    // 排序，越小越靠前
}

// ContextManager 负责组装完整的上下文
type ContextManager struct {
	slots       []Slot
	soulPath    string
	memoryPath  string
	skillReg    *skills.SkillRegistry
	budget      int64
}

// New 创建 ContextManager
func New(budget int64, skillReg *skills.SkillRegistry) *ContextManager {
	return &ContextManager{
		soulPath:   filepath.Join("data", "soul.md"),
		memoryPath: filepath.Join("data", "memory.md"),
		skillReg:   skillReg,
		budget:     budget,
	}
}

// --- Slot 注入方法 ---

// SetMode 根据 mode 设置 global_rules 内容
func (cm *ContextManager) SetMode(mode string) {
	var content string
	switch mode {
	case "aiops":
		content = prompt.AIOpsCoreBlock().Content
	default:
		content = prompt.CoreBlock().Content
	}
	// 追加工具规范
	content += "\n\n" + prompt.ToolsRuleBlock().Content
	cm.setSlot("global_rules", content, 0)
}

// LoadSoul 从文件加载 SOUL.md，文件不存在则跳过
func (cm *ContextManager) LoadSoul() {
	data, err := os.ReadFile(cm.soulPath)
	if err != nil {
		return // 文件不存在，跳过
	}
	cm.setSlot("soul", strings.TrimSpace(string(data)), 1)
}

// LoadProjectContext 从文件加载 CLAUDE.md，文件不存在则跳过
func (cm *ContextManager) LoadProjectContext(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	cm.setSlot("project_context", strings.TrimSpace(string(data)), 3)
}

// LoadMemory 从文件加载 memory.md，文件不存在则跳过
func (cm *ContextManager) LoadMemory() {
	data, err := os.ReadFile(cm.memoryPath)
	if err != nil {
		return
	}
	cm.setSlot("memory", strings.TrimSpace(string(data)), 5)
}

// LoadSkills 从 SkillRegistry 加载技能索引
func (cm *ContextManager) LoadSkills() {
	if cm.skillReg == nil {
		return
	}
	index := cm.skillReg.DescribeAvailable()
	if index == "" {
		return
	}
	cm.setSlot("skills", strings.TrimSpace(index), 2)
}

// setSlot 设置或更新一个 slot
func (cm *ContextManager) setSlot(tag, content string, order int) {
	for i := range cm.slots {
		if cm.slots[i].Tag == tag {
			cm.slots[i].Content = content
			cm.slots[i].Order = order
			return
		}
	}
	cm.slots = append(cm.slots, Slot{Tag: tag, Content: content, Order: order})
}

// --- 历史消息处理 ---

// BuildHistory 处理历史消息，超预算时压缩
func (cm *ContextManager) BuildHistory(
	ctx context.Context,
	history []*schema.Message,
	chatModel model.BaseChatModel,
	sessionID, conversationID string,
) []*schema.Message {
	if len(history) == 0 {
		return history
	}

	// 计算固定开销
	fixedCost := cm.estimateFixedCost()
	availableBudget := cm.budget - fixedCost
	if availableBudget <= 0 {
		availableBudget = cm.budget / 2 // 极端情况，给一半预算
	}

	historyTokens := contexttool.EstimateTokens(history)
	usagePercent := float64(historyTokens) / float64(availableBudget) * 100

	log.Printf("ContextManager: 历史消息 %d tokens, 可用预算 %d tokens (%.1f%%)", historyTokens, availableBudget, usagePercent)

	// 低于 60% 不压缩
	if usagePercent < 60 {
		return history
	}

	// 60%-85% 微压缩
	if usagePercent < 85 {
		log.Printf("ContextManager: 触发 MicroCompact")
		return contexttool.MicroCompactFunc(history)
	}

	// 85%+ 完整压缩
	log.Printf("ContextManager: 触发 CompactFunc")
	return contexttool.CompactFunc(ctx, history, chatModel, sessionID, conversationID)
}

// estimateFixedCost 估算固定 slot 的 token 开销
func (cm *ContextManager) estimateFixedCost() int64 {
	var total int64
	for _, s := range cm.slots {
		total += int64(len(s.Content) / 3) // 粗估：3 字符 ≈ 1 token
	}
	// 加上 XML 标签开销，每个 slot 约 20 tokens
	total += int64(len(cm.slots) * 20)
	return total
}

// --- 最终组装 ---

// Build 组装最终的 system message，用 XML 标签包裹每个 slot
func (cm *ContextManager) Build() string {
	if len(cm.slots) == 0 {
		return ""
	}

	// 按 Order 排序
	sorted := make([]Slot, len(cm.slots))
	copy(sorted, cm.slots)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Order < sorted[j].Order
	})

	var parts []string
	for _, s := range sorted {
		if strings.TrimSpace(s.Content) == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("<%s>\n%s\n</%s>", s.Tag, s.Content, s.Tag))
	}
	return strings.Join(parts, "\n\n")
}

// BuildMessages 组装最终发给 LLM 的消息数组
func (cm *ContextManager) BuildMessages(
	history []*schema.Message,
	userMsg string,
) []*schema.Message {
	systemMsg := cm.Build()
	messages := make([]*schema.Message, 0, 1+len(history)+1)
	if systemMsg != "" {
		messages = append(messages, schema.SystemMessage(systemMsg))
	}
	messages = append(messages, history...)
	messages = append(messages, schema.UserMessage(userMsg))
	return messages
}
