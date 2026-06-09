package contexttool

import "testing"

func TestConfigureContextStateUpdatesMaxTokens(t *testing.T) {
	state := NewContextState(DefaultMaxTokens)
	state.ConfigureMaxTokens(1000)
	state.SetTokens(250)

	usage := state.GetUsage()
	if usage.MaxTokens != 1000 {
		t.Fatalf("MaxTokens = %d, want 1000", usage.MaxTokens)
	}
	if usage.Percentage != 25 {
		t.Fatalf("Percentage = %.1f, want 25.0", usage.Percentage)
	}
}
