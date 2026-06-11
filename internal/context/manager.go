package context

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

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
	slots            []Slot
	soulPath         string
	agentPath        string // AGENT.md 路径
	memoryPath       string
	systemPromptPath string // system_prompt.md 路径
	aiOpsPromptPath  string // ai_ops.md 路径
	skillReg         *skills.SkillRegistry
	budget           int64
}

// New 创建 ContextManager
func New(budget int64, skillReg *skills.SkillRegistry) *ContextManager {
	return &ContextManager{
		soulPath:         filepath.Join("data", "soul.md"),
		agentPath:        "AGENT.md",
		memoryPath:       filepath.Join("data", "memory.md"),
		systemPromptPath: filepath.Join("internal", "prompt", "system_prompt.md"),
		aiOpsPromptPath:  filepath.Join("internal", "prompt", "ai_ops.md"),
		skillReg:         skillReg,
		budget:           budget,
	}
}



// --- Slot 注入方法 ---

// SetMode 加载 global_rules：system_prompt.md 始终加载，aiops 模式额外追加 ai_ops.md
func (cm *ContextManager) SetMode(mode string) {
	// 始终加载 system_prompt.md
	data, err := os.ReadFile(cm.systemPromptPath)
	if err != nil {
		log.Printf("ContextManager: 读取 system_prompt.md 失败: %v", err)
		return
	}
	content := strings.TrimSpace(string(data))

	// aiops 模式追加诊断指令
	if mode == "aiops" {
		aiOpsData, err := os.ReadFile(cm.aiOpsPromptPath)
		if err != nil {
			log.Printf("ContextManager: 读取 ai_ops.md 失败: %v", err)
		} else {
			content += "\n\n" + strings.TrimSpace(string(aiOpsData))
		}
	}

	cm.setSlot("global_rules", content, 0)
}




// LoadSystemInfo 注入当前系统环境信息
func (cm *ContextManager) LoadSystemInfo() {
	info := fmt.Sprintf("OS: %s\nArch: %s", runtime.GOOS, runtime.GOARCH)
	switch runtime.GOOS {
	case "windows":
		info += "\nShell: cmd/PowerShell\n注意: Windows 系统，使用 dir/cls/type 等 Windows 命令，不要使用 ls/clear/cat 等 Linux 命令。"
	case "linux":
		info += "\nShell: bash\n注意: Linux 系统，使用 ls/cat/grep 等标准命令。"
	case "darwin":
		info += "\nShell: zsh/bash\n注意: macOS 系统，使用 ls/cat/grep 等标准命令。"
	}
	cm.setSlot("system_info", info, 0)
}

// LoadSoul 从文件加载 SOUL.md，文件不存在则跳过
func (cm *ContextManager) LoadSoul() {
	data, err := os.ReadFile(cm.soulPath)
	if err != nil {
		return
	}
	cm.setSlot("soul", strings.TrimSpace(string(data)), 1)
}

// LoadProjectContext 从文件加载 AGENT.md，文件不存在则跳过
func (cm *ContextManager) LoadProjectContext() {
	data, err := os.ReadFile(cm.agentPath)
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
