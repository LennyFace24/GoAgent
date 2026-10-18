package contexttool

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

//go:embed compact_rule.md
var compactRule string

const (
	KeepRecentTurns        = 6
	KeepRecentMicroCompact = 6
	TranscriptDir          = "data/transcripts"

	// HistoryIDExtraKey 由 store.LoadHistory 写入每条历史消息的稳定 ID。
	HistoryIDExtraKey = "history_id"
)

// CompactionResult 描述一次完整压缩的落盘信息。
type CompactionResult struct {
	Summary      string
	FirstKeptID  string
	KeptMessages []*schema.Message
}

// ---------- compact 工具定义 ----------

type CompactInput struct{}

type CompactTrigger struct {
	triggered atomic.Bool
}

// 全局共享触发器：compact 工具与 /compact 命令共用同一实例
var globalTrigger = &CompactTrigger{}

// GetCompactTrigger 返回全局压缩触发器
func GetCompactTrigger() *CompactTrigger {
	return globalTrigger
}

// Trigger 手动触发压缩标记（/compact 命令使用）
func (t *CompactTrigger) Trigger() {
	t.triggered.Store(true)
}

func (t *CompactTrigger) IsTriggered() bool {
	return t.triggered.Swap(false)
}

// NewCompactTool 返回工具和触发器引用
// 工具注册到 ToolHandler，触发器传给 service 层
func NewCompactTool() (tool.InvokableTool, *CompactTrigger, error) {
	tool, err := utils.InferTool(
		"compact",
		"主动触发上下文压缩。当对话过长、影响回答质量时调用。系统会保留最近的对话，将早期对话压缩为摘要。",
		func(ctx context.Context, input CompactInput) (string, error) {
			globalTrigger.Trigger()
			return "上下文压缩已触发，系统将在下一轮自动压缩历史消息。", nil
		},
	)
	if err != nil {
		return nil, nil, err
	}
	return tool, globalTrigger, nil
}

// ---------- 压缩核心逻辑 ----------
// MicroCompactFunc Lever 1: 旧工具结果压缩为摘要
// 保留最近 KeepRecentMicroCompact 轮的工具结果，更早的替换为 "[compressed] toolName: 概要"
func MicroCompactFunc(messages []*schema.Message) []*schema.Message {
	if len(messages) <= 1 {
		return messages
	}

	// 从后往前找到保留边界：最近 N 个 user 消息之后的内容全保留
	cutIdx := len(messages)
	count := 0
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == schema.User {
			count++
		}
		if count >= KeepRecentMicroCompact {
			cutIdx = i
			break
		}
	}

	compressed := 0
	for i := 1; i < cutIdx; i++ {
		if messages[i].Role == schema.Tool && messages[i].Content != "" {
			toolName := messages[i].ToolName
			if toolName == "" {
				toolName = "unknown"
			}
			hint := extractHint(messages[i].Content, 80)
			messages[i].Content = fmt.Sprintf("[compressed] %s: %s", toolName, hint)
			compressed++
		}
	}

	if compressed > 0 {
		log.Printf("MicroCompactFunc: 压缩了 %d 条工具结果", compressed)
	}
	return messages
}

// extractHint 从工具结果中提取简短摘要
func extractHint(content string, maxLen int) string {
	// 取第一行作为摘要
	if idx := strings.Index(content, "\n"); idx > 0 {
		content = content[:idx]
	}
	content = strings.TrimSpace(content)
	if len(content) > maxLen {
		return content[:maxLen] + "..."
	}
	if content == "" {
		return "(empty result)"
	}
	return content
}

// CompactFunc Lever 2: 将早于最近 KeepRecentTurns 轮的消息总结为摘要。
// 返回压缩后的消息和需要落盘的边界；调用方可据此持久化，避免下次重算。
func CompactFunc(ctx context.Context, messages []*schema.Message, chatModel model.BaseChatModel, sessionID, conversationID string) ([]*schema.Message, *CompactionResult) {
	if len(messages) <= 1 {
		return messages, nil
	}

	systemMsg := messages[0]
	rest := messages[1:]

	// 从后往前定位最近 KeepRecentTurns 轮；不足时 cutIdx 保持 0，不压缩。
	// 边界必须是某个 user 消息，绝不能把当前用户输入一起摘要掉。
	cutIdx := 0
	count := 0
	for i := len(rest) - 1; i >= 0; i-- {
		if rest[i].Role == schema.User {
			count++
			if count >= KeepRecentTurns {
				cutIdx = i
				break
			}
		}
	}
	if cutIdx <= 0 {
		return messages, nil
	}

	oldMessages := rest[:cutIdx]
	recentMessages := rest[cutIdx:]

	summary, err := summarizeOldMessages(ctx, oldMessages, chatModel)
	if err != nil {
		log.Printf("CompactFunc: 总结失败，跳过压缩: %v", err)
		return messages, nil
	}
	persistTranscript(messages, sessionID, conversationID)

	result := make([]*schema.Message, 0, len(recentMessages)+2)
	result = append(result, systemMsg, schema.UserMessage(summary))
	result = append(result, recentMessages...)

	// 只落盘带原始 ID 的历史消息；当前轮新消息由 handler 统一保存，避免重复。
	kept := make([]*schema.Message, 0, len(recentMessages))
	for _, msg := range recentMessages {
		if _, ok := msg.Extra[HistoryIDExtraKey]; ok {
			kept = append(kept, msg)
		}
	}
	firstKeptID := ""
	if len(kept) > 0 {
		firstKeptID, _ = kept[0].Extra[HistoryIDExtraKey].(string)
	}
	if len(kept) == 0 {
		// 没有可识别的原始消息（例如测试构造的内存历史），只做本次内存压缩。
		return result, nil
	}

	log.Printf("CompactFunc: 压缩完成，消息数 %d -> %d，保留原始消息 %d", len(messages), len(result), len(kept))
	return result, &CompactionResult{
		Summary:      summary,
		FirstKeptID:  firstKeptID,
		KeptMessages: kept,
	}
}

func summarizeOldMessages(ctx context.Context, messages []*schema.Message, chatModel model.BaseChatModel) (string, error) {
	var content string
	for _, m := range messages {
		switch m.Role {
		case schema.User:
			content += "[User]" + m.Content + "\n"
		case schema.Assistant:
			if m.Content != "" {
				content += "[Assistant] " + m.Content + "\n"
			}
			for _, tc := range m.ToolCalls {
				content += fmt.Sprintf("[toolCall] : %s(%s)\n", tc.Function.Name, tc.Function.Arguments)
			}
		case schema.Tool:
			content += fmt.Sprintf("[tool] : %s(%s)\n", m.ToolName, truncate(m.Content, 500))
		case schema.System:
			content += "[System] : " + m.Content + "\n"
		}
	}

	if len(content) > 100000 {
		content = content[:100000] + "\n...(截断)"
	}
	data := strings.TrimSpace(compactRule)
	if data == "" {
		return "", fmt.Errorf("压缩规则为空")
	}

	summarizePrompt := []*schema.Message{
		schema.SystemMessage(data),
		schema.UserMessage(content),
	}

	resp, err := chatModel.Generate(ctx, summarizePrompt)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// 持久化压缩前的对话记录，便于后续分析和调试
func persistTranscript(messages []*schema.Message, sessionID, conversationID string) {
	dir := filepath.Join(TranscriptDir, sessionID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("persistTranscript: 创建目录失败: %v", err)
		return
	}

	filename := fmt.Sprintf("%s_%s_%s.jsonl", conversationID, time.Now().Format("20060102_150405"), "pre_compact")
	path := filepath.Join(dir, filename)

	f, err := os.Create(path)
	if err != nil {
		log.Printf("persistTranscript: 创建文件失败: %v", err)
		return
	}
	defer f.Close()

	for _, m := range messages {
		line := map[string]any{
			"role":    m.Role,
			"content": m.Content,
		}
		if len(m.ToolCalls) > 0 {
			line["tool_calls"] = m.ToolCalls
		}
		if m.ToolCallID != "" {
			line["tool_call_id"] = m.ToolCallID
		}
		if m.ToolName != "" {
			line["tool_name"] = m.ToolName
		}
		data, _ := json.Marshal(line)
		f.Write(data)
		f.WriteString("\n")
	}
	log.Printf("persistTranscript: 转录已保存到 %s", path)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...(截断)"
}
