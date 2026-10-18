package contexttool

import (
	"log"

	"github.com/cloudwego/eino/schema"
	"github.com/pkoukk/tiktoken-go"
)

var enc *tiktoken.Tiktoken

func init() {
	var err error
	enc, err = tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		log.Printf("tiktoken 初始化失败，将使用字符估算: %v", err)
	}
}

// CompactLevel 压缩级别
type CompactLevel int

const (
	CompactNone  CompactLevel = iota // 不需要压缩
	CompactMicro                     // Lever 1: 旧工具结果替换为 [expired]
	CompactFull                      // Lever 2: old 消息总结为摘要
)

// messageOverheadTokens is the fixed per-message estimate used by every
// context-budget calculation in this package.
const messageOverheadTokens = 4

// ShouldCompact 根据传入的 ContextState 判断是否需要压缩。
// 阈值基于预算百分比。首次调用（或消息已变化）时重新估算。
func ShouldCompact(messages []*schema.Message, state *ContextState) CompactLevel {
	usage := state.GetUsage()

	// 估算当前消息集（幂等：每次调用都基于消息实际内容重算，
	// 避免 currentTokens==0 与"消息确实为空"混淆导致的不稳定压缩）
	if len(messages) > 0 {
		tokens := EstimateTokens(messages)
		state.SetTokens(int64(tokens))
		usage = state.GetUsage()
	}

	// 超过 85% 触发完整压缩（LLM 摘要）
	if usage.Percentage > 85 {
		return CompactFull
	}
	// 超过 60% 触发微压缩（旧工具结果替换为 [expired]）
	if usage.Percentage > 60 {
		return CompactMicro
	}
	return CompactNone
}

// EstimateTextTokens estimates one text value with the shared tokenizer and
// fallback. Callers must not use a second character-based estimate.
func EstimateTextTokens(text string) int {
	if text == "" {
		return 0
	}
	if enc == nil {
		return (len(text) + 2) / 3
	}
	return len(enc.Encode(text, nil, nil))
}

// EstimateTokens estimates the complete message representation used by the
// context budget checks, including tool-call names, arguments, and per-message
// protocol overhead.
func EstimateTokens(messages []*schema.Message) int {
	total := 0
	for _, m := range messages {
		if m == nil {
			continue
		}
		total += EstimateTextTokens(m.Content)
		for _, tc := range m.ToolCalls {
			total += EstimateTextTokens(tc.Function.Name)
			total += EstimateTextTokens(tc.Function.Arguments)
		}
		total += messageOverheadTokens
	}
	return total
}
