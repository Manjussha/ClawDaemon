package tokenizer

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Manjussha/clawdaemon/internal/db"
)

// NotifySender can send a notification message.
type NotifySender interface {
	SendTelegram(msg string)
}

// Governor checks token budget zones and triggers alerts on zone changes.
type Governor struct {
	database *db.DB
	notify   NotifySender
	// Track last known zone per worker to avoid duplicate alerts.
	lastZone map[int]BudgetZone
}

// NewGovernor creates a new Governor.
func NewGovernor(database *db.DB, notify NotifySender) *Governor {
	return &Governor{
		database: database,
		notify:   notify,
		lastZone: make(map[int]BudgetZone),
	}
}

// GetBudgetZone calculates the current zone for a worker based on today's token usage.
func (g *Governor) GetBudgetZone(ctx context.Context, workerID int) (BudgetZone, error) {
	today := time.Now().Format("2006-01-02")

	var used int
	err := g.database.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(input_tokens + output_tokens), 0)
		FROM token_usage
		WHERE worker_id=? AND date=?`, workerID, today,
	).Scan(&used)
	if err != nil {
		return ZoneGreen, fmt.Errorf("governor.GetBudgetZone: usage query: %w", err)
	}

	// Fetch budget thresholds; use defaults if not configured.
	var dailyLimit, yellowPct, orangePct, redPct int
	err = g.database.QueryRowContext(ctx, `
		SELECT daily_limit, yellow_pct, orange_pct, red_pct
		FROM token_budgets WHERE worker_id=?`, workerID,
	).Scan(&dailyLimit, &yellowPct, &orangePct, &redPct)
	if err != nil {
		// No budget configured — use defaults.
		dailyLimit = 1_000_000
		yellowPct = 60
		orangePct = 80
		redPct = 90
	}

	if dailyLimit == 0 {
		return ZoneGreen, nil
	}

	pct := (used * 100) / dailyLimit
	switch {
	case pct >= redPct:
		return ZoneRed, nil
	case pct >= orangePct:
		return ZoneOrange, nil
	case pct >= yellowPct:
		return ZoneYellow, nil
	default:
		return ZoneGreen, nil
	}
}

// CheckBudget detects zone changes and sends Telegram alerts when thresholds are crossed.
func (g *Governor) CheckBudget(ctx context.Context, workerID int) {
	zone, err := g.GetBudgetZone(ctx, workerID)
	if err != nil {
		log.Printf("governor.CheckBudget: %v", err)
		return
	}

	prev, known := g.lastZone[workerID]
	g.lastZone[workerID] = zone
	if known && zone <= prev {
		return // No escalation — don't re-alert.
	}

	switch zone {
	case ZoneYellow:
		g.notify.SendTelegram(fmt.Sprintf(
			"⚠️ Worker %d token budget at YELLOW (60%%+). Compressing context.", workerID))
	case ZoneOrange:
		g.notify.SendTelegram(fmt.Sprintf(
			"🟠 Worker %d token budget at ORANGE (80%%+). Reducing context heavily.", workerID))
	case ZoneRed:
		g.notify.SendTelegram(fmt.Sprintf(
			"🔴 Worker %d token budget at RED (90%%+). Running minimum context only!", workerID))
	}
}

// RecordUsage saves token usage for a task to the token_usage table.
func (g *Governor) RecordUsage(ctx context.Context, workerID, taskID, projectID *int, inputTokens, outputTokens int) error {
	today := time.Now().Format("2006-01-02")
	_, err := g.database.ExecContext(ctx, `
		INSERT INTO token_usage (worker_id, project_id, task_id, input_tokens, output_tokens, date)
		VALUES (?,?,?,?,?,?)`,
		toNullInt(workerID), toNullInt(projectID), toNullInt(taskID),
		inputTokens, outputTokens, today,
	)
	if err != nil {
		return fmt.Errorf("governor.RecordUsage: %w", err)
	}
	return nil
}

func toNullInt(v *int) interface{} {
	if v == nil {
		return nil
	}
	return *v
}
