package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yourusername/clawdaemon/internal/platform"
)

// SetupStatus handles GET /api/setup/status
// Returns whether a .env file already exists.
func (h *Handler) SetupStatus(w http.ResponseWriter, r *http.Request) {
	_, err := os.Stat(".env")
	ok(w, map[string]interface{}{
		"configured": err == nil,
		"work_dir":   h.config.WorkDir,
		"port":       h.config.Port,
	})
}

type portResult struct {
	Port      int    `json:"port"`
	Available bool   `json:"available"`
	Note      string `json:"note,omitempty"`
}

// ScanPorts handles GET /api/setup/ports
// Probes common ports and reports which are available.
func (h *Handler) ScanPorts(w http.ResponseWriter, r *http.Request) {
	candidates := []struct {
		port int
		note string
	}{
		{80, "HTTP standard"},
		{443, "HTTPS standard"},
		{3000, "Dev default"},
		{4000, ""},
		{5000, ""},
		{7000, ""},
		{8000, "Common alt"},
		{8080, "ClawDaemon default"},
		{8081, ""},
		{8082, ""},
		{8443, "HTTPS alt"},
		{9000, ""},
		{9090, ""},
		{9443, ""},
	}

	results := make([]portResult, 0, len(candidates))
	for _, c := range candidates {
		available := isPortFree(c.port)
		results = append(results, portResult{
			Port:      c.port,
			Available: available,
			Note:      c.note,
		})
	}
	ok(w, results)
}

// ListeningPorts handles GET /api/setup/listening
// Returns all ports currently in LISTEN state on this machine, with process names.
func (h *Handler) ListeningPorts(w http.ResponseWriter, r *http.Request) {
	ports := platform.GetListeningPorts()
	// Sort by port number ascending.
	sort.Slice(ports, func(i, j int) bool { return ports[i].Port < ports[j].Port })
	ok(w, ports)
}

// SaveSetup handles POST /api/setup/save
// Writes a .env file with the provided configuration.
func (h *Handler) SaveSetup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Port               string `json:"port"`
		AdminUsername      string `json:"admin_username"`
		AdminPassword      string `json:"admin_password"`
		WorkDir            string `json:"work_dir"`
		TelegramToken      string `json:"telegram_token"`
		TelegramChatID     string `json:"telegram_chat_id"`
		SessionExpiryHours string `json:"session_expiry_hours"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Port == "" {
		req.Port = "8080"
	}
	if req.AdminUsername == "" {
		req.AdminUsername = "admin"
	}
	if req.AdminPassword == "" {
		fail(w, http.StatusBadRequest, "admin_password is required")
		return
	}
	if req.WorkDir == "" {
		req.WorkDir = h.config.WorkDir
	}
	if req.SessionExpiryHours == "" {
		req.SessionExpiryHours = "24"
	}

	// Generate a random JWT secret if not provided.
	jwtSecret := generateSimpleSecret()

	var sb strings.Builder
	sb.WriteString("# ClawDaemon configuration\n")
	sb.WriteString(fmt.Sprintf("# Generated: %s\n\n", time.Now().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("PORT=%s\n", req.Port))
	sb.WriteString(fmt.Sprintf("WORK_DIR=%s\n", req.WorkDir))
	sb.WriteString(fmt.Sprintf("DB_PATH=%s\n", filepath.Join(req.WorkDir, "clawdaemon.db")))
	sb.WriteString(fmt.Sprintf("CHARACTER_DIR=%s\n", filepath.Join(req.WorkDir, "character")))
	sb.WriteString(fmt.Sprintf("SCREENSHOTS_DIR=%s\n", filepath.Join(req.WorkDir, "screenshots")))
	sb.WriteString(fmt.Sprintf("ADMIN_USERNAME=%s\n", req.AdminUsername))
	sb.WriteString(fmt.Sprintf("ADMIN_PASSWORD=%s\n", req.AdminPassword))
	sb.WriteString(fmt.Sprintf("JWT_SECRET=%s\n", jwtSecret))
	sb.WriteString(fmt.Sprintf("SESSION_EXPIRY_HOURS=%s\n", req.SessionExpiryHours))
	sb.WriteString("BRUTE_FORCE_MAX_ATTEMPTS=5\n")
	sb.WriteString("BRUTE_FORCE_BLOCK_MINUTES=15\n")
	if req.TelegramToken != "" {
		sb.WriteString(fmt.Sprintf("TELEGRAM_TOKEN=%s\n", req.TelegramToken))
	}
	if req.TelegramChatID != "" {
		sb.WriteString(fmt.Sprintf("TELEGRAM_CHAT_ID=%s\n", req.TelegramChatID))
	}

	if err := os.WriteFile(".env", []byte(sb.String()), 0600); err != nil {
		fail(w, http.StatusInternalServerError, "write .env: "+err.Error())
		return
	}
	ok(w, map[string]string{
		"message":  "Configuration saved to .env — restart ClawDaemon to apply.",
		"env_path": ".env",
	})
}

// isPortFree returns true if the TCP port can be bound (i.e. nothing is using it).
func isPortFree(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

// generateSimpleSecret returns a cryptographically random 32-char hex string.
func generateSimpleSecret() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback: time-based (unlikely to happen)
		t := time.Now().UnixNano()
		for i := range b {
			b[i] = byte(t >> uint(i*4))
		}
	}
	return hex.EncodeToString(b)
}
