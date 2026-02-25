// Package db provides the SQLite database wrapper and model types for ClawDaemon.
package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps *sql.DB and provides migration support.
type DB struct {
	*sql.DB
}

// New opens a SQLite connection with WAL mode and foreign keys enabled.
// Driver name is "sqlite" (modernc.org/sqlite, not mattn/go-sqlite3).
func New(path string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", path+"?_journal=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("db.New: open: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("db.New: ping: %w", err)
	}
	// Limit to 1 writer at a time to avoid SQLITE_BUSY in WAL mode.
	sqlDB.SetMaxOpenConns(1)
	return &DB{sqlDB}, nil
}

// Migrate runs all CREATE TABLE IF NOT EXISTS migrations exactly once per schema version.
func (d *DB) Migrate() error {
	// Ensure the settings table exists first (holds schema_version).
	if _, err := d.Exec(ddlSettings); err != nil {
		return fmt.Errorf("db.Migrate: settings table: %w", err)
	}

	// Seed user-facing default settings on every startup.
	// INSERT OR IGNORE is idempotent — existing values are never overwritten.
	defaults := []struct{ k, v string }{
		{"telegram_token", ""},
		{"telegram_chat_id", ""},
		{"duckdns_token", ""},
		{"duckdns_domain", ""},
		{"session_expiry_hours", "24"},
		{"brute_force_max_attempts", "5"},
		{"brute_force_block_minutes", "15"},
	}
	for _, s := range defaults {
		if _, err := d.Exec(`INSERT OR IGNORE INTO settings (key, value) VALUES (?, ?)`, s.k, s.v); err != nil {
			return fmt.Errorf("db.Migrate: seed setting %q: %w", s.k, err)
		}
	}

	// Read current schema version.
	var version int
	row := d.QueryRow(`SELECT value FROM settings WHERE key='schema_version' LIMIT 1`)
	_ = row.Scan(&version) // Ignore scan error — row may not exist yet (version=0).

	if version >= schemaVersion {
		return nil
	}

	tables := []string{
		ddlUsers,
		ddlSessions,
		ddlTwofaOTP,
		ddlLoginAttempts,
		ddlProjects,
		ddlWorkers,
		ddlTasks,
		ddlSchedules,
		ddlLogs,
		ddlWebhooks,
		ddlTemplates,
		ddlTokenUsage,
		ddlTokenBudgets,
	}

	for _, ddl := range tables {
		if _, err := d.Exec(ddl); err != nil {
			return fmt.Errorf("db.Migrate: %w", err)
		}
	}

	// Upsert schema version.
	_, err := d.Exec(`INSERT INTO settings (key, value) VALUES ('schema_version', ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, schemaVersion)
	if err != nil {
		return fmt.Errorf("db.Migrate: schema_version upsert: %w", err)
	}
	return nil
}

const schemaVersion = 1

// ── Model Types ──────────────────────────────────────────────────────────────

// User represents an admin user.
type User struct {
	ID           int       `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// Session represents an authenticated session.
type Session struct {
	ID            int       `json:"id"`
	UserID        int       `json:"user_id"`
	Token         string    `json:"-"`
	TwoFAVerified bool      `json:"twofa_verified"`
	ExpiresAt     time.Time `json:"expires_at"`
	CreatedAt     time.Time `json:"created_at"`
}

// TwofaOTP is a one-time password for 2FA.
type TwofaOTP struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	OTP       string    `json:"-"`
	Used      bool      `json:"used"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// LoginAttempt tracks login tries per IP for brute-force protection.
type LoginAttempt struct {
	ID        int       `json:"id"`
	IP        string    `json:"ip"`
	Success   bool      `json:"success"`
	CreatedAt time.Time `json:"created_at"`
}

// Project is a managed project directory.
type Project struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	ClaudeMDPath string    `json:"claude_md_path"`
	MemoryPath   string    `json:"memory_path"`
	CLIType      string    `json:"cli_type"`
	Description  string    `json:"description"`
	CreatedAt    time.Time `json:"created_at"`
}

// Worker is a CLI worker slot.
type Worker struct {
	ID          int           `json:"id"`
	Name        string        `json:"name"`
	CLIType     string        `json:"cli_type"`
	Command     string        `json:"command"`
	WorkDir     string        `json:"work_dir"`
	MaxParallel int           `json:"max_parallel"`
	Status      string        `json:"status"`
	DailyBudget int           `json:"daily_budget"`
	BudgetZone  string        `json:"budget_zone"`
	ProjectID   sql.NullInt64 `json:"project_id,omitempty"`
	Skill       string        `json:"skill"`
	Model       string        `json:"model"`
	CreatedAt   time.Time     `json:"created_at"`
}

// Task is a queued unit of work.
type Task struct {
	ID             int           `json:"id"`
	Title          string        `json:"title"`
	Prompt         string        `json:"prompt"`
	ProjectID      sql.NullInt64 `json:"project_id,omitempty"`
	WorkerID       sql.NullInt64 `json:"worker_id,omitempty"`
	Priority       int           `json:"priority"`
	Status         string        `json:"status"`
	Output         string        `json:"output,omitempty"`
	Diff           string        `json:"diff,omitempty"`
	CheckpointData string        `json:"-"`
	Progress       int           `json:"progress"`
	InputTokens    int           `json:"input_tokens"`
	OutputTokens   int           `json:"output_tokens"`
	TemplateID     sql.NullInt64 `json:"template_id,omitempty"`
	ErrorMessage   string        `json:"error_message,omitempty"`
	CLIType        string        `json:"cli_type"`
	Skill          string        `json:"skill"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

// Schedule defines a cron-triggered task.
type Schedule struct {
	ID         int          `json:"id"`
	Name       string       `json:"name"`
	CronExpr   string       `json:"cron_expr"`
	TaskTitle  string       `json:"task_title"`
	TaskPrompt string       `json:"prompt"`
	ProjectID  sql.NullInt64 `json:"project_id,omitempty"`
	WorkerID   sql.NullInt64 `json:"worker_id,omitempty"`
	CLIType    string       `json:"cli_type"`
	Priority   int          `json:"priority"`
	Enabled    bool         `json:"enabled"`
	NextRun    sql.NullTime `json:"next_run,omitempty"`
	LastRun    sql.NullTime `json:"last_run,omitempty"`
	CreatedAt  time.Time    `json:"created_at"`
}

// Log is a structured log line.
type Log struct {
	ID        int           `json:"id"`
	WorkerID  sql.NullInt64 `json:"worker_id,omitempty"`
	TaskID    sql.NullInt64 `json:"task_id,omitempty"`
	Level     string        `json:"level"`
	Message   string        `json:"message"`
	CreatedAt time.Time     `json:"created_at"`
}

// Webhook defines an outbound webhook subscription.
type Webhook struct {
	ID         int          `json:"id"`
	Name       string       `json:"name"`
	URL        string       `json:"url"`
	Events     string       `json:"events"`
	Secret     string       `json:"-"`
	Enabled    bool         `json:"enabled"`
	LastStatus int          `json:"last_status"`
	LastFired  sql.NullTime `json:"last_fired,omitempty"`
	CreatedAt  time.Time    `json:"created_at"`
}

// Template is a reusable task prompt template.
type Template struct {
	ID        int           `json:"id"`
	Name      string        `json:"name"`
	Prompt    string        `json:"prompt"`
	CLIType   string        `json:"cli_type"`
	ProjectID sql.NullInt64 `json:"project_id,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
}

// TokenUsage records token consumption per task.
type TokenUsage struct {
	ID           int           `json:"id"`
	WorkerID     sql.NullInt64 `json:"worker_id,omitempty"`
	ProjectID    sql.NullInt64 `json:"project_id,omitempty"`
	TaskID       sql.NullInt64 `json:"task_id,omitempty"`
	InputTokens  int           `json:"input_tokens"`
	OutputTokens int           `json:"output_tokens"`
	Date         string        `json:"date"`
	CreatedAt    time.Time     `json:"created_at"`
}

// TokenBudget defines per-worker daily token limits.
type TokenBudget struct {
	ID            int       `json:"id"`
	WorkerID      int       `json:"worker_id"`
	DailyLimit    int       `json:"daily_limit"`
	YellowPct     int       `json:"yellow_pct"`
	OrangePct     int       `json:"orange_pct"`
	RedPct        int       `json:"red_pct"`
	AlertTelegram bool      `json:"alert_telegram"`
	CreatedAt     time.Time `json:"created_at"`
}

// ── DDL Statements ───────────────────────────────────────────────────────────

const ddlSettings = `CREATE TABLE IF NOT EXISTS settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL DEFAULT ''
);`

const ddlUsers = `CREATE TABLE IF NOT EXISTS users (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	username      TEXT    NOT NULL UNIQUE,
	password_hash TEXT    NOT NULL,
	created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);`

const ddlSessions = `CREATE TABLE IF NOT EXISTS sessions (
	id              INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	token           TEXT    NOT NULL UNIQUE,
	twofa_verified  INTEGER NOT NULL DEFAULT 0,
	expires_at      DATETIME NOT NULL,
	created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);`

const ddlTwofaOTP = `CREATE TABLE IF NOT EXISTS twofa_otp (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	otp        TEXT    NOT NULL,
	used       INTEGER NOT NULL DEFAULT 0,
	expires_at DATETIME NOT NULL,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);`

const ddlLoginAttempts = `CREATE TABLE IF NOT EXISTS login_attempts (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	ip         TEXT    NOT NULL,
	success    INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);`

const ddlProjects = `CREATE TABLE IF NOT EXISTS projects (
	id             INTEGER PRIMARY KEY AUTOINCREMENT,
	name           TEXT    NOT NULL,
	path           TEXT    NOT NULL,
	claude_md_path TEXT    NOT NULL DEFAULT '',
	memory_path    TEXT    NOT NULL DEFAULT '',
	created_at     DATETIME DEFAULT CURRENT_TIMESTAMP
);`

const ddlWorkers = `CREATE TABLE IF NOT EXISTS workers (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	name         TEXT    NOT NULL,
	cli_type     TEXT    NOT NULL DEFAULT 'claude',
	command      TEXT    NOT NULL DEFAULT 'claude',
	work_dir     TEXT    NOT NULL DEFAULT '',
	max_parallel INTEGER NOT NULL DEFAULT 1,
	status       TEXT    NOT NULL DEFAULT 'idle',
	project_id   INTEGER REFERENCES projects(id) ON DELETE SET NULL,
	created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);`

const ddlTasks = `CREATE TABLE IF NOT EXISTS tasks (
	id              INTEGER PRIMARY KEY AUTOINCREMENT,
	title           TEXT    NOT NULL DEFAULT '',
	prompt          TEXT    NOT NULL,
	project_id      INTEGER REFERENCES projects(id) ON DELETE SET NULL,
	worker_id       INTEGER REFERENCES workers(id) ON DELETE SET NULL,
	priority        INTEGER NOT NULL DEFAULT 5,
	status          TEXT    NOT NULL DEFAULT 'pending',
	output          TEXT    NOT NULL DEFAULT '',
	diff            TEXT    NOT NULL DEFAULT '',
	checkpoint_data TEXT    NOT NULL DEFAULT '',
	progress        INTEGER NOT NULL DEFAULT 0,
	input_tokens    INTEGER NOT NULL DEFAULT 0,
	output_tokens   INTEGER NOT NULL DEFAULT 0,
	template_id     INTEGER REFERENCES templates(id) ON DELETE SET NULL,
	error_message   TEXT    NOT NULL DEFAULT '',
	created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);`

const ddlSchedules = `CREATE TABLE IF NOT EXISTS schedules (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	name        TEXT    NOT NULL,
	cron_expr   TEXT    NOT NULL,
	task_title  TEXT    NOT NULL DEFAULT '',
	task_prompt TEXT    NOT NULL,
	project_id  INTEGER REFERENCES projects(id) ON DELETE SET NULL,
	worker_id   INTEGER REFERENCES workers(id) ON DELETE SET NULL,
	enabled     INTEGER NOT NULL DEFAULT 1,
	next_run    DATETIME,
	last_run    DATETIME,
	created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);`

const ddlLogs = `CREATE TABLE IF NOT EXISTS logs (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	worker_id  INTEGER REFERENCES workers(id) ON DELETE SET NULL,
	task_id    INTEGER REFERENCES tasks(id) ON DELETE SET NULL,
	level      TEXT    NOT NULL DEFAULT 'info',
	message    TEXT    NOT NULL,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);`

const ddlWebhooks = `CREATE TABLE IF NOT EXISTS webhooks (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	name        TEXT    NOT NULL,
	url         TEXT    NOT NULL,
	events      TEXT    NOT NULL DEFAULT '',
	secret      TEXT    NOT NULL DEFAULT '',
	enabled     INTEGER NOT NULL DEFAULT 1,
	last_status INTEGER NOT NULL DEFAULT 0,
	last_fired  DATETIME,
	created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);`

const ddlTemplates = `CREATE TABLE IF NOT EXISTS templates (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	name       TEXT    NOT NULL,
	prompt     TEXT    NOT NULL,
	cli_type   TEXT    NOT NULL DEFAULT 'claude',
	project_id INTEGER REFERENCES projects(id) ON DELETE SET NULL,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);`

const ddlTokenUsage = `CREATE TABLE IF NOT EXISTS token_usage (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	worker_id     INTEGER REFERENCES workers(id) ON DELETE SET NULL,
	project_id    INTEGER REFERENCES projects(id) ON DELETE SET NULL,
	task_id       INTEGER REFERENCES tasks(id) ON DELETE SET NULL,
	input_tokens  INTEGER NOT NULL DEFAULT 0,
	output_tokens INTEGER NOT NULL DEFAULT 0,
	date          TEXT    NOT NULL,
	created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);`

const ddlTokenBudgets = `CREATE TABLE IF NOT EXISTS token_budgets (
	id             INTEGER PRIMARY KEY AUTOINCREMENT,
	worker_id      INTEGER NOT NULL UNIQUE REFERENCES workers(id) ON DELETE CASCADE,
	daily_limit    INTEGER NOT NULL DEFAULT 1000000,
	yellow_pct     INTEGER NOT NULL DEFAULT 60,
	orange_pct     INTEGER NOT NULL DEFAULT 80,
	red_pct        INTEGER NOT NULL DEFAULT 90,
	alert_telegram INTEGER NOT NULL DEFAULT 1,
	created_at     DATETIME DEFAULT CURRENT_TIMESTAMP
);`

// ── Helpers ───────────────────────────────────────────────────────────────────

// WriteLog inserts a log line into the logs table.
func (d *DB) WriteLog(workerID, taskID *int, level, message string) {
	var wid, tid sql.NullInt64
	if workerID != nil {
		wid = sql.NullInt64{Int64: int64(*workerID), Valid: true}
	}
	if taskID != nil {
		tid = sql.NullInt64{Int64: int64(*taskID), Valid: true}
	}
	_, _ = d.Exec(
		`INSERT INTO logs (worker_id, task_id, level, message) VALUES (?,?,?,?)`,
		wid, tid, level, message,
	)
}

// GetSetting retrieves a settings value by key, returning fallback if not found.
func (d *DB) GetSetting(key, fallback string) string {
	var v string
	if err := d.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&v); err != nil {
		return fallback
	}
	return v
}

// SetSetting upserts a settings key-value pair.
func (d *DB) SetSetting(key, value string) error {
	_, err := d.Exec(
		`INSERT INTO settings (key, value) VALUES (?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("db.SetSetting: %w", err)
	}
	return nil
}
