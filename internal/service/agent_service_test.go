package service

import (
	"errors"
	"testing"

	"github.com/cloudwego/eino/schema"
)

// sendStream 构造一个 StreamReader，依次发送给定消息后关闭。
func sendStream(msgs ...*schema.Message) *schema.StreamReader[*schema.Message] {
	sr, sw := schema.Pipe[*schema.Message](len(msgs) + 1)
	go func() {
		defer sw.Close()
		for _, m := range msgs {
			sw.Send(m, nil)
		}
	}()
	return sr
}

// sendStreamWithError 在发送给定消息后追加一个流错误，用于验证错误传播。
func sendStreamWithError(err error, msgs ...*schema.Message) *schema.StreamReader[*schema.Message] {
	sr, sw := schema.Pipe[*schema.Message](len(msgs) + 1)
	go func() {
		defer sw.Close()
		for _, m := range msgs {
			sw.Send(m, nil)
		}
		sw.Send(nil, err)
	}()
	return sr
}

func TestCollectTurnAggregatesReply(t *testing.T) {
	stream := sendStream(
		&schema.Message{Role: schema.Assistant, Content: "Hello"},
		&schema.Message{Role: schema.Assistant, Content: " world"},
	)
	ch := make(chan ToolEvent, 16)

	result := collectTurnWithMeta(stream, ch)

	if result.err != nil {
		t.Fatalf("unexpected stream error: %v", result.err)
	}
	if len(result.toolCalls) != 0 {
		t.Fatalf("toolCalls = %d, want 0", len(result.toolCalls))
	}
	if result.thinking != "" {
		t.Fatalf("thinking = %q, want empty", result.thinking)
	}
	if result.reply != "Hello world" {
		t.Fatalf("reply = %q, want %q", result.reply, "Hello world")
	}
}

func TestCollectTurnMergesSplitToolCall(t *testing.T) {
	// 模拟流式分片：第一次带工具名和部分参数，第二次补全参数
	stream := sendStream(
		&schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{{
				ID:   "t1",
				Type: "function",
				Function: schema.FunctionCall{
					Name:      "foo",
					Arguments: `{"a":`,
				},
			}},
		},
		&schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{{
				Function: schema.FunctionCall{
					Arguments: `1}`,
				},
			}},
		},
	)
	ch := make(chan ToolEvent, 16)

	result := collectTurnWithMeta(stream, ch)

	if len(result.toolCalls) != 1 {
		t.Fatalf("toolCalls = %d, want 1 (分片应合并为单个 tool_call)", len(result.toolCalls))
	}
	tc := result.toolCalls[0]
	if tc.ID != "t1" {
		t.Fatalf("ID = %q, want t1", tc.ID)
	}
	if tc.Function.Name != "foo" {
		t.Fatalf("Name = %q, want foo", tc.Function.Name)
	}
	if tc.Function.Arguments != `{"a":1}` {
		t.Fatalf("Arguments = %q, want %q", tc.Function.Arguments, `{"a":1}`)
	}
}

func TestCollectTurnEmitsDeltaAndThinkingEvents(t *testing.T) {
	stream := sendStream(
		&schema.Message{Role: schema.Assistant, ReasoningContent: "想"},
		&schema.Message{Role: schema.Assistant, ReasoningContent: "一下"},
		&schema.Message{Role: schema.Assistant, Content: "答"},
	)
	ch := make(chan ToolEvent, 16)

	result := collectTurnWithMeta(stream, ch)
	close(ch)

	if len(result.toolCalls) != 0 {
		t.Fatalf("toolCalls = %d, want 0", len(result.toolCalls))
	}
	if result.thinking != "想一下" {
		t.Fatalf("thinking = %q, want %q", result.thinking, "想一下")
	}
	if result.reply != "答" {
		t.Fatalf("reply = %q, want %q", result.reply, "答")
	}

	// 校验推送到 toolEvents 的事件类型与顺序
	var got []string
	for ev := range ch {
		got = append(got, ev.Type)
	}
	want := []string{"thinking_delta", "thinking_delta", "delta"}
	if len(got) != len(want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("events[%d] = %q, want %q (full=%v)", i, got[i], want[i], got)
		}
	}
}

func TestCollectTurnEmptyStream(t *testing.T) {
	stream := sendStream() // 立即关闭
	ch := make(chan ToolEvent, 4)

	result := collectTurnWithMeta(stream, ch)

	if result.err != nil || len(result.toolCalls) != 0 || result.thinking != "" || result.reply != "" {
		t.Fatalf("expected all empty, got err=%v toolCalls=%d thinking=%q reply=%q",
			result.err, len(result.toolCalls), result.thinking, result.reply)
	}
}

func TestCollectTurnRecordsFinishReason(t *testing.T) {
	stream := sendStream(&schema.Message{
		Role:         schema.Assistant,
		Content:      "partial",
		ResponseMeta: &schema.ResponseMeta{FinishReason: "length"},
	})
	ch := make(chan ToolEvent, 8)

	result := collectTurnWithMeta(stream, ch)

	if result.reply != "partial" || result.finishReason != "length" {
		t.Fatalf("finish reason lost: reply=%q reason=%q", result.reply, result.finishReason)
	}
}

func TestCollectTurnPropagatesStreamError(t *testing.T) {
	streamErr := errors.New("stream broke")
	stream := sendStreamWithError(streamErr, &schema.Message{Role: schema.Assistant, Content: "partial"})
	ch := make(chan ToolEvent, 8)

	result := collectTurnWithMeta(stream, ch)

	if !errors.Is(result.err, streamErr) {
		t.Fatalf("stream error not propagated: %v", result.err)
	}
	if result.reply != "partial" {
		t.Fatalf("partial content lost on stream error: %q", result.reply)
	}
}
