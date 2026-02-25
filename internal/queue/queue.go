// Package queue manages the SQLite-backed task queue.
package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/Manjussha/clawdaemon/internal/db"
)

// Queue wraps the database and provides task queue operations.
type Queue struct {
	database *db.DB
}

// New creates a Queue.
func New(database *db.DB) *Queue {
	return &Queue{database: database}
}

// Enqueue inserts a task into the tasks table with status='pending'.
func (q *Queue) Enqueue(ctx context.Context, task *db.Task) (int64, error) {
	res, err := q.database.ExecContext(ctx, `
		INSERT INTO tasks (title, prompt, project_id, worker_id, priority, status,
		                   template_id, created_at, updated_at)
		VALUES (?,?,?,?,?,'pending',?,?,?)`,
		task.Title, task.Prompt, task.ProjectID, task.WorkerID,
		task.Priority, task.TemplateID,
		time.Now(), time.Now(),
	)
	if err != nil {
		return 0, fmt.Errorf("queue.Enqueue: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("queue.Enqueue: last insert id: %w", err)
	}
	return id, nil
}

// EnqueueRaw enqueues a task from raw fields. Used by the scheduler.
func (q *Queue) EnqueueRaw(ctx context.Context, title, prompt string, projectID, workerID *int, priority int) error {
	task := &db.Task{
		Title:    title,
		Prompt:   prompt,
		Priority: priority,
	}
	if projectID != nil {
		task.ProjectID.Int64 = int64(*projectID)
		task.ProjectID.Valid = true
	}
	if workerID != nil {
		task.WorkerID.Int64 = int64(*workerID)
		task.WorkerID.Valid = true
	}
	_, err := q.Enqueue(ctx, task)
	return err
}

// Dequeue atomically fetches the next pending task for a worker and marks it running.
// Tasks are ordered by priority ASC, then created_at ASC.
// Returns nil, nil when the queue is empty.
func (q *Queue) Dequeue(ctx context.Context, workerID int) (*db.Task, error) {
	tx, err := q.database.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("queue.Dequeue: begin tx: %w", err)
	}
	defer tx.Rollback()

	var t db.Task
	err = tx.QueryRowContext(ctx, `
		SELECT id, title, prompt, project_id, worker_id, priority, status,
		       output, diff, checkpoint_data, progress, input_tokens, output_tokens,
		       template_id, error_message, created_at, updated_at
		FROM tasks
		WHERE status='pending'
		ORDER BY priority ASC, created_at ASC
		LIMIT 1`,
	).Scan(
		&t.ID, &t.Title, &t.Prompt, &t.ProjectID, &t.WorkerID,
		&t.Priority, &t.Status, &t.Output, &t.Diff, &t.CheckpointData,
		&t.Progress, &t.InputTokens, &t.OutputTokens, &t.TemplateID,
		&t.ErrorMessage, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		// No rows = queue is empty.
		tx.Rollback()
		return nil, nil
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE tasks SET status='running', worker_id=?, updated_at=? WHERE id=?`,
		workerID, time.Now(), t.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("queue.Dequeue: update status: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("queue.Dequeue: commit: %w", err)
	}

	t.Status = "running"
	t.WorkerID.Int64 = int64(workerID)
	t.WorkerID.Valid = true
	return &t, nil
}

// SaveCheckpoint saves intermediate output and progress for a running task.
func (q *Queue) SaveCheckpoint(ctx context.Context, taskID int, output string, progress int) error {
	_, err := q.database.ExecContext(ctx, `
		UPDATE tasks SET checkpoint_data=?, progress=?, updated_at=? WHERE id=?`,
		output, progress, time.Now(), taskID,
	)
	if err != nil {
		return fmt.Errorf("queue.SaveCheckpoint: %w", err)
	}
	return nil
}

// GetCheckpoint retrieves the saved checkpoint data for a task.
func (q *Queue) GetCheckpoint(ctx context.Context, taskID int) (string, error) {
	var checkpoint string
	err := q.database.QueryRowContext(ctx,
		`SELECT checkpoint_data FROM tasks WHERE id=?`, taskID,
	).Scan(&checkpoint)
	if err != nil {
		return "", fmt.Errorf("queue.GetCheckpoint: %w", err)
	}
	return checkpoint, nil
}

// MarkDone updates a task as completed with its output and token counts.
func (q *Queue) MarkDone(ctx context.Context, taskID int, output, diff string, inputTokens, outputTokens int) error {
	_, err := q.database.ExecContext(ctx, `
		UPDATE tasks
		SET status='done', output=?, diff=?, input_tokens=?, output_tokens=?,
		    progress=100, updated_at=?
		WHERE id=?`,
		output, diff, inputTokens, outputTokens, time.Now(), taskID,
	)
	if err != nil {
		return fmt.Errorf("queue.MarkDone: %w", err)
	}
	return nil
}

// MarkFailed updates a task as failed with an error message.
func (q *Queue) MarkFailed(ctx context.Context, taskID int, errMsg string) error {
	_, err := q.database.ExecContext(ctx, `
		UPDATE tasks SET status='failed', error_message=?, updated_at=? WHERE id=?`,
		errMsg, time.Now(), taskID,
	)
	if err != nil {
		return fmt.Errorf("queue.MarkFailed: %w", err)
	}
	return nil
}

// UpdateStatus updates only the status field of a task.
func (q *Queue) UpdateStatus(ctx context.Context, taskID int, status string) error {
	_, err := q.database.ExecContext(ctx, `
		UPDATE tasks SET status=?, updated_at=? WHERE id=?`,
		status, time.Now(), taskID,
	)
	if err != nil {
		return fmt.Errorf("queue.UpdateStatus: %w", err)
	}
	return nil
}

// GetTask fetches a task by ID.
func (q *Queue) GetTask(ctx context.Context, taskID int) (*db.Task, error) {
	var t db.Task
	err := q.database.QueryRowContext(ctx, `
		SELECT id, title, prompt, project_id, worker_id, priority, status,
		       output, diff, checkpoint_data, progress, input_tokens, output_tokens,
		       template_id, error_message, created_at, updated_at
		FROM tasks WHERE id=?`, taskID,
	).Scan(
		&t.ID, &t.Title, &t.Prompt, &t.ProjectID, &t.WorkerID,
		&t.Priority, &t.Status, &t.Output, &t.Diff, &t.CheckpointData,
		&t.Progress, &t.InputTokens, &t.OutputTokens, &t.TemplateID,
		&t.ErrorMessage, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("queue.GetTask: %w", err)
	}
	return &t, nil
}

// ListPending returns all pending tasks ordered by priority.
func (q *Queue) ListPending(ctx context.Context) ([]db.Task, error) {
	rows, err := q.database.QueryContext(ctx, `
		SELECT id, title, prompt, project_id, worker_id, priority, status,
		       output, diff, checkpoint_data, progress, input_tokens, output_tokens,
		       template_id, error_message, created_at, updated_at
		FROM tasks
		WHERE status='pending'
		ORDER BY priority ASC, created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("queue.ListPending: %w", err)
	}
	defer rows.Close()
	return scanTasks(rows)
}

func scanTasks(rows interface {
	Next() bool
	Scan(...interface{}) error
	Err() error
}) ([]db.Task, error) {
	var tasks []db.Task
	for rows.Next() {
		var t db.Task
		if err := rows.Scan(
			&t.ID, &t.Title, &t.Prompt, &t.ProjectID, &t.WorkerID,
			&t.Priority, &t.Status, &t.Output, &t.Diff, &t.CheckpointData,
			&t.Progress, &t.InputTokens, &t.OutputTokens, &t.TemplateID,
			&t.ErrorMessage, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("queue.scanTasks: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}
