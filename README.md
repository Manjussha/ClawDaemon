# ClawDaemon 🐾

[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.22+-00ADD8.svg)](https://golang.org)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey.svg)]()

**An open-source, multi-CLI AI agent orchestrator daemon.**

Run Claude Code, Gemini CLI, and custom AI tools 24×7 on your server with parallel worker pools,
browser automation, token budget governance, Telegram integration, and a real-time web dashboard.

---

## Features

- **Multi-CLI Worker Pools** — Claude Code, Gemini CLI, browser automation (Playwright + Lightpanda)
- **Priority Task Queue** — SQLite-backed, persistent across restarts
- **Token Budget Governance** — GREEN/YELLOW/ORANGE/RED zones with automatic context compression
- **Character System** — Inject IDENTITY, THINKING, RULES, MEMORY, and skills into every task
- **Rate Limit Detection** — Auto-checkpoint and Telegram alert on rate limits
- **Telegram Bot** — Add tasks, check status, skip tasks, pause workers — from your phone
- **Cron Scheduler** — Schedule recurring AI tasks with standard cron syntax
- **Web Dashboard** — Real-time log streaming via WebSocket, task management UI
- **Outbound Webhooks** — Fire events to external services on task completion
- **2FA Login** — OTP via Telegram, brute-force protection
- **Cross-Platform** — Runs on Linux, macOS, Windows — pure Go, no CGO

---

## Quick Start (Docker)

```bash
git clone https://github.com/yourusername/clawdaemon
cd clawdaemon/docker

# Configure
cp .env.example .env
nano .env  # Set TELEGRAM_TOKEN, TELEGRAM_CHAT_ID, ADMIN_PASSWORD

# Launch
docker compose up -d

# Dashboard
open https://your-domain.com
```

---

## Architecture

```
                    ┌─────────────────────────────────────────┐
                    │           ClawDaemon Daemon              │
                    │                                          │
 Telegram Bot ──────┤   ┌──────────┐    ┌──────────────────┐  │
                    │   │  Task    │    │   Worker Pool    │  │
 Web Dashboard ─────┤   │  Queue   │───▶│  (goroutines)    │  │
    (WebSocket)     │   │ (SQLite) │    │                  │  │
                    │   └──────────┘    │ Claude Code CLI  │  │
 REST API ──────────┤                  │ Gemini CLI       │  │
                    │   ┌──────────┐    │ Browser (node)   │  │
 Cron Scheduler ────┤   │Character │    └──────────────────┘  │
                    │   │ Injector │                          │
                    │   └──────────┘                          │
                    └─────────────────────────────────────────┘
```

---

## Platform Compatibility

| Platform         | Status | Browser        |
|-----------------|--------|----------------|
| Linux (amd64)   | ✅     | Lightpanda     |
| Linux (arm64)   | ✅     | Lightpanda     |
| macOS (amd64)   | ✅     | Chrome         |
| macOS (arm64)   | ✅     | Chrome         |
| Windows (amd64) | ✅     | Chrome         |

---

## Build from Source

```bash
# Install Go 1.22+
go mod download && go mod tidy
go build .              # build for current platform
go test ./... -v        # run tests
make release            # cross-compile all 5 platforms
```

---

## API

All endpoints prefixed `/api/v1/`. Authentication via session cookie or `Authorization: Bearer <token>`.

```
POST   /api/v1/auth/login
POST   /api/v1/auth/logout
POST   /api/v1/auth/2fa/verify
GET    /api/v1/status
GET    /api/v1/tasks
POST   /api/v1/tasks
GET    /api/v1/tasks/{id}
DELETE /api/v1/tasks/{id}
GET    /api/v1/workers
POST   /api/v1/workers
GET    /api/v1/logs
GET    /api/v1/usage
GET    /api/v1/settings
PUT    /api/v1/settings/{key}
```

Standard response envelope:
```json
{"success": true, "data": {}}
{"success": false, "error": "message"}
```

---

## Configuration (.env)

| Variable | Default | Description |
|----------|---------|-------------|
| PORT | 8080 | HTTP server port |
| WORK_DIR | OS data dir | Working directory for data |
| DB_PATH | $WORK_DIR/clawdaemon.db | SQLite database path |
| CHARACTER_DIR | $WORK_DIR/character | Character files directory |
| TELEGRAM_TOKEN | — | Bot token from @BotFather |
| TELEGRAM_CHAT_ID | — | Your Telegram chat ID |
| ADMIN_USERNAME | admin | Dashboard login username |
| ADMIN_PASSWORD | changeme | Dashboard login password |
| SESSION_EXPIRY_HOURS | 24 | Session lifetime in hours |
| BRUTE_FORCE_MAX_ATTEMPTS | 5 | Failed logins before block |
| BRUTE_FORCE_BLOCK_MINUTES | 15 | Block duration in minutes |

---

## Telegram Commands

| Command | Description |
|---------|-------------|
| `/status` | Show worker status |
| `/tasks` | List active tasks |
| `/add <prompt>` | Add a new task |
| `/skip <id>` | Skip a task |
| `/stop` | Pause all workers |
| `/memory [note]` | View or add memory |
| `/projects` | List projects |
| `/skills` | List available skills |

---

## License

Apache 2.0 — see [LICENSE](LICENSE).

Built by Manju, CEO of Printigly, Bangalore.
