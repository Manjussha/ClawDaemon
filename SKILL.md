---
name: clawdaemon-dev
description: >
  Build ClawDaemon — an open-source multi-CLI AI agent orchestrator daemon in Go.
  Use this skill for ALL ClawDaemon development tasks including: writing Go packages,
  implementing the worker pool, building the REST API, creating the SQLite schema,
  setting up auth with Telegram 2FA, building the WebSocket hub, implementing token
  optimization, writing the browser worker with Lightpanda, creating the frontend
  dashboard, and configuring Docker/Nginx deployment. Trigger this skill whenever
  working on any file inside the clawdaemon project directory.
---

# ClawDaemon Development Skill

## First Step — Always
Before writing ANY code, read `CLAUDE.md` in the project root. It contains the
complete tech stack, database schema, business logic rules, and coding standards.
Do not skip this step. Every decision is documented there.

Then check `TODO.md` for the current task checklist. Work through items in order
unless told otherwise.

---

## Architecture Summary

ClawDaemon is a single Go binary with embedded frontend. Key layers:

```
HTTP Server (net/http)
├── /                    → embedded web dashboard (embed.FS)
├── /api/v1/...          → REST API (JSON)
├── /ws                  → WebSocket (live logs + status)
└── /static/...          → CSS, JS assets

Core Engine
├── WorkerPool           → goroutines per CLI worker
├── TaskQueue            → SQLite-backed, survives restart
├── CharacterSystem      → context injection before every task
├── TokenGovernor        → budget zones + context compression
├── RateLimiter          → detect CLI limits, checkpoint, alert
└── Scheduler            → cron jobs → task injection

Integrations
├── TelegramBot          → admin control + alerts
├── WebhookDispatcher    → fire events to external URLs
└── BrowserWorker        → Lightpanda + Playwright
```

---

## Package Implementation Guide

### internal/db/
```go
// db.go pattern
package db

import (
    "database/sql"
    _ "github.com/mattn/go-sqlite3"
)

type DB struct {
    *sql.DB
}

func New(path string) (*DB, error) {
    db, err := sql.Open("sqlite3", path+"?_journal=WAL&_foreign_keys=on")
    // always enable WAL and foreign keys
}
```

### internal/worker/worker.go
```go
// Worker runs as a goroutine. Key loop:
func (w *Worker) run(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        default:
            task := w.queue.Dequeue(w.ID)
            if task == nil {
                time.Sleep(2 * time.Second)
                continue
            }
            w.executeTask(ctx, task)
        }
    }
}

func (w *Worker) executeTask(ctx context.Context, task *Task) {
    // 1. Health check CLI
    // 2. Get budget zone → optimize context
    // 3. Build context via character injector
    // 4. Run CLI as subprocess, stream output line by line
    // 5. Detect rate limit in each line
    // 6. On completion → save output, fire webhooks, update token usage
}
```

### internal/character/injector.go
```go
// Context building — order matters, defined in CLAUDE.md
func (i *Injector) BuildContext(task Task, project Project, zone BudgetZone) string {
    var parts []string

    switch zone {
    case GREEN:
        parts = append(parts, i.loader.LoadIdentity())     // full
        parts = append(parts, i.loader.LoadThinking())     // full
    case YELLOW:
        parts = append(parts, i.compressSummary(i.loader.LoadIdentity(), 200))
        parts = append(parts, i.compressSummary(i.loader.LoadThinking(), 150))
    case ORANGE, RED:
        // skip identity and thinking entirely
    }

    parts = append(parts, i.loader.LoadRules())            // NEVER compressed
    parts = append(parts, i.relevantMemory(task, zone))    // filtered
    parts = append(parts, i.loader.AutoSelectSkill(task.Prompt)) // skill
    parts = append(parts, project.ClaudeMD)                // project context
    parts = append(parts, i.memory.LoadProjectMemory(project.ID)) // project memory
    
    if task.Checkpoint != "" {
        parts = append(parts, i.buildCheckpointContext(task, zone))
    }
    
    parts = append(parts, task.Prompt)                     // NEVER compressed
    return strings.Join(parts, "\n\n---\n\n")
}
```

### internal/tokenizer/estimator.go
```go
// Simple but effective token estimation
func EstimateTokens(text string) int {
    // Average: 1 token ≈ 4 characters for English code/text
    return len(text) / 4
}

func EstimateTaskCost(context, prompt string) TokenEstimate {
    inputTokens := EstimateTokens(context) + EstimateTokens(prompt)
    // Estimate output as 60% of input for code tasks
    outputTokens := int(float64(inputTokens) * 0.6)
    return TokenEstimate{
        Input:  inputTokens,
        Output: outputTokens,
        Total:  inputTokens + outputTokens,
    }
}
```

### internal/telegram/interactive.go
```go
// Rate limit alert with inline keyboard
func (b *Bot) SendLimitAlert(workerName, taskTitle string) {
    msg := tgbotapi.NewMessage(b.adminChatID, fmt.Sprintf(
        "⚠️ *%s hit rate limit*\nTask: _%s_\n\nWhat should I do?",
        workerName, taskTitle,
    ))
    msg.ParseMode = "Markdown"
    msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("🔄 Switch CLI", "limit:switch"),
            tgbotapi.NewInlineKeyboardButtonData("⏳ Wait", "limit:wait"),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("⏭ Skip Task", "limit:skip"),
            tgbotapi.NewInlineKeyboardButtonData("⏸ Pause All", "limit:pause"),
        ),
    )
    b.api.Send(msg)
}
```

### internal/api/api.go
```go
// Router setup — net/http only, no framework
func SetupRoutes(mux *http.ServeMux, h *Handlers) {
    // Auth (no auth middleware)
    mux.HandleFunc("POST /api/v1/auth/login", h.Login)
    mux.HandleFunc("POST /api/v1/auth/logout", h.Logout)
    mux.HandleFunc("POST /api/v1/auth/2fa/verify", h.Verify2FA)

    // Protected routes — wrap with RequireAuth middleware
    mux.Handle("GET /api/v1/status", auth.RequireAuth(h.GetStatus))
    mux.Handle("GET /api/v1/tasks", auth.RequireAuth(h.ListTasks))
    mux.Handle("POST /api/v1/tasks", auth.RequireAuth(h.CreateTask))
    // ... etc

    // WebSocket
    mux.HandleFunc("GET /ws", h.WebSocket)

    // Frontend — serve embedded files
    mux.Handle("/", http.FileServer(http.FS(web.Files)))
}
```

### web/assets/app.js
```javascript
// WebSocket client pattern
const ws = new WebSocket(`ws://${location.host}/ws`);

ws.onmessage = (event) => {
    const msg = JSON.parse(event.data);
    switch (msg.type) {
        case 'log_line':
            appendLogLine(msg.worker_id, msg.line, msg.level);
            break;
        case 'worker_status':
            updateWorkerCard(msg.worker_id, msg.status);
            break;
        case 'task_update':
            updateTaskRow(msg.task_id, msg.status);
            break;
        case 'system_status':
            updateDashboard(msg.data);
            break;
    }
};
```

---

## CLI Execution Pattern

```go
// Run CLI as subprocess, stream output
func (w *Worker) runCLI(ctx context.Context, task *Task, fullPrompt string) error {
    // Write prompt to temp file (avoid shell escaping issues)
    tmpFile, _ := os.CreateTemp("", "clawdaemon-*.txt")
    tmpFile.WriteString(fullPrompt)
    tmpFile.Close()
    defer os.Remove(tmpFile.Name())

    // Build command
    args := append(w.config.Args, "--prompt-file", tmpFile.Name())
    cmd := exec.CommandContext(ctx, w.config.Command, args...)
    cmd.Dir = task.Project.FolderPath

    // Stream stdout line by line
    stdout, _ := cmd.StdoutPipe()
    stderr, _ := cmd.StderrPipe()
    cmd.Start()

    scanner := bufio.NewScanner(stdout)
    for scanner.Scan() {
        line := scanner.Text()
        w.hub.BroadcastToWorker(w.ID, line)
        w.logger.LogLine(task.ID, w.ID, line)

        // Check for rate limit
        if w.limiter.DetectLimit(line, w.config.LimitPattern) {
            w.limiter.HandleLimit(w.ID, task.ID)
            return ErrRateLimit
        }

        // Save checkpoint every 60 seconds
        w.checkpointBuffer.WriteString(line + "\n")
    }

    return cmd.Wait()
}
```

---

## Browser Worker Pattern

```javascript
// Playwright script template written by agent
// Saved to /tmp/clawdaemon-browser-{taskID}.js
const { chromium } = require('playwright');

(async () => {
    // Connect to Lightpanda via CDP
    const browser = await chromium.connectOverCDP('http://localhost:9222');
    const page = await browser.newPage();

    try {
        // Agent writes task-specific automation here
        await page.goto('TARGET_URL');

        // Screenshot on each action
        await page.screenshot({ path: 'SCREENSHOT_PATH/before.png' });

        // ... task specific actions ...

        // Final result
        console.log('CLAWDAEMON_RESULT:' + JSON.stringify({
            success: true,
            screenshots: ['before.png', 'after.png'],
            testsPassed: 0,
            testsFailed: 0,
        }));
    } catch (err) {
        console.error('CLAWDAEMON_ERROR:' + err.message);
        // Trigger fallback to chrome-headless-shell
    } finally {
        await browser.close();
    }
})();
```

---

## Docker Setup

```dockerfile
# docker/Dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o clawdaemon .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates sqlite nodejs npm
WORKDIR /app
COPY --from=builder /app/clawdaemon .
RUN npm install -g playwright
EXPOSE 8080
CMD ["./clawdaemon"]
```

```yaml
# docker/docker-compose.yml
services:
  daemon:
    build: .
    volumes:
      - ./data:/data
      - /var/run/docker.sock:/var/run/docker.sock
    env_file: .env
    restart: unless-stopped
    ports:
      - "8080:8080"

  nginx:
    image: nginx:alpine
    volumes:
      - ./docker/nginx.conf:/etc/nginx/nginx.conf
      - ./certbot/conf:/etc/letsencrypt
    ports:
      - "80:80"
      - "443:443"
    depends_on:
      - daemon
    restart: unless-stopped

  lightpanda:
    image: lightpanda/browser:nightly
    ports:
      - "9222:9222"
    restart: unless-stopped
```

---

## JSON Response Format

All API responses must use this format:

```go
type Response struct {
    Success bool        `json:"success"`
    Data    interface{} `json:"data,omitempty"`
    Error   string      `json:"error,omitempty"`
    Meta    *Meta       `json:"meta,omitempty"` // for paginated responses
}

type Meta struct {
    Total  int `json:"total"`
    Page   int `json:"page"`
    Limit  int `json:"limit"`
}

func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(data)
}

func WriteError(w http.ResponseWriter, status int, err string) {
    WriteJSON(w, status, Response{Success: false, Error: err})
}
```

---

## WebSocket Message Format

```go
type WSMessage struct {
    Type      string      `json:"type"`
    WorkerID  int         `json:"worker_id,omitempty"`
    TaskID    int         `json:"task_id,omitempty"`
    Level     string      `json:"level,omitempty"`   // INFO|SUCCESS|WARN|ERROR
    Message   string      `json:"message,omitempty"`
    Data      interface{} `json:"data,omitempty"`
    Timestamp time.Time   `json:"timestamp"`
}

// Types: log_line, worker_status, task_update, task_complete, 
//        rate_limit, system_status, budget_warning
```

---

## Error Handling Pattern

```go
// Always wrap errors with context
func (q *Queue) Dequeue(workerID int) (*Task, error) {
    row := q.db.QueryRow(`
        SELECT id, title, prompt, project_id, checkpoint
        FROM tasks 
        WHERE status = 'pending'
        ORDER BY priority ASC, created_at ASC
        LIMIT 1
    `)
    
    var task Task
    if err := row.Scan(&task.ID, &task.Title, &task.Prompt, &task.ProjectID, &task.Checkpoint); err != nil {
        if err == sql.ErrNoRows {
            return nil, nil  // no tasks — not an error
        }
        return nil, fmt.Errorf("queue.Dequeue: %w", err)
    }
    
    return &task, nil
}
```

---

## Testing Pattern

```go
// Each package has _test.go
func TestQueue_EnqueueDequeue(t *testing.T) {
    db := setupTestDB(t)
    q := queue.New(db)
    
    task := &queue.Task{
        Title:  "Test task",
        Prompt: "Do something",
    }
    
    err := q.Enqueue(task)
    require.NoError(t, err)
    
    got, err := q.Dequeue(1)
    require.NoError(t, err)
    require.NotNil(t, got)
    assert.Equal(t, task.Title, got.Title)
}

func setupTestDB(t *testing.T) *db.DB {
    t.Helper()
    tmpFile, _ := os.CreateTemp("", "test-*.db")
    t.Cleanup(func() { os.Remove(tmpFile.Name()) })
    d, _ := db.New(tmpFile.Name())
    d.Migrate()
    return d
}
```

---

## Common Pitfalls — Avoid These

1. **SQLite concurrent writes** — Use WAL mode (already in db.go). Serialize writes with a mutex if needed.
2. **Goroutine leaks** — Always pass `context.Context`, always select on `ctx.Done()`
3. **embed.FS with Go** — Files must be in the same package or subdirectory. Use `//go:embed web/*`
4. **Shell injection** — Never pass task prompt directly to shell. Write to temp file first.
5. **Telegram rate limits** — Don't send more than 30 messages/second. Use a send queue.
6. **SQLite and CGO** — `go-sqlite3` requires CGO. Use `CGO_ENABLED=1` in build.
7. **Checkpoint race condition** — Use DB transaction when saving checkpoint + updating status.
8. **Frontend CSRF** — Set CSRF token in meta tag, read in JS, send as header on all mutations.

---

## Cross-Platform Rules — Read This Before Writing Any Code

ClawDaemon runs on **Windows, macOS, and Linux**. These rules are mandatory.

### SQLite — driver name changed
```go
// go.mod uses modernc.org/sqlite (pure Go, no CGO)
import _ "modernc.org/sqlite"

// sql.Open uses "sqlite" not "sqlite3"
db, err := sql.Open("sqlite", path+"?_journal=WAL&_foreign_keys=on")
```

### Never hardcode paths — always use filepath.Join
```go
// ✅ CORRECT
path := filepath.Join(workDir, "screenshots", projectID, "before.png")

// ❌ WRONG — breaks on Windows
path := workDir + "/screenshots/" + projectID + "/before.png"
```

### Use internal/platform package for all OS differences
```go
import "github.com/yourusername/clawdaemon/internal/platform"

// Get work directory (OS-aware)
workDir := platform.DefaultWorkDir()

// Get browser config (Lightpanda on Linux, Chrome on Windows/Mac)
browser := platform.DefaultBrowserConfig()

// Find a CLI tool
path, found := platform.LookupCLI("claude")

// Get service manager type
mgr := platform.ServiceManager() // "systemd" | "launchd" | "windows-service"
```

### Browser worker — Lightpanda is Linux only
| Platform | Primary | Fallback |
|---|---|---|
| Linux | Lightpanda (25MB, fast) | chrome-headless-shell |
| macOS | chrome-headless-shell | chrome-headless-shell |
| Windows | chrome-headless-shell | chrome-headless-shell |

### Build commands — no CGO needed
```bash
# Build for current platform
go build .

# Cross-compile for all platforms from any machine
GOOS=linux   GOARCH=amd64  go build -o clawdaemon-linux-amd64 .
GOOS=darwin  GOARCH=arm64  go build -o clawdaemon-darwin-arm64 .
GOOS=windows GOARCH=amd64  go build -o clawdaemon-windows.exe  .
```

### Home directory
```go
// ✅ Works on all platforms
home, _ := os.UserHomeDir()

// ❌ Unix only
home := os.Getenv("HOME")
```
