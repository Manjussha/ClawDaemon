// Package cli manages CLI worker adapters (Claude Code, Gemini, browser).
package cli

import (
	"context"
	"fmt"
	"os/exec"
)

// Worker defines the interface every CLI adapter must implement.
type Worker interface {
	// Name returns the human-readable name of this CLI.
	Name() string
	// CLIType returns the type identifier: "claude", "gemini", or "browser".
	CLIType() string
	// Command returns the executable name (without args).
	Command() string
	// HealthCheck runs a quick check that the CLI is reachable.
	HealthCheck(ctx context.Context) error
	// LimitPattern returns the rate-limit detection keyword list for this CLI.
	LimitPattern() []string
}

// Registry holds registered CLI adapters.
type Registry struct {
	workers map[string]Worker
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{workers: make(map[string]Worker)}
}

// Register adds a CLI adapter.
func (r *Registry) Register(w Worker) {
	r.workers[w.CLIType()] = w
}

// Get returns a Worker by CLI type.
func (r *Registry) Get(cliType string) (Worker, bool) {
	w, ok := r.workers[cliType]
	return w, ok
}

// List returns all registered CLI types.
func (r *Registry) List() []string {
	types := make([]string, 0, len(r.workers))
	for t := range r.workers {
		types = append(types, t)
	}
	return types
}

// HealthCheck runs the adapter's health check.
func (r *Registry) HealthCheck(ctx context.Context, cliType string) error {
	w, ok := r.workers[cliType]
	if !ok {
		return fmt.Errorf("cli.Registry.HealthCheck: unknown cli type %q", cliType)
	}
	return w.HealthCheck(ctx)
}

// versionCheck runs `command --version` to verify the CLI is in PATH.
func versionCheck(ctx context.Context, command string) error {
	cmd := exec.CommandContext(ctx, command, "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cli.versionCheck: %s --version: %w (output: %s)", command, err, out)
	}
	return nil
}

// DefaultRegistry creates a registry with Claude, Gemini, and Browser adapters.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(NewClaudeAdapter())
	r.Register(NewGeminiAdapter())
	r.Register(NewBrowserAdapter())
	return r
}
