package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/yourusername/clawdaemon/internal/platform"
)

// BrowserAdapter implements Worker for browser-based automation via Playwright.
type BrowserAdapter struct {
	browserCfg platform.BrowserConfig
}

// NewBrowserAdapter creates a BrowserAdapter with the platform-default browser config.
func NewBrowserAdapter() *BrowserAdapter {
	return &BrowserAdapter{
		browserCfg: platform.DefaultBrowserConfig(),
	}
}

func (a *BrowserAdapter) Name() string    { return "Browser (Playwright)" }
func (a *BrowserAdapter) CLIType() string { return "browser" }
func (a *BrowserAdapter) Command() string { return "node" }

func (a *BrowserAdapter) LimitPattern() []string {
	return []string{
		"timeout",
		"connection refused",
		"net::ERR_",
	}
}

// HealthCheck verifies that node is available.
func (a *BrowserAdapter) HealthCheck(ctx context.Context) error {
	if err := versionCheck(ctx, "node"); err != nil {
		return fmt.Errorf("browser.HealthCheck: node not found: %w", err)
	}
	return nil
}

// RunScript writes a Playwright JS script to a temp file and executes it via node.
// Never uses exec.Command("bash", ...) — always exec.Command("node", scriptPath).
func (a *BrowserAdapter) RunScript(ctx context.Context, script string) ([]byte, error) {
	tmp, err := os.CreateTemp("", "clawdaemon-browser-*.js")
	if err != nil {
		return nil, fmt.Errorf("browser.RunScript: create temp: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString(script); err != nil {
		tmp.Close()
		return nil, fmt.Errorf("browser.RunScript: write script: %w", err)
	}
	tmp.Close()

	scriptPath := filepath.Clean(tmp.Name())
	cmd := exec.CommandContext(ctx, "node", scriptPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("browser.RunScript: node exec: %w", err)
	}
	return out, nil
}
