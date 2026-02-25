// Package wizard provides the interactive terminal setup wizard for ClawDaemon.
// Invoke with: clawdaemon setup | clawdaemon --setup | clawdaemon -setup
package wizard

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/Manjussha/clawdaemon/internal/platform"
)

// stdinReader is shared across all prompts. term.ReadPassword bypasses it via raw fd.
var stdinReader = bufio.NewReader(os.Stdin)

// wizardConfig holds all values collected during the wizard.
type wizardConfig struct {
	Port           string
	AdminUsername  string
	AdminPassword  string
	WorkDir        string
	DefaultCLI     string
	DefaultModel   string
	TelegramToken  string
	TelegramChatID string
}

// cliSpec describes a supported CLI tool and its known models.
type cliSpec struct {
	Name        string
	DisplayName string
	Models      []modelSpec
}

// modelSpec describes a single model option.
type modelSpec struct {
	ID          string
	Description string
	Recommended bool
}

// knownCLIs is the built-in registry of supported AI CLI tools and models.
var knownCLIs = []cliSpec{
	{
		Name:        "claude",
		DisplayName: "Claude Code (Anthropic)",
		Models: []modelSpec{
			{ID: "claude-opus-4-6", Description: "most powerful"},
			{ID: "claude-sonnet-4-6", Description: "balanced", Recommended: true},
			{ID: "claude-haiku-4-5", Description: "fastest / cheapest"},
		},
	},
	{
		Name:        "gemini",
		DisplayName: "Gemini CLI (Google)",
		Models: []modelSpec{
			{ID: "gemini-2.0-flash", Description: "fast", Recommended: true},
			{ID: "gemini-1.5-pro", Description: "powerful"},
			{ID: "gemini-1.5-flash", Description: "efficient"},
		},
	},
}

// ── Entry point ───────────────────────────────────────────────────────────────

// Run executes the 6-step interactive setup wizard.
// On success it writes .env to the current working directory.
func Run(version string) error {
	printBanner(version)

	cfg := &wizardConfig{}
	var err error

	if cfg.Port, err = stepPort(); err != nil {
		return fmt.Errorf("wizard: port: %w", err)
	}
	if cfg.AdminUsername, cfg.AdminPassword, err = stepAdmin(); err != nil {
		return fmt.Errorf("wizard: admin: %w", err)
	}
	if cfg.WorkDir, err = stepWorkDir(); err != nil {
		return fmt.Errorf("wizard: workdir: %w", err)
	}
	if cfg.DefaultCLI, cfg.DefaultModel, err = stepCLI(); err != nil {
		return fmt.Errorf("wizard: cli: %w", err)
	}
	if cfg.TelegramToken, cfg.TelegramChatID, err = stepTelegram(); err != nil {
		return fmt.Errorf("wizard: telegram: %w", err)
	}
	if !stepConfirm(cfg) {
		fmt.Println("\n  Cancelled — no changes made.")
		return nil
	}
	if err := writeEnv(cfg); err != nil {
		return fmt.Errorf("wizard: writeEnv: %w", err)
	}

	fmt.Println()
	fmt.Println("  " + c("\033[32m", "✓") + " .env saved — run clawdaemon to start.")
	PrintDashboardURLs(cfg.Port)
	return nil
}

// ── Banner ────────────────────────────────────────────────────────────────────

func printBanner(version string) {
	const width = 56
	fmt.Println()
	fmt.Println(c("\033[36m", "╔"+strings.Repeat("═", width)+"╗"))
	bannerLine("", width)
	bannerLine("  ClawDaemon "+version, width)
	bannerLine("  Multi-CLI AI Agent Orchestrator", width)
	bannerLine("", width)
	fmt.Println(c("\033[36m", "╚"+strings.Repeat("═", width)+"╝"))
	fmt.Println()
	fmt.Println("  Welcome! Let's get you set up in 6 steps.")
	fmt.Println("  Press Enter to accept defaults, Ctrl+C to cancel.")
}

func bannerLine(text string, width int) {
	pad := width - len(text)
	if pad < 0 {
		pad = 0
	}
	fmt.Println(c("\033[36m", "║") + text + strings.Repeat(" ", pad) + c("\033[36m", "║"))
}

// ── Step 1: Port ──────────────────────────────────────────────────────────────

func stepPort() (string, error) {
	for {
		fmt.Println()
		fmt.Println(c("\033[33m", "━━━  1 / 6  —  PORT  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))
		fmt.Println()
		fmt.Println("  Scanning ports on this machine...")
		fmt.Println()

		candidates := []int{80, 443, 3000, 8080, 8081, 8082, 9000, 9090}

		// Build port→ListeningPort map.
		portMap := make(map[int]platform.ListeningPort)
		for _, lp := range platform.GetListeningPorts() {
			portMap[lp.Port] = lp
		}

		// Default: 8080 if free, else first free candidate.
		defaultPort := "8080"
		if _, busy := portMap[8080]; busy {
			for _, p := range candidates {
				if _, busy := portMap[p]; !busy {
					defaultPort = strconv.Itoa(p)
					break
				}
			}
		}

		// Print table.
		fmt.Printf("  %-6s  %-10s  %-18s\n", "PORT", "STATUS", "PROCESS")
		fmt.Printf("  %-6s  %-10s  %-18s\n", "──────", "──────────", "──────────────────")
		for _, p := range candidates {
			lp, busy := portMap[p]
			var statusStr, procStr, marker string
			if busy {
				statusStr = c("\033[31m", "● in use") + "  "
				procStr = lp.Process
				if procStr == "" {
					procStr = "unknown"
				}
			} else {
				statusStr = c("\033[32m", "○ free") + "    "
				procStr = "—"
			}
			if strconv.Itoa(p) == defaultPort && !busy {
				marker = "  " + c("\033[33m", "← default")
			}
			fmt.Printf("  %6d  %s  %-18s%s\n", p, statusStr, procStr, marker)
		}

		fmt.Println()
		portStr := prompt(fmt.Sprintf("Listen port [%s]", defaultPort), defaultPort)
		portNum, err := strconv.Atoi(strings.TrimSpace(portStr))
		if err != nil || portNum < 1 || portNum > 65535 {
			fmt.Println("  " + c("\033[31m", "✗") + " Invalid port — enter a number 1–65535.")
			continue
		}

		// If chosen port is busy, offer to kill the process.
		if lp, busy := portMap[portNum]; busy {
			proc := lp.Process
			if proc == "" {
				proc = "unknown"
			}
			fmt.Printf("\n  "+c("\033[31m", "✗")+" Port %d is in use by: %s (PID %d)\n",
				portNum, proc, lp.PID)
			ans := prompt("Kill it and use this port? [Y/n]", "Y")
			if strings.ToUpper(strings.TrimSpace(ans)) == "N" {
				continue // re-show table
			}
			fmt.Printf("  Killing PID %d... ", lp.PID)
			if err := platform.KillPort(portNum); err != nil {
				fmt.Println(c("\033[31m", "failed"))
				fmt.Println("  " + err.Error())
				fmt.Println("  Kill it manually or choose a different port.")
				continue
			}
			time.Sleep(500 * time.Millisecond)
			fmt.Println(c("\033[32m", "done"))
		}

		return strconv.Itoa(portNum), nil
	}
}

// ── Step 2: Admin ─────────────────────────────────────────────────────────────

func stepAdmin() (username, password string, err error) {
	fmt.Println()
	fmt.Println(c("\033[33m", "━━━  2 / 6  —  ADMIN ACCOUNT  ━━━━━━━━━━━━━━━━━━━"))
	fmt.Println()

	username = prompt("Username [admin]", "admin")

	for {
		fmt.Print("  Password: ")
		rawPass, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return "", "", fmt.Errorf("ReadPassword: %w", err)
		}

		fmt.Print("  Confirm:  ")
		rawConfirm, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return "", "", fmt.Errorf("ReadPassword confirm: %w", err)
		}

		if string(rawPass) != string(rawConfirm) {
			fmt.Println("  " + c("\033[31m", "✗") + " Passwords do not match — try again.")
			continue
		}
		password = string(rawPass)
		if password == "" {
			password = "admin"
		}
		return username, password, nil
	}
}

// ── Step 3: Work directory ────────────────────────────────────────────────────

func stepWorkDir() (string, error) {
	fmt.Println()
	fmt.Println(c("\033[33m", "━━━  3 / 6  —  WORK DIRECTORY  ━━━━━━━━━━━━━━━━━━"))
	fmt.Println()

	defaultDir := platform.DefaultWorkDir()
	fmt.Printf("  Recommended for your OS:\n  %s\n\n", c("\033[36m", defaultDir))

	dir := prompt(fmt.Sprintf("Path [%s]", defaultDir), defaultDir)
	return filepath.Clean(dir), nil
}

// ── Step 4: CLI & Model ───────────────────────────────────────────────────────

func stepCLI() (cliName, model string, err error) {
	fmt.Println()
	fmt.Println(c("\033[33m", "━━━  4 / 6  —  LLM CLI  ━━━━━━━━━━━━━━━━━━━━━━━━━"))
	fmt.Println()
	fmt.Println("  Scanning for installed CLI tools...")
	fmt.Println()

	type entry struct {
		spec cliSpec
		path string
		ok   bool
	}
	var entries []entry
	for _, spec := range knownCLIs {
		path, ok := platform.LookupCLI(spec.Name)
		entries = append(entries, entry{spec, path, ok})
		tick := c("\033[32m", "✓")
		info := c("\033[90m", path)
		if !ok {
			tick = c("\033[31m", "✗")
			info = c("\033[90m", "not found — install it first")
		}
		fmt.Printf("  %s  %-10s %s\n", tick, spec.Name, info)
	}
	fmt.Println()

	// Numbered list.
	fmt.Println("  Select default CLI:")
	for i, e := range entries {
		tag := c("\033[90m", "[not found]")
		if e.ok {
			tag = c("\033[32m", "[installed]")
		}
		fmt.Printf("  %d.  %-10s %-30s %s\n", i+1, e.spec.Name, e.spec.DisplayName, tag)
	}
	skipIdx := len(entries) + 1
	fmt.Printf("  %d.  Skip — configure workers manually later\n", skipIdx)
	fmt.Println()

	// Default to first installed CLI.
	defaultIdx := skipIdx
	for i, e := range entries {
		if e.ok {
			defaultIdx = i + 1
			break
		}
	}

	sel := promptInt(fmt.Sprintf("Select CLI [%d]", defaultIdx), 1, skipIdx, defaultIdx)
	if sel == skipIdx {
		fmt.Println("  " + c("\033[90m", "Skipped — you can set DEFAULT_CLI in .env later."))
		return "", "", nil
	}

	chosen := entries[sel-1]
	cliName = chosen.spec.Name

	// Model selection.
	fmt.Println()
	fmt.Printf("  Default model for %s:\n", c("\033[36m", cliName))
	fmt.Println("  " + strings.Repeat("─", 52))

	models := chosen.spec.Models
	defaultModelIdx := 1
	for i, m := range models {
		rec := ""
		if m.Recommended {
			rec = "  " + c("\033[33m", "← recommended")
			defaultModelIdx = i + 1
		}
		fmt.Printf("  %d.  %-32s %s%s\n", i+1, m.ID, c("\033[90m", m.Description), rec)
	}
	customIdx := len(models) + 1
	fmt.Printf("  %d.  Enter custom model name...\n", customIdx)
	fmt.Println()

	modelSel := promptInt(fmt.Sprintf("Select model [%d]", defaultModelIdx), 1, customIdx, defaultModelIdx)
	if modelSel == customIdx {
		model = prompt("Custom model ID", models[defaultModelIdx-1].ID)
	} else {
		model = models[modelSel-1].ID
	}

	fmt.Printf("\n  %s  %s / %s\n", c("\033[32m", "✓"), cliName, model)
	return cliName, model, nil
}

// ── Step 5: Telegram ──────────────────────────────────────────────────────────

func stepTelegram() (token, chatID string, err error) {
	fmt.Println()
	fmt.Println(c("\033[33m", "━━━  5 / 6  —  TELEGRAM  (Enter to skip)  ━━━━━━━"))
	fmt.Println()
	fmt.Println("  Create a bot at https://t.me/BotFather, then paste the token.")
	fmt.Println()

	token = prompt("Bot Token (Enter to skip)", "")
	if token == "" {
		fmt.Println("  " + c("\033[90m", "Skipped — set TELEGRAM_TOKEN in .env later."))
		return "", "", nil
	}

	// Validate token.
	fmt.Print("  Verifying token...")
	botUsername, botName, err := telegramGetMe(token)
	if err != nil {
		fmt.Println()
		fmt.Println("  " + c("\033[31m", "✗") + " Token error: " + err.Error())
		fmt.Println("  " + c("\033[90m", "Saved anyway — fix TELEGRAM_TOKEN in .env later."))
		return token, "", nil
	}
	fmt.Printf("\r  %s Bot: @%s (%s)%s\n",
		c("\033[32m", "✓"), botUsername, botName, strings.Repeat(" ", 20))

	// Auto-capture chat ID.
	fmt.Println()
	fmt.Printf("  Open Telegram and send any message to @%s\n", c("\033[36m", botUsername))
	fmt.Println("  Waiting up to 3 minutes... (press Enter to skip)")
	fmt.Println()

	type pollResult struct {
		id        int64
		firstName string
	}
	pollCh := make(chan pollResult, 1)

	// Poll goroutine — long-polls Telegram getUpdates.
	go func() {
		id, name, err := telegramPollChatID(token, 3*time.Minute)
		if err == nil && id != 0 {
			pollCh <- pollResult{id: id, firstName: name}
		}
	}()

	// Skip goroutine — reads directly from os.Stdin (not the shared bufio reader)
	// so it cannot steal bytes meant for later prompts.
	skipCh := make(chan struct{}, 1)
	go func() {
		buf := make([]byte, 1)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				return
			}
			if n > 0 && (buf[0] == '\n' || buf[0] == '\r') {
				skipCh <- struct{}{}
				return
			}
		}
	}()

	var paired pollResult
	select {
	case paired = <-pollCh:
		// fall through to success path
	case <-skipCh:
		fmt.Println("  " + c("\033[90m", "Skipped — set TELEGRAM_CHAT_ID in .env or Settings later."))
		return token, "", nil
	case <-time.After(3 * time.Minute):
		fmt.Println("  " + c("\033[90m", "Timed out — set TELEGRAM_CHAT_ID in .env or Settings later."))
		return token, "", nil
	}

	chatID = strconv.FormatInt(paired.id, 10)
	fmt.Printf("  %s Paired with %s  (Chat ID: %s)\n",
		c("\033[32m", "✓"), paired.firstName, chatID)

	// Send a confirmation message to the paired user immediately.
	_ = telegramSendMessage(token, paired.id, fmt.Sprintf(
		"🐾 *ClawDaemon paired\\!*\n\nHi %s\\! The setup wizard has connected successfully\\.\n\nChat ID: `%s`\n\n*What\\'s next:*\n• Finish the setup wizard\n• Run `clawdaemon` to start\n• Send /status to check daemon health",
		telegramEscape(paired.firstName), chatID,
	))

	return token, chatID, nil
}

// ── Step 6: Confirm ───────────────────────────────────────────────────────────

func stepConfirm(cfg *wizardConfig) bool {
	fmt.Println()
	fmt.Println(c("\033[33m", "━━━  6 / 6  —  CONFIRM  ━━━━━━━━━━━━━━━━━━━━━━━━━"))
	fmt.Println()

	rows := [][2]string{
		{"PORT", cfg.Port},
		{"ADMIN", cfg.AdminUsername},
		{"WORK DIR", cfg.WorkDir},
		{"CLI", dash(cfg.DefaultCLI)},
		{"MODEL", dash(cfg.DefaultModel)},
		{"TELEGRAM", dash(cfg.TelegramToken)},
		{"CHAT ID", dash(cfg.TelegramChatID)},
	}
	for _, r := range rows {
		fmt.Printf("  %-12s %s\n", r[0], r[1])
	}
	fmt.Println()

	ans := prompt("Save to .env? [Y/n]", "Y")
	ans = strings.TrimSpace(strings.ToUpper(ans))
	return ans == "" || ans == "Y" || ans == "YES"
}

func dash(s string) string {
	if s == "" {
		return c("\033[90m", "—")
	}
	return s
}

// ── Write .env ────────────────────────────────────────────────────────────────

func writeEnv(cfg *wizardConfig) error {
	jwtSecret, err := randomHex(32)
	if err != nil {
		return fmt.Errorf("writeEnv: %w", err)
	}
	lines := []string{
		"PORT=" + cfg.Port,
		"WORK_DIR=" + cfg.WorkDir,
		"DB_PATH=" + filepath.Join(cfg.WorkDir, "clawdaemon.db"),
		"CHARACTER_DIR=" + filepath.Join(cfg.WorkDir, "character"),
		"SCREENSHOTS_DIR=" + filepath.Join(cfg.WorkDir, "screenshots"),
		"ADMIN_USERNAME=" + cfg.AdminUsername,
		"ADMIN_PASSWORD=" + cfg.AdminPassword,
		"DEFAULT_CLI=" + cfg.DefaultCLI,
		"DEFAULT_MODEL=" + cfg.DefaultModel,
		"TELEGRAM_TOKEN=" + cfg.TelegramToken,
		"TELEGRAM_CHAT_ID=" + cfg.TelegramChatID,
		"DUCKDNS_TOKEN=",
		"DUCKDNS_DOMAIN=",
		"JWT_SECRET=" + jwtSecret,
		"SESSION_EXPIRY_HOURS=24",
		"BRUTE_FORCE_MAX_ATTEMPTS=5",
		"BRUTE_FORCE_BLOCK_MINUTES=15",
	}
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(".env", []byte(content), 0600); err != nil {
		return fmt.Errorf("writeEnv WriteFile: %w", err)
	}
	return nil
}

// ── Dashboard URLs ────────────────────────────────────────────────────────────

// PrintDashboardURLs prints LAN IPs + localhost. Called by main.go on every start.
func PrintDashboardURLs(port string) {
	var ips []string
	if ifaces, err := net.Interfaces(); err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, _ := iface.Addrs()
			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				if ip4 := ip.To4(); ip4 != nil && !ip4.IsLoopback() {
					ips = append(ips, ip4.String())
				}
			}
		}
	}

	var urls []string
	for _, ip := range ips {
		urls = append(urls, fmt.Sprintf("http://%s:%s", ip, port))
	}
	urls = append(urls, fmt.Sprintf("http://localhost:%s", port))

	fmt.Println()
	fmt.Printf("  Dashboard → %s\n", urls[0])
	for _, u := range urls[1:] {
		fmt.Printf("              %s\n", u)
	}
	fmt.Println()
}

// ── Telegram API ──────────────────────────────────────────────────────────────

type tgEnvelope struct {
	OK          bool            `json:"ok"`
	Description string          `json:"description"`
	Result      json.RawMessage `json:"result"`
}

func telegramGetMe(token string) (username, firstName string, err error) {
	resp, err := http.Get("https://api.telegram.org/bot" + token + "/getMe")
	if err != nil {
		return "", "", fmt.Errorf("getMe: %w", err)
	}
	defer resp.Body.Close()

	var env tgEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return "", "", fmt.Errorf("getMe decode: %w", err)
	}
	if !env.OK {
		return "", "", fmt.Errorf("%s", env.Description)
	}
	var bot struct {
		Username  string `json:"username"`
		FirstName string `json:"first_name"`
	}
	if err := json.Unmarshal(env.Result, &bot); err != nil {
		return "", "", fmt.Errorf("getMe parse: %w", err)
	}
	return bot.Username, bot.FirstName, nil
}

func telegramPollChatID(token string, timeout time.Duration) (chatID int64, firstName string, err error) {
	client := &http.Client{Timeout: 35 * time.Second}
	offset := 0
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		url := fmt.Sprintf(
			"https://api.telegram.org/bot%s/getUpdates?offset=%d&timeout=25&limit=1",
			token, offset,
		)
		resp, err := client.Get(url)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var result struct {
			OK     bool `json:"ok"`
			Result []struct {
				UpdateID int `json:"update_id"`
				Message  struct {
					From struct {
						ID        int64  `json:"id"`
						FirstName string `json:"first_name"`
					} `json:"from"`
					Chat struct {
						ID int64 `json:"id"`
					} `json:"chat"`
				} `json:"message"`
			} `json:"result"`
		}
		if err := json.Unmarshal(body, &result); err != nil || !result.OK {
			time.Sleep(2 * time.Second)
			continue
		}
		if len(result.Result) == 0 {
			continue
		}
		upd := result.Result[0]
		offset = upd.UpdateID + 1
		return upd.Message.Chat.ID, upd.Message.From.FirstName, nil
	}
	return 0, "", fmt.Errorf("timeout")
}

// telegramSendMessage sends a MarkdownV2 message to a chat via the Bot API.
func telegramSendMessage(token string, chatID int64, text string) error {
	payload, err := json.Marshal(map[string]any{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "MarkdownV2",
	})
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(
		"https://api.telegram.org/bot"+token+"/sendMessage",
		"application/json",
		bytes.NewReader(payload),
	)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// telegramEscape escapes special characters for MarkdownV2.
func telegramEscape(s string) string {
	special := `\_*[]()~` + "`" + `>#+-=|{}.!`
	var b strings.Builder
	for _, ch := range s {
		if strings.ContainsRune(special, ch) {
			b.WriteRune('\\')
		}
		b.WriteRune(ch)
	}
	return b.String()
}

// ── Input helpers ─────────────────────────────────────────────────────────────

func prompt(label, defaultVal string) string {
	fmt.Printf("  %s: ", label)
	line, _ := stdinReader.ReadString('\n')
	line = strings.TrimRight(line, "\r\n")
	if strings.TrimSpace(line) == "" {
		return defaultVal
	}
	return line
}

func promptInt(label string, min, max, defaultVal int) int {
	for {
		s := prompt(label, strconv.Itoa(defaultVal))
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err == nil && n >= min && n <= max {
			return n
		}
		fmt.Printf("  Enter a number between %d and %d.\n", min, max)
	}
}

func supportsColor() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

func c(ansi, text string) string {
	if !supportsColor() {
		return text
	}
	return ansi + text + "\033[0m"
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("randomHex: %w", err)
	}
	return hex.EncodeToString(b), nil
}
