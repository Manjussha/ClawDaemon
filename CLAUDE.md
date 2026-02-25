# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

> Read every line before writing any code.

---

## What Is ClawDaemon
ClawDaemon is an open-source, multi-CLI AI agent orchestrator daemon built in Go.
It runs 24x7 on a Linux server, managing multiple AI CLI tools (Claude Code, Gemini CLI,
custom CLIs) in parallel worker pools, with browser automation, token optimization,
Telegram integration, and a web dashboard.

**Module:** `github.com/yourusername/clawdaemon`
**Owner:** Manju (CEO, Printigly, Bangalore)
**License:** Apache 2.0
**First release:** v0.1.0-alpha

---

## Build & Development Commands

```bash
# Build for current platform
make build          # → ./clawdaemon (or clawdaemon.exe on Windows)

# Run (builds first)
make run

# Tests with race detector
make test           # → go test ./... -v -race

# Run a single package test
go test ./internal/queue/... -v -run TestQueue_EnqueueDequeue

# Lint
make lint           # → gofmt -w . && go vet ./...
make fmt            # → gofmt -w .
make vet            # → go vet ./...

# Install dependencies
make deps           # → go mod download && go mod tidy

# Cross-compile for all platforms (from any OS — no CGO needed)
make release        # outputs to ./build/

# Docker
make docker         # build image
make docker-up      # start docker-compose stack
make docker-down    # stop stack
```

---

## Tech Stack — Locked, Do Not Change
| Layer | Choice |
|---|---|
| Language | Go 1.22+ |
| Database | SQLite via **modernc.org/sqlite** (pure Go, NO CGO) |
| Web server | net/http stdlib only — no Gin/Echo/Fiber |
| WebSockets | gorilla/websocket |
| Scheduler | robfig/cron v3 |
| Telegram | go-telegram-bot-api v5 |
| Password | bcrypt (golang.org/x/crypto) |
| Frontend | Vanilla HTML/CSS/JS via embed.FS — no framework |

### SQLite Driver — Critical
Use `modernc.org/sqlite` (pure Go, no CGO). **Never use `mattn/go-sqlite3`** (requires CGO).

```go
import _ "modernc.org/sqlite"
db, err := sql.Open("sqlite", path+"?_journal=WAL&_foreign_keys=on")
//                   ^^^^^^^^ "sqlite" not "sqlite3"
```

> **Known inconsistency to fix:** `SKILL.md` and `TODO.md` still contain old references to
> `mattn/go-sqlite3` and `sql.Open("sqlite3", ...)`. The `go.mod` and `Dockerfile` are correct.
> Always follow `go.mod` (which uses `modernc.org/sqlite`).

---

## Current Code State

The project is in early development. Files that currently exist:
- `main.go` — entry point (to be created)
- `platform.go` — OS-aware helpers (exists, in the **root package** `package platform`)
- `go.mod` / `go.sum` — dependencies locked
- `Makefile`, `Dockerfile`, install scripts

Most `internal/` packages are not yet created. `TODO.md` is the authoritative task checklist.

> **Note:** `platform.go` is currently in the root directory (not `internal/platform/`).
> `TODO.md` refers to it as `internal/platform/` — when moving, update all import paths.

---

## Project Structure
```
clawdaemon/
├── main.go                    ← entry point
├── platform.go                ← OS helpers (currently in root package)
├── go.mod / go.sum
├── Makefile / Dockerfile
├── CLAUDE.md / TODO.md / SKILL.md
├── internal/
│   ├── db/                    ← SQLite + migrations
│   ├── auth/                  ← sessions, bcrypt, 2FA, brute force
│   ├── worker/                ← worker pool, goroutines, checkpoint
│   ├── cli/                   ← CLI registry, health check, claude/gemini adapters
│   ├── queue/                 ← shared task queue (SQLite-backed)
│   ├── scheduler/             ← cron engine (robfig/cron)
│   ├── character/             ← identity/thinking/rules/memory loader + injector
│   ├── memory/                ← per-project memory markdown
│   ├── limiter/               ← rate limit detection + handling
│   ├── tokenizer/             ← token estimator, optimizer, tracker, governor
│   ├── telegram/              ← bot setup, commands, interactive responses
│   ├── notify/                ← notification dispatcher
│   ├── webhook/               ← outbound webhooks
│   ├── chat/                  ← agent conversation mode
│   ├── wizard/                ← first-run setup wizard
│   ├── api/                   ← REST API router + middleware + handlers
│   └── ws/                    ← WebSocket hub
├── web/                       ← embedded frontend (embed.FS)
├── character/                 ← default character files (IDENTITY/THINKING/RULES/MEMORY + skills/)
├── docker/
└── scripts/
```

---

## Architecture — Key Data Flows

### HTTP Server Layers
```
net/http ServeMux
├── /                    → embed.FS (web dashboard)
├── /api/v1/...          → REST handlers (JSON)
├── /ws                  → WebSocket (live logs + status)
└── /static/...          → CSS/JS assets
```

### Core Engine
```
WorkerPool  →  goroutines per CLI worker
                └→ pulls from TaskQueue (SQLite-backed, priority ASC, created_at ASC)
                └→ CharacterSystem injects context before every task
                └→ TokenGovernor applies compression per budget zone
                └→ RateLimiter scans output, checkpoints on match
                └→ WebSocketHub streams log lines to dashboard
```

### Context Injection Order (every task, never change this)
1. IDENTITY.md (compressed at YELLOW+)
2. THINKING.md (compressed at YELLOW+)
3. RULES.md (**never** compressed)
4. MEMORY.md (relevant facts only)
5. Relevant skill file
6. Project CLAUDE.md
7. Project memory.md
8. Checkpoint output (if resuming)
9. Task prompt (**never** compressed)

### Token Budget Zones
| Zone | Usage | Action |
|---|---|---|
| GREEN | 0–60% | Full context, no compression |
| YELLOW | 60–80% | Compress IDENTITY+THINKING, trim memory |
| ORANGE | 80–90% | Skip optional context, essentials only |
| RED | 90–100% | Minimum context + Telegram alert |

---

## Database Tables
`workers`, `tasks`, `projects`, `schedules`, `users`, `sessions`, `twofa_otp`,
`login_attempts`, `logs`, `webhooks`, `templates`, `token_usage`, `token_budgets`, `settings`

Full DDL is in `TODO.md` §1.2. Migrations are versioned — each runs exactly once.
Enable WAL mode and foreign keys on every connection:
```go
sql.Open("sqlite", path+"?_journal=WAL&_foreign_keys=on")
```

---

## API Structure
All routes prefixed `/api/v1/`. Auth routes unprotected; all others require session cookie or Bearer token.
- Auth: `POST /api/v1/auth/login`, `/api/v1/auth/logout`, `/api/v1/auth/2fa/verify`
- Standard JSON envelope: `{"success":true,"data":{}}` or `{"success":false,"error":"..."}`
- WebSocket: `GET /ws` — message types: `log_line`, `worker_status`, `task_update`, `system_status`

---

## Cross-Platform Rules — Critical

**Never add new `runtime.GOOS` checks outside `platform.go`.** All OS differences go there.

### File Paths — always use `filepath` package
```go
// ✅ filepath.Join(workDir, "clawdaemon.db")
// ❌ workDir + "/clawdaemon.db"   — breaks on Windows
```

### Shell Commands — no bash/sh assumptions
```go
// ✅ exec.Command("claude", "--dangerously-skip-permissions")
// ❌ exec.Command("bash", "-c", "claude --flag")  — bash not on Windows
```

### CLI tasks from subprocess — write prompt to temp file
```go
tmp, _ := os.CreateTemp("", "clawdaemon-*.txt")
defer os.Remove(tmp.Name())
// avoids shell escaping and prompt injection
```

### Browser
- Lightpanda (Linux only, CDP port 9222) → `platform.DefaultBrowserConfig()` handles this
- Fallback: chrome-headless-shell (Windows/macOS)
- Playwright scripts written to temp file, executed via `node script.js`

### Home directory
```go
home, _ := os.UserHomeDir()  // ✅ all platforms
// os.Getenv("HOME")         // ❌ Unix only
```

---

## Security Rules
- All routes except `/login` require valid session token (cookie middleware)
- 2FA: password login → 6-digit OTP sent to Telegram → must verify before access
- Brute force: block IP after 5 failed attempts for 15 minutes
- CSRF: all POST/PUT/DELETE require `X-CSRF-Token` header
- API routes use `Authorization: Bearer <token>`
- bcrypt cost factor: 12

---

## Coding Standards
- No ignored errors — wrap with `fmt.Errorf("pkg.Func: %w", err)`
- `context.Context` for all blocking operations
- All DB ops use prepared statements
- Log to `logs` table AND structured stdout
- No global variables except initialized singletons
- No goroutines without context cancellation (`select { case <-ctx.Done(): return }`)
- Concurrent SQLite writes: WAL mode is set; serialize writes with a mutex if needed
- `go vet` and `gofmt` must pass before committing

---

## What NOT To Do
- Do NOT use any Go web framework (Gin, Echo, Fiber)
- Do NOT use an ORM — raw SQL with `database/sql` only
- Do NOT use `exec.Command("bash", ...)` or `exec.Command("sh", ...)`
- Do NOT hardcode path separators — use `filepath.Join`
- Do NOT add `runtime.GOOS` checks outside `platform.go`
- Do NOT use `mattn/go-sqlite3` (requires CGO)

---

## Environment Variables (.env)
```
PORT=8080
WORK_DIR=/data
DB_PATH=/data/clawdaemon.db
CHARACTER_DIR=/data/character
SCREENSHOTS_DIR=/data/screenshots
ADMIN_USERNAME=admin
ADMIN_PASSWORD=changeme
TELEGRAM_TOKEN=
TELEGRAM_CHAT_ID=
DUCKDNS_TOKEN=
DUCKDNS_DOMAIN=
JWT_SECRET=
SESSION_EXPIRY_HOURS=24
BRUTE_FORCE_MAX_ATTEMPTS=5
BRUTE_FORCE_BLOCK_MINUTES=15
```
