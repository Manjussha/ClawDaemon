package cli

import (
	"context"
	"fmt"
)

// ClaudeAdapter implements Worker for Claude Code CLI.
type ClaudeAdapter struct{}

// NewClaudeAdapter creates a ClaudeAdapter.
func NewClaudeAdapter() *ClaudeAdapter { return &ClaudeAdapter{} }

func (a *ClaudeAdapter) Name() string    { return "Claude Code" }
func (a *ClaudeAdapter) CLIType() string { return "claude" }
func (a *ClaudeAdapter) Command() string { return "claude" }

func (a *ClaudeAdapter) LimitPattern() []string {
	return []string{
		"rate limit",
		"rate_limit",
		"too many requests",
		"429",
		"overloaded",
	}
}

// HealthCheck verifies the claude binary is in PATH.
func (a *ClaudeAdapter) HealthCheck(ctx context.Context) error {
	if err := versionCheck(ctx, "claude"); err != nil {
		return fmt.Errorf("claude.HealthCheck: %w", err)
	}
	return nil
}

// DefaultArgs returns the CLI flags used when launching Claude.
func (a *ClaudeAdapter) DefaultArgs() []string {
	return []string{"--dangerously-skip-permissions"}
}
