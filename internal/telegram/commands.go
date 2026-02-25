package telegram

import (
	"context"
	"fmt"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/yourusername/clawdaemon/internal/db"
)

// CommandHandler handles Telegram bot commands.
type CommandHandler struct {
	database *db.DB
	bot      *Bot
}

// NewCommandHandler creates a CommandHandler.
func NewCommandHandler(database *db.DB) *CommandHandler {
	return &CommandHandler{database: database}
}

// Handle dispatches incoming messages to the correct command handler.
func (h *CommandHandler) Handle(msg *tgbotapi.Message) {
	if msg == nil || !msg.IsCommand() {
		return
	}
	ctx := context.Background()
	switch msg.Command() {
	case "status":
		h.handleStatus(ctx, msg)
	case "tasks":
		h.handleTasks(ctx, msg)
	case "add":
		h.handleAdd(ctx, msg)
	case "skip":
		h.handleSkip(ctx, msg)
	case "stop":
		h.handleStop(ctx, msg)
	case "memory":
		h.handleMemory(ctx, msg)
	case "projects":
		h.handleProjects(ctx, msg)
	case "skills":
		h.handleSkills(ctx, msg)
	case "help":
		h.handleHelp(msg)
	default:
		h.bot.reply(msg.Chat.ID, "Unknown command. Use /help for a list of commands.")
	}
}

// HandleCallback processes inline keyboard button presses.
func (h *CommandHandler) HandleCallback(data, queryID string) {
	ctx := context.Background()
	switch {
	case strings.HasPrefix(data, "skip_"):
		taskID := 0
		fmt.Sscanf(data, "skip_%d", &taskID)
		if taskID > 0 {
			if _, err := h.database.ExecContext(ctx,
				`UPDATE tasks SET status='failed', error_message='Skipped via Telegram' WHERE id=?`,
				taskID); err != nil {
				log.Printf("telegram: skip task %d: %v", taskID, err)
			}
		}
	case strings.HasPrefix(data, "wait_"):
		// Mark as pending again — will be picked up after rate limit window.
		taskID := 0
		fmt.Sscanf(data, "wait_%d", &taskID)
		if taskID > 0 {
			if _, err := h.database.ExecContext(ctx,
				`UPDATE tasks SET status='pending' WHERE id=? AND status='limit'`,
				taskID); err != nil {
				log.Printf("telegram: wait task %d: %v", taskID, err)
			}
		}
	case data == "pause_all":
		if _, err := h.database.ExecContext(ctx,
			`UPDATE workers SET status='paused'`); err != nil {
			log.Printf("telegram: pause all workers: %v", err)
		}
	}
}

func (h *CommandHandler) handleStatus(ctx context.Context, msg *tgbotapi.Message) {
	rows, err := h.database.QueryContext(ctx,
		`SELECT name, status FROM workers ORDER BY id`)
	if err != nil {
		h.bot.reply(msg.Chat.ID, "Error fetching worker status.")
		return
	}
	defer rows.Close()

	var sb strings.Builder
	sb.WriteString("*Worker Status*\n\n")
	for rows.Next() {
		var name, status string
		if err := rows.Scan(&name, &status); err != nil {
			continue
		}
		icon := statusIcon(status)
		sb.WriteString(fmt.Sprintf("%s %s — `%s`\n", icon, name, status))
	}
	h.bot.reply(msg.Chat.ID, sb.String())
}

func (h *CommandHandler) handleTasks(ctx context.Context, msg *tgbotapi.Message) {
	rows, err := h.database.QueryContext(ctx, `
		SELECT id, title, status FROM tasks
		WHERE status IN ('pending','running','limit')
		ORDER BY priority ASC, created_at ASC LIMIT 10`)
	if err != nil {
		h.bot.reply(msg.Chat.ID, "Error fetching tasks.")
		return
	}
	defer rows.Close()

	var sb strings.Builder
	sb.WriteString("*Pending/Running Tasks*\n\n")
	count := 0
	for rows.Next() {
		var id int
		var title, status string
		if err := rows.Scan(&id, &title, &status); err != nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("#%d [%s] %s\n", id, status, title))
		count++
	}
	if count == 0 {
		sb.WriteString("_No active tasks._")
	}
	h.bot.reply(msg.Chat.ID, sb.String())
}

func (h *CommandHandler) handleAdd(ctx context.Context, msg *tgbotapi.Message) {
	args := msg.CommandArguments()
	if args == "" {
		h.bot.reply(msg.Chat.ID, "Usage: /add <task prompt>")
		return
	}
	_, err := h.database.ExecContext(ctx,
		`INSERT INTO tasks (title, prompt, priority, status) VALUES (?,?,'5','pending')`,
		"Telegram Task", args)
	if err != nil {
		h.bot.reply(msg.Chat.ID, fmt.Sprintf("Error adding task: %v", err))
		return
	}
	h.bot.reply(msg.Chat.ID, "✅ Task added to queue.")
}

func (h *CommandHandler) handleSkip(ctx context.Context, msg *tgbotapi.Message) {
	args := msg.CommandArguments()
	var taskID int
	if _, err := fmt.Sscanf(args, "%d", &taskID); err != nil {
		h.bot.reply(msg.Chat.ID, "Usage: /skip <task_id>")
		return
	}
	_, _ = h.database.ExecContext(ctx,
		`UPDATE tasks SET status='failed', error_message='Skipped via Telegram' WHERE id=?`, taskID)
	h.bot.reply(msg.Chat.ID, fmt.Sprintf("Task #%d skipped.", taskID))
}

func (h *CommandHandler) handleStop(ctx context.Context, msg *tgbotapi.Message) {
	_, _ = h.database.ExecContext(ctx, `UPDATE workers SET status='paused'`)
	h.bot.reply(msg.Chat.ID, "⏹ All workers paused.")
}

func (h *CommandHandler) handleMemory(ctx context.Context, msg *tgbotapi.Message) {
	args := msg.CommandArguments()
	if args != "" {
		// Append to global MEMORY.md via settings.
		current := h.database.GetSetting("global_memory", "")
		updated := current + "\n- " + args
		if err := h.database.SetSetting("global_memory", updated); err != nil {
			h.bot.reply(msg.Chat.ID, "Error updating memory.")
			return
		}
		h.bot.reply(msg.Chat.ID, "✅ Memory updated.")
		return
	}
	memory := h.database.GetSetting("global_memory", "_No memory entries yet._")
	h.bot.reply(msg.Chat.ID, "*Global Memory*\n\n"+memory)
}

func (h *CommandHandler) handleProjects(ctx context.Context, msg *tgbotapi.Message) {
	rows, err := h.database.QueryContext(ctx, `SELECT id, name, path FROM projects ORDER BY id`)
	if err != nil {
		h.bot.reply(msg.Chat.ID, "Error fetching projects.")
		return
	}
	defer rows.Close()

	var sb strings.Builder
	sb.WriteString("*Projects*\n\n")
	count := 0
	for rows.Next() {
		var id int
		var name, path string
		if err := rows.Scan(&id, &name, &path); err != nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("#%d %s\n`%s`\n\n", id, name, path))
		count++
	}
	if count == 0 {
		sb.WriteString("_No projects configured._")
	}
	h.bot.reply(msg.Chat.ID, sb.String())
}

func (h *CommandHandler) handleSkills(ctx context.Context, msg *tgbotapi.Message) {
	skills := h.database.GetSetting("skill_list", "bug-fixer, code-reviewer, laravel-developer, flutter-developer, wordpress, seo-writer, devops, playwright-tester")
	h.bot.reply(msg.Chat.ID, "*Available Skills*\n\n"+skills)
}

func (h *CommandHandler) handleHelp(msg *tgbotapi.Message) {
	help := `*ClawDaemon Commands*

/status — Worker status
/tasks — Active tasks
/add <prompt> — Add a task
/skip <id> — Skip a task
/stop — Pause all workers
/memory [note] — View or add memory
/projects — List projects
/skills — List available skills
/help — This help`
	h.bot.reply(msg.Chat.ID, help)
}

func statusIcon(status string) string {
	switch status {
	case "running":
		return "🟢"
	case "idle":
		return "⚪"
	case "paused":
		return "🟡"
	case "error":
		return "🔴"
	default:
		return "⚫"
	}
}
