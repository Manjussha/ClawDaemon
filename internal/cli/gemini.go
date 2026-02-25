package cli

import (
	"context"
	"fmt"
)

// GeminiAdapter implements Worker for the Gemini CLI.
type GeminiAdapter struct{}

// NewGeminiAdapter creates a GeminiAdapter.
func NewGeminiAdapter() *GeminiAdapter { return &GeminiAdapter{} }

func (a *GeminiAdapter) Name() string    { return "Gemini CLI" }
func (a *GeminiAdapter) CLIType() string { return "gemini" }
func (a *GeminiAdapter) Command() string { return "gemini" }

func (a *GeminiAdapter) LimitPattern() []string {
	return []string{
		"quota exceeded",
		"rate limit",
		"429",
		"resource exhausted",
	}
}

// HealthCheck verifies the gemini binary is in PATH.
func (a *GeminiAdapter) HealthCheck(ctx context.Context) error {
	if err := versionCheck(ctx, "gemini"); err != nil {
		return fmt.Errorf("gemini.HealthCheck: %w", err)
	}
	return nil
}

// DefaultArgs returns the CLI flags used when launching Gemini.
func (a *GeminiAdapter) DefaultArgs() []string {
	return []string{}
}
