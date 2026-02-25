# ClawDaemon — TODO List
> Use this as your working checklist. Check off tasks as you complete them.
> Read CLAUDE.md fully before starting any task.

---

## PHASE 1 — Foundation

### 1.1 Project Bootstrap
- [ ] `go mod init github.com/yourusername/clawdaemon`
- [ ] Add all dependencies to go.mod:
  - `github.com/mattn/go-sqlite3`
  - `github.com/gorilla/websocket`
  - `github.com/robfig/cron/v3`
  - `github.com/go-telegram-bot-api/telegram-bot-api/v5`
  - `golang.org/x/crypto`
- [ ] Run `go mod tidy`
- [ ] Create `main.go` — wire all packages, start HTTP server, handle graceful shutdown
- [ ] Create `.env.example` with all env vars documented
- [ ] Create `Makefile` with: `build`, `run`, `test`, `fmt`, `vet`, `clean`

### 1.2 Database (internal/db/)
- [ ] `db.go` — open SQLite connection, enable WAL mode, enable foreign keys
- [ ] `migrations.go` — run all CREATE TABLE statements from CLAUDE.md schema in order
- [ ] Migrations are versioned — each migration has an ID, only runs once
- [ ] Test: DB file created, all tables exist after startup

### 1.3 Configuration (internal/config/)
- [ ] `config.go` — load all env vars into typed Config struct
- [ ] Validate required fields on startup — panic if missing critical config
- [ ] Provide sensible defaults for optional fields

### 1.4 Authentication (internal/auth/)
- [ ] `auth.go` — Login(username, password) → creates session, returns token
- [ ] `auth.go` — Logout(token) → deletes session
- [ ] `auth.go` — ValidateSession(token) → returns user or error
- [ ] `auth.go` — Password hashing with bcrypt (cost factor 12)
- [ ] `twofa.go` — GenerateOTP() → 6-digit code, save to twofa_otp table
- [ ] `twofa.go` — SendOTP(chatID, otp) → send via Telegram bot
- [ ] `twofa.go` — VerifyOTP(userID, otp) → validate, mark used, mark session verified
- [ ] `bruteforce.go` — TrackAttempt(ip, success)
- [ ] `bruteforce.go` — IsBlocked(ip) → true if 5 fails in last 15 min
- [ ] `middleware.go` — RequireAuth handler wrapper — checks session cookie
- [ ] `middleware.go` — RequireAPIKey handler wrapper — checks Bearer token
- [ ] `middleware.go` — RateLimit middleware — brute force check on /login
- [ ] Test: login flow, wrong password blocked after 5 attempts, OTP verified

### 1.5 Settings (internal/settings/)
- [ ] `settings.go` — Get(key) string, Set(key, value), GetAll() map
- [ ] Pre-seed default settings on first run
- [ ] Settings stored in `settings` table

### 1.6 Web Server + Routing (internal/api/)
- [ ] `api.go` — setup all routes using net/http ServeMux
- [ ] `middleware.go` — CSRF protection, logging, recovery from panic
- [ ] Serve embedded frontend from `web/` via embed.FS
- [ ] Health check route: `GET /health` → `{"status":"ok","uptime":"6h42m"}`
- [ ] Static assets served from `/static/`

### 1.7 Auth API Handlers (internal/api/handlers/)
- [ ] `POST /api/v1/auth/login` — validate credentials, create session, return token
- [ ] `POST /api/v1/auth/logout` — destroy session
- [ ] `POST /api/v1/auth/2fa/verify` — verify OTP, mark session as 2FA verified
- [ ] `GET  /api/v1/auth/me` — return current user info

### 1.8 Frontend — Login Page (web/login.html)
- [ ] Clean dark theme login form (username + password)
- [ ] On success → redirect to /2fa page
- [ ] Show error message on wrong credentials
- [ ] Show "account locked" message when brute forced

### 1.9 Frontend — 2FA Page (web/2fa.html)
- [ ] Show "Check your Telegram for OTP" message
- [ ] 6-digit OTP input field
- [ ] 5-minute countdown timer
- [ ] On verify → redirect to dashboard

### 1.10 Setup Wizard (internal/wizard/ + web/wizard.html)
- [ ] Wizard shows on first run (no users in DB)
- [ ] Step 1: Create admin username + password
- [ ] Step 2: Enter Telegram bot token + chat ID → test connection
- [ ] Step 3: Detect installed CLIs (run `which claude`, `which gemini`)
- [ ] Step 4: Configure DuckDNS token + domain (optional)
- [ ] Step 5: Done — redirect to dashboard
- [ ] `wizard.go` — IsFirstRun() bool, CompleteSetup(config)

---

## PHASE 2 — Core Engine

### 2.1 CLI Registry (internal/cli/)
- [ ] `registry.go` — RegisterWorker(worker), GetWorker(id), ListWorkers()
- [ ] `health.go` — HealthCheck(worker) → run `command --version`, return ok/error
- [ ] `claude.go` — Claude Code adapter: command template, limit pattern, output parser
- [ ] `gemini.go` — Gemini CLI adapter: command template, limit pattern, output parser
- [ ] `browser.go` — Browser worker adapter: Lightpanda launch, fallback to chrome-headless-shell
- [ ] `custom.go` — Generic CLI adapter using worker config from DB
- [ ] Test: health check passes for installed CLIs, fails gracefully for missing ones

### 2.2 Task Queue (internal/queue/)
- [ ] `queue.go` — Enqueue(task) — add to DB
- [ ] `queue.go` — Dequeue(workerID) — get next pending task by priority+time
- [ ] `queue.go` — UpdateStatus(taskID, status)
- [ ] `queue.go` — SaveCheckpoint(taskID, output, progress)
- [ ] `queue.go` — GetCheckpoint(taskID) → resume data
- [ ] `queue.go` — MarkDone(taskID, output, diff)
- [ ] `queue.go` — MarkFailed(taskID, error)
- [ ] Queue is backed by SQLite — survives daemon restart
- [ ] Test: tasks enqueued, dequeued in priority order, checkpoint saved/loaded

### 2.3 Worker Pool (internal/worker/)
- [ ] `worker.go` — Single worker goroutine: pull task → inject context → run CLI → parse output → save result
- [ ] `worker.go` — Stream CLI stdout/stderr line by line to WebSocket hub
- [ ] `worker.go` — Detect rate limit in output via regex → checkpoint → notify
- [ ] `pool.go` — WorkerPool struct: manage N workers, start/stop all
- [ ] `pool.go` — StartAll(), StopAll(), GetStatus() 
- [ ] `pool.go` — RestartWorker(id) — restart individual worker goroutine
- [ ] `checkpoint.go` — BuildResumeContext(task) → prepend previous output to prompt
- [ ] Each worker slot is its own goroutine with context cancellation
- [ ] Test: 2 workers run tasks in parallel, one hitting limit doesn't block other

### 2.4 Rate Limit Handler (internal/limiter/)
- [ ] `limiter.go` — DetectLimit(line, pattern) bool
- [ ] `limiter.go` — HandleLimit(workerID, taskID) → checkpoint, update status, notify
- [ ] `limiter.go` — ParseWaitTime(output) → extract wait duration if present
- [ ] `limiter.go` — AutoResume(workerID, waitDuration) → goroutine that waits then restarts

### 2.5 Character System (internal/character/)
- [ ] `loader.go` — LoadIdentity(), LoadThinking(), LoadRules(), LoadMemory()
- [ ] `loader.go` — LoadSkill(name) → read from character/skills/{name}.md
- [ ] `loader.go` — AutoSelectSkill(taskPrompt) → pick best skill by keyword matching
- [ ] `injector.go` — BuildContext(task, project, budgetZone) → assembled prompt string
- [ ] Context injection order follows CLAUDE.md specification exactly
- [ ] `skills.go` — ListSkills(), GetSkill(name), SaveSkill(name, content) (for agent-written skills)
- [ ] Test: context assembled correctly, rules never compressed

### 2.6 Project Memory (internal/memory/)
- [ ] `memory.go` — LoadProjectMemory(projectID) → string
- [ ] `memory.go` — UpdateProjectMemory(projectID, content)
- [ ] `memory.go` — AppendMemoryEntry(projectID, entry) → adds new line
- [ ] After each successful task → agent can append learned facts to project memory

### 2.7 Token Management (internal/tokenizer/)
- [ ] `estimator.go` — EstimateTokens(text) → int (use tiktoken-style approximation: chars/4)
- [ ] `estimator.go` — EstimateTaskCost(task, project) → {inputTokens, outputTokens, total}
- [ ] `optimizer.go` — OptimizeContext(context, budgetZone) → compressed context string
- [ ] `optimizer.go` — CompressSummary(text, maxTokens) → shortened version
- [ ] `tracker.go` — RecordUsage(taskID, workerID, inputTokens, outputTokens, saved)
- [ ] `tracker.go` — GetDailyUsage(workerID) → total tokens today
- [ ] `tracker.go` — GetMonthlyUsage(workerID) → total tokens this month  
- [ ] `governor.go` — GetBudgetZone(workerID) → GREEN|YELLOW|ORANGE|RED
- [ ] `governor.go` — CheckBudget(workerID) → alert Telegram if zone changed
- [ ] Test: token estimation within 10% of actual, compression reduces size

### 2.8 Telegram Bot (internal/telegram/)
- [ ] `bot.go` — Initialize bot, set commands menu, start polling
- [ ] `bot.go` — Only respond to configured TELEGRAM_CHAT_ID — ignore all others
- [ ] `commands.go` — /status → daemon + worker status message
- [ ] `commands.go` — /tasks → list top 10 pending tasks
- [ ] `commands.go` — /add → prompt user for task details via conversation
- [ ] `commands.go` — /skip → ask which worker, then skip current task
- [ ] `commands.go` — /stop → confirm then stop daemon
- [ ] `commands.go` — /memory → show project memory (ask which project)
- [ ] `commands.go` — /projects → list all projects
- [ ] `commands.go` — /skills → list skill library
- [ ] `interactive.go` — SendLimitAlert(workerID, taskID) → inline keyboard: [Switch CLI][Wait][Skip][Pause All]
- [ ] `interactive.go` — HandleCallbackQuery → process button responses
- [ ] `interactive.go` — SendTaskComplete(task) → summary message
- [ ] `interactive.go` — SendDailyHeartbeat() → morning summary
- [ ] `interactive.go` — SendBudgetAlert(zone, workerID) → token budget warning
- [ ] Test: bot responds to /status, limit alert sends inline keyboard

### 2.9 Notifications (internal/notify/)
- [ ] `notify.go` — Dispatcher: route events to configured adapters
- [ ] `telegram.go` — Telegram adapter: send message to admin
- [ ] `webhook.go` — Webhook adapter: POST JSON to configured URL
- [ ] Events: task.complete, task.failed, task.limit, budget.warning, budget.critical, screenshot.saved, test.failed

### 2.10 Webhooks (internal/webhook/)
- [ ] `webhook.go` — Fire(event, payload) → POST to all matching webhook URLs
- [ ] Retry 3 times on failure with exponential backoff
- [ ] Record last_status and last_fired in DB
- [ ] Webhook payload is JSON with: event, timestamp, data object

### 2.11 WebSocket Hub (internal/ws/)
- [ ] `hub.go` — Hub struct: manage connected clients
- [ ] `hub.go` — Broadcast(message) → send to all connected clients
- [ ] `hub.go` — BroadcastToWorker(workerID, line) → send log line
- [ ] `hub.go` — Message types: log_line, worker_status, task_update, system_status
- [ ] `GET /ws` → WebSocket upgrade endpoint
- [ ] Test: client connects, receives log lines as tasks run

### 2.12 Browser Worker (internal/cli/browser.go extended)
- [ ] Launch Lightpanda: `docker run -d --name lp -p 9222:9222 lightpanda/browser:nightly`
- [ ] Connect via CDP (Chrome DevTools Protocol) on port 9222
- [ ] If Lightpanda fails/times out → launch chrome-headless-shell as fallback
- [ ] Agent writes Playwright script to temp file
- [ ] Execute script via `node script.js`
- [ ] Parse test results (pass/fail/error counts)
- [ ] Collect screenshot paths from script output
- [ ] Save screenshot metadata to DB
- [ ] Fire screenshot.saved webhook if configured
- [ ] Save screenshots to /data/screenshots/{project_id}/{task_id}/

### 2.13 Scheduler (internal/scheduler/)
- [ ] `scheduler.go` — Initialize cron engine
- [ ] `scheduler.go` — LoadSchedules() → load all enabled schedules from DB
- [ ] `scheduler.go` — AddJob(schedule) → register with cron engine
- [ ] `scheduler.go` — RemoveJob(scheduleID)
- [ ] `scheduler.go` — UpdateNextRun(scheduleID)
- [ ] On trigger → create task and enqueue it
- [ ] Heartbeat job → registered at startup from settings

### 2.14 REST API Handlers (internal/api/handlers/)
- [ ] `daemon.go` — GET /status, POST /daemon/start, POST /daemon/stop
- [ ] `tasks.go` — CRUD + reorder + output/diff endpoints
- [ ] `workers.go` — CRUD + health check endpoint
- [ ] `projects.go` — CRUD + memory endpoints
- [ ] `character.go` — GET/PUT character files + skills CRUD
- [ ] `schedules.go` — CRUD
- [ ] `webhooks.go` — CRUD + test fire
- [ ] `logs.go` — GET paginated logs with filters
- [ ] `usage.go` — GET token usage stats (daily/weekly/monthly/per-worker/per-project)
- [ ] `settings.go` — GET/PUT settings
- [ ] `users.go` — GET/PUT users
- [ ] `templates.go` — CRUD
- [ ] All handlers return consistent JSON: `{"success":true,"data":{...}}` or `{"success":false,"error":"..."}`

### 2.15 Frontend Dashboard (web/)
- [ ] `index.html` — main dashboard shell with sidebar navigation
- [ ] All 14 pages implemented (see prototype in clawdaemon-dashboard.html)
- [ ] WebSocket client: connect to /ws, handle all message types
- [ ] Live log stream auto-scrolls
- [ ] Worker status cards update in real-time
- [ ] Add Task modal functional
- [ ] Add Worker modal functional
- [ ] All toggle switches save to API
- [ ] Settings form saves to API
- [ ] Character files editable in browser (textarea)
- [ ] Screenshot gallery on Test Results page
- [ ] Usage charts (canvas-based, no external lib)

### 2.16 Agent Chat (internal/chat/)
- [ ] `chat.go` — NewConversation(projectID, workerID)
- [ ] `chat.go` — SendMessage(conversationID, message) → run through worker → stream response
- [ ] `chat.go` — GetHistory(conversationID) → message history
- [ ] Chat uses same context injection as tasks
- [ ] Chat responses streamed via WebSocket to frontend

### 2.17 Deployment Files
- [ ] `docker/Dockerfile` — multi-stage build: Go build → minimal runtime image
- [ ] `docker/docker-compose.yml` — daemon + nginx + lightpanda services
- [ ] `docker/nginx.conf` — HTTPS, rate limiting, proxy to daemon
- [ ] `scripts/install.sh` — detect OS, download binary, setup systemd service, run wizard
- [ ] `scripts/duckdns-renew.sh` — update DuckDNS IP via API
- [ ] `scripts/build.sh` — cross-compile for all platforms
- [ ] `.github/workflows/release.yml` — build + release on git tag
- [ ] `.github/workflows/test.yml` — run tests on PR

---

## PHASE 2 — Character Files

### Default Character Files (character/)
- [ ] `IDENTITY.md` — write default agent identity (helpful, direct, technical)
- [ ] `THINKING.md` — write default thinking approach
- [ ] `RULES.md` — write default safety rules
- [ ] `MEMORY.md` — write empty memory template with instructions
- [ ] `skills/laravel-developer.md`
- [ ] `skills/flutter-developer.md`
- [ ] `skills/wordpress.md`
- [ ] `skills/bug-fixer.md`
- [ ] `skills/code-reviewer.md`
- [ ] `skills/seo-writer.md`
- [ ] `skills/devops.md`
- [ ] `skills/playwright-tester.md` ← for browser worker tasks

---

## PHASE 2 — Open Source Files

### GitHub Repo Files
- [ ] `README.md` — hero section, features, install command, architecture diagram, badges
- [ ] `CONTRIBUTING.md` — fork, branch naming, commit format, PR checklist
- [ ] `CODE_OF_CONDUCT.md` — Contributor Covenant v2.1
- [ ] `CHANGELOG.md` — start with v0.1.0-alpha entry
- [ ] `LICENSE` — Apache 2.0
- [ ] `.github/ISSUE_TEMPLATE/bug_report.md`
- [ ] `.github/ISSUE_TEMPLATE/feature_request.md`
- [ ] `.github/PULL_REQUEST_TEMPLATE.md`

---

## Testing Checklist (run before v0.1.0 release)
- [ ] `go test ./...` passes with no failures
- [ ] Login + 2FA flow works end to end
- [ ] Brute force blocks after 5 attempts
- [ ] Task added via dashboard appears in queue
- [ ] Worker picks up task and streams logs
- [ ] Rate limit detected → checkpoint saved → Telegram alert sent
- [ ] Switching worker resumes task from checkpoint
- [ ] Token budget zone changes trigger correct compression
- [ ] Telegram /status returns correct data
- [ ] Webhook fires on task.complete
- [ ] Browser worker takes screenshot and saves it
- [ ] Schedule triggers task at correct time
- [ ] Settings saved via API persist after restart
- [ ] Docker build succeeds
- [ ] install.sh runs without errors on fresh Ubuntu 24

---

## Definition of Done for v0.1.0-alpha
- All TODO items above checked
- All tests passing
- Docker image builds and runs
- Fresh install via install.sh works in under 10 minutes
- README has demo GIF
- GitHub release v0.1.0 created with binaries for all platforms

---

## CROSS-PLATFORM TASKS (add to Phase 1)

### Platform Package (internal/platform/)
- [ ] `platform.go` already created — review and confirm all functions
- [ ] Test `DefaultWorkDir()` on your OS — check path is correct
- [ ] Test `LookupCLI("claude")` finds Claude Code on your machine
- [ ] Test `DefaultBrowserConfig()` returns correct browser for your OS

### SQLite Driver Fix
- [ ] Confirm go.mod uses `modernc.org/sqlite` NOT `mattn/go-sqlite3`
- [ ] In db.go: use `sql.Open("sqlite", ...)` not `sql.Open("sqlite3", ...)`
- [ ] Run `go build` on Windows/Mac to verify no CGO errors

### File Paths Audit
- [ ] Grep codebase for any hardcoded `/` path separators in strings
- [ ] Replace all `path/path` imports with `path/filepath`
- [ ] Verify all temp files use `os.CreateTemp()`

### CLI Execution Audit  
- [ ] No `exec.Command("bash", ...)` anywhere — use direct exec
- [ ] No `exec.Command("sh", ...)` anywhere
- [ ] All CLI invocations use `exec.Command(name, args...)` pattern

### Installer Scripts
- [ ] `scripts/install-linux.sh` — systemd service setup
- [ ] `scripts/install-mac.sh` — launchd plist setup
- [ ] `scripts/install-windows.ps1` — Windows Service via sc.exe (PowerShell)
- [ ] All scripts detect if running as admin/sudo and request if needed

### Setup Wizard — Platform Aware
- [ ] Wizard detects OS and shows appropriate install path
- [ ] Wizard shows correct service start command per OS:
  - Linux: `sudo systemctl start clawdaemon`
  - macOS: `launchctl start com.clawdaemon`
  - Windows: `Start-Service ClawDaemon`
- [ ] Browser section shows Lightpanda only on Linux; Chrome path detection on all

### GitHub Actions CI
- [ ] `test.yml` — run tests on ubuntu-latest, macos-latest, windows-latest
- [ ] `release.yml` — build binaries for all 5 targets on release tag

### Platform Compatibility Matrix (update README)
| Feature | Linux | macOS | Windows |
|---|---|---|---|
| Core daemon | ✅ | ✅ | ✅ |
| Web dashboard | ✅ | ✅ | ✅ |
| Telegram bot | ✅ | ✅ | ✅ |
| Claude Code worker | ✅ | ✅ | ✅ |
| Gemini CLI worker | ✅ | ✅ | ✅ |
| Lightpanda browser | ✅ | ❌ (Chrome fallback) | ❌ (Chrome fallback) |
| Chrome browser | ✅ | ✅ | ✅ |
| Docker deployment | ✅ | ✅ | ✅ |
| systemd service | ✅ | ❌ | ❌ |
| launchd service | ❌ | ✅ | ❌ |
| Windows Service | ❌ | ❌ | ✅ |
