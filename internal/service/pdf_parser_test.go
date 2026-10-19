package service

import (
	"strings"
	"testing"
)

func TestSplitPDFTextCapsChunkSizeAndKeepsContent(t *testing.T) {
	text := strings.Repeat("第一段内容。\n", 500)
	chunks := splitPDFText(text, 300, 0)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	joined := strings.Join(chunks, "")
	for _, chunk := range chunks {
		if runes := []rune(chunk); len(runes) > 300 {
			t.Fatalf("chunk too large: %d runes", len(runes))
		}
		if strings.TrimSpace(chunk) == "" {
			t.Fatal("empty chunk")
		}
	}
	if strings.ReplaceAll(joined, "\n", "") != strings.ReplaceAll(strings.TrimSpace(text), "\n", "") {
		t.Fatal("chunking lost content")
	}
}

func TestSplitPDFTextEmpty(t *testing.T) {
	if chunks := splitPDFText(" \n\t ", 100, 0); len(chunks) != 0 {
		t.Fatalf("empty text produced %d chunks", len(chunks))
	}
}
