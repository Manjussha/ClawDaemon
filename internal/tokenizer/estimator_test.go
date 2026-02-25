package tokenizer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEstimateTokens(t *testing.T) {
	assert.Equal(t, 0, EstimateTokens(""))
	assert.Equal(t, 1, EstimateTokens("test"))
	assert.Equal(t, 15, EstimateTokens("The quick brown fox jumps over the lazy dog. This is a test."))
}

func TestEstimateTaskCost(t *testing.T) {
	est := EstimateTaskCost("context text", "task prompt")
	assert.Greater(t, est.InputTokens, 0)
	assert.Greater(t, est.OutputTokens, 0)
	assert.Equal(t, est.TotalTokens, est.InputTokens+est.OutputTokens)
}
