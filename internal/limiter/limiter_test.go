package limiter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectLimit_Claude(t *testing.T) {
	d := New("claude")
	assert.True(t, d.DetectLimit("Error: rate limit exceeded, please try again"))
	assert.True(t, d.DetectLimit("429 Too Many Requests"))
	assert.False(t, d.DetectLimit("Task completed successfully"))
}

func TestDetectLimit_Gemini(t *testing.T) {
	d := New("gemini")
	assert.True(t, d.DetectLimit("quota exceeded for this project"))
	assert.False(t, d.DetectLimit("Response generated successfully"))
}

func TestErrRateLimit(t *testing.T) {
	err := &ErrRateLimit{Line: "rate limit hit"}
	assert.Contains(t, err.Error(), "rate limit detected")
}
