package prompt

import (
	"sort"
	"strings"
)

// PromptBlock 是 system prompt 的一个结构化片段。
type PromptBlock struct {
	Name    string // 标识名，用于 Upsert 去重
	Content string // 文本内容
	Order   int    // 排序，越小越靠前
}

// PromptBuilder 按 Order 组装 PromptBlock 为最终 system prompt 字符串。
type PromptBuilder struct {
	blocks []PromptBlock
}

func NewBuilder() *PromptBuilder {
	return &PromptBuilder{}
}

// Add 追加一个 block。同名 block 不会去重，允许多个。
func (b *PromptBuilder) Add(block PromptBlock) {
	b.blocks = append(b.blocks, block)
}

// Upsert 设置指定 name 的 block。若已存在同名则覆盖内容和顺序，否则追加。
func (b *PromptBuilder) Upsert(name, content string, order int) {
	for i := range b.blocks {
		if b.blocks[i].Name == name {
			b.blocks[i].Content = content
			b.blocks[i].Order = order
			return
		}
	}
	b.Add(PromptBlock{Name: name, Content: content, Order: order})
}

// Build 按 Order 升序拼接所有 block，用双换行分隔。跳过空内容 block。
func (b *PromptBuilder) Build() string {
	if len(b.blocks) == 0 {
		return ""
	}
	sorted := make([]PromptBlock, len(b.blocks))
	copy(sorted, b.blocks)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Order < sorted[j].Order
	})

	var parts []string
	for _, blk := range sorted {
		if strings.TrimSpace(blk.Content) != "" {
			parts = append(parts, blk.Content)
		}
	}
	return strings.Join(parts, "\n\n")
}
