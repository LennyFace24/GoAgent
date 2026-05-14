package contexttool

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

const (
	CoarseThreshold         = 1_500_000
	MicroCompactThreshold   = 400_000
	CompactThreshold        = 800_000
	KeepRecentTurns         = 6
	KeepRecentMicroCompact  = 10
	TranscriptDir           = "data/transcripts"
)

// ---------- compact 工具定义 ----------

type CompactInput struct{}

// CompactTrigger 共享触发器，service 层通过检查 Triggered 判断是否需要压缩
type CompactTrigger struct {
	triggered atomic.Bool
}

func (t *CompactTrigger) IsTriggered() bool {
	return t.triggered.Swap(false)
}

// NewCompactTool 返回工具和触发器引用
// 工具注册到 ToolHandler，触发器传给 service 层
func NewCompactTool() (tool.InvokableTool, *CompactTrigger, error) {
	trigger := &CompactTrigger{}

	tool, err := utils.InferTool(
		"compact",
		"主动触发上下文压缩。当对话过长、影响回答质量时调用。系统会保留最近的对话，将早期对话压缩为摘要。",
		func(ctx context.Context, input CompactInput) (string, error) {
			trigger.triggered.Store(true)
			return "上下文压缩已触发，系统将在下一轮自动压缩历史消息。", nil
		},
	)
	if err != nil {
		return nil, nil, err
	}
	return tool, trigger, nil
}

// ---------- 压缩核心逻辑 ----------

// MicroCompactFunc Lever 1: 旧工具结果替换为 [expired]
// 保留最近 KeepRecentMicroCompact 轮的工具结果，更早的替换为占位符
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
			messages[i].Content = "[expired]"
			compressed++
		}
	}

	if compressed > 0 {
		log.Printf("MicroCompactFunc: 压缩了 %d 条工具结果", compressed)
	}
	return messages
}

// CompactFunc Lever 2: old 消息总结为摘要，service 层调用
func CompactFunc(ctx context.Context, messages []*schema.Message, chatModel model.BaseChatModel, sessionID, conversationID string) []*schema.Message {
	if len(messages) <= 1 {
		return messages
	}

	persistTranscript(messages, sessionID, conversationID)

	systemMsg := messages[0]
	rest := messages[1:]

	cutIdx := len(rest)
	count := 0
	for i := len(rest) - 1; i >= 0; i-- {
		if rest[i].Role == schema.User {
			count++
		}
		if count >= KeepRecentTurns {
			cutIdx = i
			break
		}
	}

	if cutIdx <= 2 {
		return messages
	}

	oldMessages := rest[:cutIdx]
	recentMessages := rest[cutIdx:]

	summary, err := summarizeOldMessages(ctx, oldMessages, chatModel)
	if err != nil {
		log.Printf("CompactFunc: 总结失败，跳过压缩: %v", err)
		return messages
	}

	result := messages[:0]
	result = append(result, systemMsg)
	result = append(result, schema.UserMessage("[系统] 以下是之前对话的压缩摘要：\n"+summary))
	result = append(result, schema.AssistantMessage("好的，我已了解之前的对话内容，请继续。", nil))
	result = append(result, recentMessages...)

	log.Printf("CompactFunc: 压缩完成，消息数 %d -> %d", len(messages), len(result))
	return result
}

func summarizeOldMessages(ctx context.Context, messages []*schema.Message, chatModel model.BaseChatModel) (string, error) {
	var content string
	for _, m := range messages {
		switch m.Role {
		case schema.User:
			content += "用户: " + m.Content + "\n"
		case schema.Assistant:
			if m.Content != "" {
				content += "助手: " + m.Content + "\n"
			}
			for _, tc := range m.ToolCalls {
				content += fmt.Sprintf("助手调用工具: %s(%s)\n", tc.Function.Name, tc.Function.Arguments)
			}
		case schema.Tool:
			content += fmt.Sprintf("工具[%s]: %s\n", m.ToolName, truncate(m.Content, 500))
		case schema.System:
			content += "系统: " + m.Content + "\n"
		}
	}

	if len(content) > 100000 {
		content = content[:100000] + "\n...(截断)"
	}

	summarizePrompt := []*schema.Message{
		schema.SystemMessage("你是一个对话摘要助手。请将以下对话历史压缩为简洁的摘要，保留关键信息、决策、工具调用结果和未完成的任务。摘要应控制在3000字以内。"),
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
