// Package tokenizer provides token estimation, context optimization, and budget governance.
package tokenizer

// TokenEstimate holds token cost projections for a task.
type TokenEstimate struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

// EstimateTokens estimates the token count of a text string.
// Uses the rule of thumb: ~4 characters per token.
func EstimateTokens(text string) int {
	if len(text) == 0 {
		return 0
	}
	return (len(text) + 3) / 4
}

// EstimateTaskCost estimates the token cost for a task given context and prompt.
// Output is estimated at ~60% of the total input tokens.
func EstimateTaskCost(context, prompt string) TokenEstimate {
	input := EstimateTokens(context) + EstimateTokens(prompt)
	output := int(float64(input) * 0.6)
	return TokenEstimate{
		InputTokens:  input,
		OutputTokens: output,
		TotalTokens:  input + output,
	}
}
