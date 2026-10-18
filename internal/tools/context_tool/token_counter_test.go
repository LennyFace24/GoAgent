package contexttool

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestEstimateTokensUsesOneEstimatorForMessageParts(t *testing.T) {
	message := &schema.Message{
		Role:    schema.Assistant,
		Content: "assistant content",
		ToolCalls: []schema.ToolCall{{
			Function: schema.FunctionCall{
				Name:      "read_file",
				Arguments: `{"path":"README.md"}`,
			},
		}},
	}

	want := EstimateTextTokens(message.Content) +
		EstimateTextTokens(message.ToolCalls[0].Function.Name) +
		EstimateTextTokens(message.ToolCalls[0].Function.Arguments) + messageOverheadTokens
	if got := EstimateTokens([]*schema.Message{message}); got != want {
		t.Fatalf("EstimateTokens = %d, want %d", got, want)
	}
}

func TestEstimateTokensCountsFinalSystemAndUserMessagesConsistently(t *testing.T) {
	system := schema.SystemMessage("<global_rules>system instructions</global_rules>")
	user := schema.UserMessage("current question")

	want := EstimateTextTokens(system.Content) + messageOverheadTokens +
		EstimateTextTokens(user.Content) + messageOverheadTokens
	if got := EstimateTokens([]*schema.Message{system, user}); got != want {
		t.Fatalf("EstimateTokens = %d, want %d", got, want)
	}
}
