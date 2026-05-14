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

// ShouldCompact 判断是否需要压缩，返回压缩级别
func ShouldCompact(messages []*schema.Message) CompactLevel {
	totalChars := 0
	for _, m := range messages {
		totalChars += len(m.Content)
		for _, tc := range m.ToolCalls {
			totalChars += len(tc.Function.Arguments)
		}
	}

	// 字符数粗判
	if totalChars < CoarseThreshold {
		return CompactNone
	}

	tokens := EstimateTokens(messages)
	if tokens > CompactThreshold {
		return CompactFull
	}
	if tokens > MicroCompactThreshold {
		return CompactMicro
	}
	return CompactNone
}

// EstimateTokens 使用 tiktoken 估算消息的 token 数
func EstimateTokens(messages []*schema.Message) int {
	if enc == nil {
		total := 0
		for _, m := range messages {
			total += len(m.Content) / 3
		}
		return total
	}

	total := 0
	for _, m := range messages {
		total += len(enc.Encode(m.Content, nil, nil))
		for _, tc := range m.ToolCalls {
			total += len(enc.Encode(tc.Function.Name, nil, nil))
			total += len(enc.Encode(tc.Function.Arguments, nil, nil))
		}
		total += 4
	}
	return total
}
