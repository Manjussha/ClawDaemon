# ClawDaemon 🐾

[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.22+-00ADD8.svg)](https://golang.org)
[![Release](https://img.shields.io/github/v/release/Manjussha/ClawDaemon)](https://github.com/Manjussha/ClawDaemon/releases)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey.svg)]()

**An open-source, multi-CLI AI agent orchestrator daemon.**

Run Claude Code, Gemini CLI, and custom AI tools 24×7 on your server with parallel worker pools,
browser automation, token budget governance, Telegram integration, and a real-time web dashboard.

---

## Install

### One-liner (Linux / macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/Manjussha/ClawDaemon/main/scripts/install.sh | sh
```

### One-liner (Windows PowerShell)

```powershell
iwr -useb https://raw.githubusercontent.com/Manjussha/ClawDaemon/main/scripts/install.ps1 | iex
```

### Go install

```bash
go install github.com/Manjussha/clawdaemon@latest
```

After install:

```bash
clawdaemon setup    # interactive wizard — port, admin, CLI, Telegram
clawdaemon          # start (uses defaults if no .env)
```

Dashboard → http://localhost:8080  (default credentials: admin / admin)

---

## Quick Start (Docker)

> Requires Docker + Docker Compose. No domain needed for local use.

```bash
git clone https://github.com/Manjussha/ClawDaemon
cd ClawDaemon/docker

# Configure
cp .env.example .env
nano .env   # set ADMIN_PASSWORD, and optionally TELEGRAM_TOKEN + TELEGRAM_CHAT_ID

# Launch
docker compose up -d

# Dashboard
open http://localhost:8080
```

### With HTTPS (production server + domain)

Edit `docker/nginx.conf` and replace `your-domain.com` with your domain, then:

```bash
# Get a free SSL cert via Let's Encrypt
docker compose run --rm certbot certonly --webroot \
  -w /var/www/certbot -d your-domain.com

docker compose up -d

open https://your-domain.com
```

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
- **Cross-Platform** — Linux, macOS, Windows — pure Go, no CGO

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
 REST API ──────────┤                   │ Gemini CLI       │  │
                    │   ┌──────────┐    │ Browser (node)   │  │
 Cron Scheduler ────┤   │Character │    └──────────────────┘  │
                    │   │ Injector │                          │
                    │   └──────────┘                          │
                    └─────────────────────────────────────────┘
```

---

## Platform Compatibility

| Platform         | Status | Browser    |
|-----------------|--------|------------|
| Linux (amd64)   | ✅     | Lightpanda |
| Linux (arm64)   | ✅     | Lightpanda |
| macOS (amd64)   | ✅     | Chrome     |
| macOS (arm64)   | ✅     | Chrome     |
| Windows (amd64) | ✅     | Chrome     |

---

## Build from Source

```bash
git clone https://github.com/Manjussha/ClawDaemon
cd ClawDaemon

go mod download && go mod tidy
go build -o clawdaemon .       # current platform
go test ./... -v -race         # run tests
make release                   # cross-compile all 5 platforms → ./build/
```

---

## API

All endpoints prefixed `/api/v1/`. Auth via session cookie or `Authorization: Bearer <token>`.

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
POST   /api/v1/daemon/restart
```

Response envelope:
```json
{"success": true,  "data": {}}
{"success": false, "error": "message"}
```

---

## Configuration (.env)

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP listen port |
| `WORK_DIR` | OS data dir | Data directory |
| `DB_PATH` | `$WORK_DIR/clawdaemon.db` | SQLite database |
| `CHARACTER_DIR` | `$WORK_DIR/character` | Character files |
| `ADMIN_USERNAME` | `admin` | Dashboard username |
| `ADMIN_PASSWORD` | `admin` | Dashboard password |
| `DEFAULT_CLI` | — | Default CLI (`claude`, `gemini`) |
| `DEFAULT_MODEL` | — | Default model ID |
| `TELEGRAM_TOKEN` | — | Bot token from @BotFather |
| `TELEGRAM_CHAT_ID` | — | Your Telegram chat ID |
| `SESSION_EXPIRY_HOURS` | `24` | Session lifetime |
| `BRUTE_FORCE_MAX_ATTEMPTS` | `5` | Failed logins before block |
| `BRUTE_FORCE_BLOCK_MINUTES` | `15` | Block duration |

---

## Telegram Commands

| Command | Description |
|---------|-------------|
| `/status` | Worker status |
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
