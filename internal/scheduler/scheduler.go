// Package scheduler wraps robfig/cron to manage scheduled task creation.
package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/Manjussha/clawdaemon/internal/db"
)

// TaskEnqueuer can enqueue a new task.
type TaskEnqueuer interface {
	EnqueueRaw(ctx context.Context, title, prompt string, projectID, workerID *int, priority int) error
}

// Engine manages the cron scheduler.
type Engine struct {
	cron     *cron.Cron
	database *db.DB
	enqueuer TaskEnqueuer
	entries  map[int]cron.EntryID
}

// New creates a new cron-based Engine.
func New(database *db.DB, enqueuer TaskEnqueuer) *Engine {
	return &Engine{
		cron:     cron.New(cron.WithSeconds()),
		database: database,
		enqueuer: enqueuer,
		entries:  make(map[int]cron.EntryID),
	}
}

// Start begins the cron engine and loads all enabled schedules.
func (e *Engine) Start(ctx context.Context) error {
	if err := e.LoadSchedules(ctx); err != nil {
		return fmt.Errorf("scheduler.Start: %w", err)
	}
	e.cron.Start()
	go func() {
		<-ctx.Done()
		e.cron.Stop()
	}()
	return nil
}

// LoadSchedules loads all enabled schedules from the DB and registers cron jobs.
func (e *Engine) LoadSchedules(ctx context.Context) error {
	rows, err := e.database.QueryContext(ctx,
		`SELECT id, cron_expr, task_title, task_prompt, project_id, worker_id FROM schedules WHERE enabled=1`)
	if err != nil {
		return fmt.Errorf("scheduler.LoadSchedules: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var s db.Schedule
		if err := rows.Scan(&s.ID, &s.CronExpr, &s.TaskTitle, &s.TaskPrompt, &s.ProjectID, &s.WorkerID); err != nil {
			log.Printf("scheduler: scan schedule: %v", err)
			continue
		}
		if err := e.addJob(s); err != nil {
			log.Printf("scheduler: add job %d: %v", s.ID, err)
		}
	}
	return rows.Err()
}

// AddJob registers a new schedule in the cron engine.
func (e *Engine) AddJob(ctx context.Context, scheduleID int) error {
	var s db.Schedule
	err := e.database.QueryRowContext(ctx,
		`SELECT id, cron_expr, task_title, task_prompt, project_id, worker_id FROM schedules WHERE id=?`,
		scheduleID,
	).Scan(&s.ID, &s.CronExpr, &s.TaskTitle, &s.TaskPrompt, &s.ProjectID, &s.WorkerID)
	if err != nil {
		return fmt.Errorf("scheduler.AddJob: %w", err)
	}
	return e.addJob(s)
}

// RemoveJob deregisters a schedule from the cron engine.
func (e *Engine) RemoveJob(scheduleID int) {
	if entryID, ok := e.entries[scheduleID]; ok {
		e.cron.Remove(entryID)
		delete(e.entries, scheduleID)
	}
}

func (e *Engine) addJob(s db.Schedule) error {
	schedID := s.ID
	entryID, err := e.cron.AddFunc(s.CronExpr, func() {
		ctx := context.Background()
		var pid, wid *int
		if s.ProjectID.Valid {
			v := int(s.ProjectID.Int64)
			pid = &v
		}
		if s.WorkerID.Valid {
			v := int(s.WorkerID.Int64)
			wid = &v
		}
		if err := e.enqueuer.EnqueueRaw(ctx, s.TaskTitle, s.TaskPrompt, pid, wid, 5); err != nil {
			log.Printf("scheduler: enqueue for schedule %d: %v", schedID, err)
			return
		}
		_, _ = e.database.Exec(
			`UPDATE schedules SET last_run=? WHERE id=?`, time.Now(), schedID)
		// Update next_run using cron next computation.
		e.updateNextRun(schedID)
	})
	if err != nil {
		return fmt.Errorf("scheduler.addJob: parse cron: %w", err)
	}
	e.entries[s.ID] = entryID
	e.updateNextRun(s.ID)
	return nil
}

func (e *Engine) updateNextRun(scheduleID int) {
	if entryID, ok := e.entries[scheduleID]; ok {
		entry := e.cron.Entry(entryID)
		if !entry.Next.IsZero() {
			_, _ = e.database.Exec(
				`UPDATE schedules SET next_run=? WHERE id=?`,
				entry.Next, scheduleID,
			)
		}
	}
}

// NullInt converts a *int to sql.NullInt64.
func NullInt(v *int) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}