// Package platform provides OS-aware helpers for paths, browsers, and services.
// All code that needs to behave differently per OS must use this package.
// Never use runtime.GOOS checks scattered across the codebase — put them here.
package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// GOOS returns the current operating system.
// Values: "linux", "darwin", "windows"
func GOOS() string {
	return runtime.GOOS
}

// IsWindows returns true when running on Windows.
func IsWindows() bool { return runtime.GOOS == "windows" }

// IsMac returns true when running on macOS.
func IsMac() bool { return runtime.GOOS == "darwin" }

// IsLinux returns true when running on Linux.
func IsLinux() bool { return runtime.GOOS == "linux" }

// DefaultWorkDir returns the OS-appropriate data directory for ClawDaemon.
//
//	Linux:   ~/.local/share/clawdaemon
//	macOS:   ~/Library/Application Support/ClawDaemon
//	Windows: %APPDATA%\ClawDaemon
//
// If WORK_DIR env var is set, that takes priority (used in Docker).
func DefaultWorkDir() string {
	if env := os.Getenv("WORK_DIR"); env != "" {
		return env
	}
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			home, _ := os.UserHomeDir()
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appData, "ClawDaemon")
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "ClawDaemon")
	default: // linux
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "share", "clawdaemon")
	}
}

// DataPath returns a path inside the work directory.
// Uses filepath.Join so it is correct on all platforms.
//
// Example: DataPath("screenshots", "project1") → ~/.local/share/clawdaemon/screenshots/project1
func DataPath(parts ...string) string {
	base := DefaultWorkDir()
	return filepath.Join(append([]string{base}, parts...)...)
}

// EnsureDir creates a directory and all parents if they don't exist.
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// BinaryName appends .exe on Windows, returns name as-is on other platforms.
func BinaryName(name string) string {
	if IsWindows() {
		return name + ".exe"
	}
	return name
}

// LookupCLI finds a CLI tool by name in PATH.
// On Windows it also tries the .exe variant automatically.
func LookupCLI(name string) (string, bool) {
	path, err := exec.LookPath(name)
	if err == nil {
		return path, true
	}
	if IsWindows() {
		path, err = exec.LookPath(name + ".exe")
		if err == nil {
			return path, true
		}
	}
	return "", false
}

// BrowserConfig holds browser configuration for the current platform.
type BrowserConfig struct {
	Primary      string // "lightpanda" or "chrome"
	PrimaryPath  string // path to primary browser binary
	FallbackPath string // path to chrome-headless-shell
	CDPPort      int    // Chrome DevTools Protocol port
}

// DefaultBrowserConfig returns the correct browser config for the current OS.
// Lightpanda is Linux-only (beta). Windows and Mac use chrome-headless-shell.
func DefaultBrowserConfig() BrowserConfig {
	cfg := BrowserConfig{
		CDPPort: 9222,
	}

	switch runtime.GOOS {
	case "linux":
		cfg.Primary = "lightpanda"
		cfg.PrimaryPath = "lightpanda" // via Docker or PATH
		cfg.FallbackPath = findChrome()
	default: // windows, darwin
		cfg.Primary = "chrome"
		cfg.PrimaryPath = findChrome()
		cfg.FallbackPath = findChrome() // same, no lightpanda available
	}

	return cfg
}

// ListeningPort represents one TCP/UDP port currently in LISTEN state.
type ListeningPort struct {
	Proto   string `json:"proto"`
	Port    int    `json:"port"`
	PID     int    `json:"pid"`
	Process string `json:"process"` // best-effort process name
}

// GetListeningPorts returns all ports currently in LISTEN state on this machine.
// Uses netstat on Windows, ss (with netstat fallback) on Linux/Mac.
func GetListeningPorts() []ListeningPort {
	if IsWindows() {
		return listeningWindows()
	}
	return listeningUnix()
}

// listeningWindows parses `netstat -ano` output and `tasklist` for process names.
func listeningWindows() []ListeningPort {
	out, err := exec.Command("netstat", "-ano").Output()
	if err != nil {
		return nil
	}
	// Build PID→process map from tasklist.
	pidNames := windowsPIDMap()

	seen := map[int]bool{}
	var ports []ListeningPort
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		// e.g. "TCP    0.0.0.0:8080    0.0.0.0:0    LISTENING    1234"
		if !strings.Contains(line, "LISTENING") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		proto := strings.ToLower(fields[0])
		addr := fields[1] // "0.0.0.0:8080" or "[::]:8080"
		pidStr := fields[len(fields)-1]

		// Extract port from address.
		port := parsePortFromAddr(addr)
		if port == 0 || seen[port] {
			continue
		}
		seen[port] = true

		pid, _ := strconv.Atoi(pidStr)
		name := pidNames[pid]
		ports = append(ports, ListeningPort{Proto: proto, Port: port, PID: pid, Process: name})
	}
	return ports
}

// windowsPIDMap runs tasklist and returns a pid→processName map.
func windowsPIDMap() map[int]string {
	out, err := exec.Command("tasklist", "/FO", "CSV", "/NH").Output()
	if err != nil {
		return nil
	}
	m := map[int]string{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		// CSV: "svchost.exe","1052","Services","0","12,432 K"
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}
		name := strings.Trim(parts[0], `"`)
		pidStr := strings.Trim(parts[1], `"`)
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}
		// Strip .exe suffix for cleaner display.
		name = strings.TrimSuffix(name, ".exe")
		m[pid] = name
	}
	return m
}

// listeningUnix parses `ss -tlnp` output on Linux/Mac, falling back to netstat.
func listeningUnix() []ListeningPort {
	// Try ss first (modern Linux).
	if out, err := exec.Command("ss", "-tlnp").Output(); err == nil {
		return parseSS(string(out))
	}
	// Fallback: netstat -tlnp
	out, err := exec.Command("netstat", "-tlnp").Output()
	if err != nil {
		return nil
	}
	return parseNetstatUnix(string(out))
}

// parseSS parses `ss -tlnp` output.
// Example line: LISTEN 0 128 0.0.0.0:22 0.0.0.0:* users:(("sshd",pid=1234,fd=3))
func parseSS(output string) []ListeningPort {
	seen := map[int]bool{}
	var ports []ListeningPort
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, "LISTEN") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		port := parsePortFromAddr(fields[4])
		if port == 0 || seen[port] {
			continue
		}
		seen[port] = true
		name := ""
		if len(fields) >= 6 {
			name = extractSSProcess(fields[5])
		}
		ports = append(ports, ListeningPort{Proto: "tcp", Port: port, Process: name})
	}
	return ports
}

// parseNetstatUnix parses `netstat -tlnp` on Linux/Mac.
func parseNetstatUnix(output string) []ListeningPort {
	seen := map[int]bool{}
	var ports []ListeningPort
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		// tcp  0  0  0.0.0.0:22  0.0.0.0:*  LISTEN  1234/sshd
		if len(fields) < 4 {
			continue
		}
		proto := strings.ToLower(fields[0])
		if !strings.HasPrefix(proto, "tcp") {
			continue
		}
		port := parsePortFromAddr(fields[3])
		if port == 0 || seen[port] {
			continue
		}
		seen[port] = true
		name := ""
		if len(fields) >= 7 {
			parts := strings.SplitN(fields[6], "/", 2)
			if len(parts) == 2 {
				name = parts[1]
			}
		}
		ports = append(ports, ListeningPort{Proto: proto, Port: port, Process: name})
	}
	return ports
}

// parsePortFromAddr extracts the port number from "addr:port" or "[ipv6]:port".
func parsePortFromAddr(addr string) int {
	// Handle IPv6: [::1]:8080
	if idx := strings.LastIndex(addr, ":"); idx >= 0 {
		portStr := addr[idx+1:]
		p, err := strconv.Atoi(portStr)
		if err == nil && p > 0 {
			return p
		}
	}
	return 0
}

// extractSSProcess pulls process name from ss users string like users:(("sshd",pid=1234,fd=3))
func extractSSProcess(s string) string {
	// Find the first quoted name after "users:(("
	if i := strings.Index(s, `""`); i >= 0 {
		s = s[i+2:]
	} else if i := strings.Index(s, `("`); i >= 0 {
		s = s[i+2:]
	}
	if i := strings.Index(s, `"`); i >= 0 {
		return s[:i]
	}
	return ""
}

// KillPort kills the process listening on the given port.
// It uses GetListeningPorts() to find the PID, then os.FindProcess + Kill().
// Works cross-platform — no shell commands needed.
func KillPort(port int) error {
	for _, lp := range GetListeningPorts() {
		if lp.Port == port && lp.PID > 0 {
			proc, err := os.FindProcess(lp.PID)
			if err != nil {
				return fmt.Errorf("platform.KillPort: find process %d: %w", lp.PID, err)
			}
			if err := proc.Kill(); err != nil {
				return fmt.Errorf("platform.KillPort: kill process %d: %w", lp.PID, err)
			}
			return nil
		}
	}
	return fmt.Errorf("platform.KillPort: no process found on port %d", port)
}

// Restart re-executes the current binary with the same arguments.
// It starts the new process then exits the current one.
// On all platforms this results in a clean restart with a fresh process.
func Restart() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("platform.Restart: executable: %w", err)
	}
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("platform.Restart: start: %w", err)
	}
	os.Exit(0)
	return nil // unreachable
}

// Ensure fmt is used (for future format calls).
var _ = fmt.Sprintf

// findChrome returns the path to Google Chrome or chromium on the current OS.
func findChrome() string {
	switch runtime.GOOS {
	case "windows":
		candidates := []string{
			filepath.Join(os.Getenv("PROGRAMFILES"), "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(os.Getenv("PROGRAMFILES(X86)"), "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Google", "Chrome", "Application", "chrome.exe"),
		}
		for _, p := range candidates {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	case "darwin":
		candidates := []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
		for _, p := range candidates {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	default: // linux
		candidates := []string{
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/chromium-browser",
			"/usr/bin/chromium",
		}
		for _, p := range candidates {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	// Last resort: rely on PATH
	if p, ok := LookupCLI("google-chrome"); ok {
		return p
	}
	if p, ok := LookupCLI("chromium"); ok {
		return p
	}
	return "chrome" // will fail with a clear error if not found
}

// ServiceConfig holds OS service configuration.
type ServiceConfig struct {
	Name        string
	DisplayName string
	Description string
	ExecPath    string
	WorkDir     string
}

// ServiceManager returns the correct service manager name for the current OS.
//
//	Linux:   "systemd"
//	macOS:   "launchd"
//	Windows: "windows-service"
func ServiceManager() string {
	switch runtime.GOOS {
	case "darwin":
		return "launchd"
	case "windows":
		return "windows-service"
	default:
		return "systemd"
	}
}

// InstallServiceFile generates the service definition file for the current OS.
// Returns the file path and content to write.
func InstallServiceFile(cfg ServiceConfig) (path string, content string) {
	switch runtime.GOOS {
	case "linux":
		path = filepath.Join("/etc", "systemd", "system", cfg.Name+".service")
		content = systemdUnit(cfg)
	case "darwin":
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, "Library", "LaunchAgents", "com."+cfg.Name+".plist")
		content = launchdPlist(cfg)
	case "windows":
		// Windows services are registered via sc.exe, not a file
		path = ""
		content = ""
	}
	return
}

func systemdUnit(cfg ServiceConfig) string {
	return `[Unit]
Description=` + cfg.Description + `
After=network.target

[Service]
Type=simple
ExecStart=` + cfg.ExecPath + `
WorkingDirectory=` + cfg.WorkDir + `
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`
}

func launchdPlist(cfg ServiceConfig) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.` + cfg.Name + `</string>
    <key>ProgramArguments</key>
    <array>
        <string>` + cfg.ExecPath + `</string>
    </array>
    <key>WorkingDirectory</key>
    <string>` + cfg.WorkDir + `</string>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
</dict>
</plist>
`
}
