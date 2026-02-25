// Package worker implements the CLI worker goroutines that execute queued tasks.
package worker

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/yourusername/clawdaemon/internal/character"
	"github.com/yourusername/clawdaemon/internal/cli"
	"github.com/yourusername/clawdaemon/internal/db"
	"github.com/yourusername/clawdaemon/internal/limiter"
	"github.com/yourusername/clawdaemon/internal/notify"
	"github.com/yourusername/clawdaemon/internal/queue"
	"github.com/yourusername/clawdaemon/internal/tokenizer"
	"github.com/yourusername/clawdaemon/internal/ws"
)

// Worker pulls tasks from the queue and executes them via a CLI adapter.
type Worker struct {
	dbWorker  db.Worker
	database  *db.DB
	queue     *queue.Queue
	registry  *cli.Registry
	injector  *character.Injector
	governor  *tokenizer.Governor
	hub       *ws.Hub
	notify    *notify.Dispatcher
	pollEvery time.Duration
}

// New creates a new Worker.
func New(
	dbWorker db.Worker,
	database *db.DB,
	q *queue.Queue,
	registry *cli.Registry,
	injector *character.Injector,
	governor *tokenizer.Governor,
	hub *ws.Hub,
	notifier *notify.Dispatcher,
) *Worker {
	return &Worker{
		dbWorker:  dbWorker,
		database:  database,
		queue:     q,
		registry:  registry,
		injector:  injector,
		governor:  governor,
		hub:       hub,
		notify:    notifier,
		pollEvery: 5 * time.Second,
	}
}

// Run is the main worker loop. Pulls tasks, executes them, sleeps between polls.
// Exits when ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	log.Printf("worker[%d] %s: started", w.dbWorker.ID, w.dbWorker.Name)
	w.setStatus(ctx, "idle")

	for {
		select {
		case <-ctx.Done():
			w.setStatus(context.Background(), "idle")
			return
		default:
		}

		task, err := w.queue.Dequeue(ctx, w.dbWorker.ID)
		if err != nil {
			log.Printf("worker[%d]: dequeue error: %v", w.dbWorker.ID, err)
			w.sleepOrExit(ctx, 10*time.Second)
			continue
		}
		if task == nil {
			w.sleepOrExit(ctx, w.pollEvery)
			continue
		}

		w.setStatus(ctx, "running")
		w.hub.BroadcastToWorker(w.dbWorker.ID, task.ID, "Starting task: "+task.Title, "info")
		w.database.WriteLog(&w.dbWorker.ID, &task.ID, "info", "Starting task: "+task.Title)

		if err := w.executeTask(ctx, task); err != nil {
			log.Printf("worker[%d]: task %d error: %v", w.dbWorker.ID, task.ID, err)
		}
		w.setStatus(ctx, "idle")
	}
}

func (w *Worker) executeTask(ctx context.Context, task *db.Task) error {
	// Health check.
	adapter, ok := w.registry.Get(w.dbWorker.CLIType)
	if !ok {
		_ = w.queue.MarkFailed(ctx, task.ID, "unknown CLI type: "+w.dbWorker.CLIType)
		return fmt.Errorf("worker.executeTask: no adapter for %s", w.dbWorker.CLIType)
	}
	if err := adapter.HealthCheck(ctx); err != nil {
		msg := fmt.Sprintf("CLI health check failed: %v", err)
		_ = w.queue.MarkFailed(ctx, task.ID, msg)
		w.notify.SendTelegram(fmt.Sprintf("❌ Worker %s: %s", w.dbWorker.Name, msg))
		return fmt.Errorf("worker.executeTask: %w", err)
	}

	// Get budget zone.
	zone, err := w.governor.GetBudgetZone(ctx, w.dbWorker.ID)
	if err != nil {
		log.Printf("worker[%d]: get budget zone: %v", w.dbWorker.ID, err)
		zone = tokenizer.ZoneGreen
	}
	w.governor.CheckBudget(ctx, w.dbWorker.ID)

	// Load project context.
	var claudeMD, projectMem string
	if task.ProjectID.Valid {
		var p db.Project
		err := w.database.QueryRowContext(ctx,
			`SELECT claude_md_path, memory_path FROM projects WHERE id=?`,
			task.ProjectID.Int64,
		).Scan(&p.ClaudeMDPath, &p.MemoryPath)
		if err == nil {
			claudeMD = character.ReadProjectFile(p.ClaudeMDPath)
			projectMem = character.ReadProjectFile(p.MemoryPath)
		}
	}

	// Resume from checkpoint if available.
	checkpoint := task.CheckpointData

	// Build full context.
	fullContext := w.injector.BuildContext(character.BuildOpts{
		Zone:            zone,
		ProjectCLAUDEMD: claudeMD,
		ProjectMemory:   projectMem,
		Checkpoint:      checkpoint,
		Prompt:          task.Prompt,
	})

	// Write prompt to temp file (avoids shell escaping and prompt injection).
	tmpFile, err := os.CreateTemp("", "clawdaemon-*.txt")
	if err != nil {
		return fmt.Errorf("worker.executeTask: create temp: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.WriteString(fullContext); err != nil {
		tmpFile.Close()
		return fmt.Errorf("worker.executeTask: write temp: %w", err)
	}
	tmpFile.Close()

	return w.runCLI(ctx, task, adapter, tmpFile.Name())
}

func (w *Worker) runCLI(ctx context.Context, task *db.Task, adapter cli.Worker, promptFile string) error {
	workDir := w.dbWorker.WorkDir
	if workDir == "" {
		workDir = "."
	}
	workDir = filepath.Clean(workDir)

	// Build command: never use bash/sh.
	var args []string
	switch a := adapter.(type) {
	case *cli.ClaudeAdapter:
		args = append(a.DefaultArgs(), promptFile)
	case *cli.GeminiAdapter:
		args = append(a.DefaultArgs(), promptFile)
	default:
		args = []string{promptFile}
	}

	cmd := exec.CommandContext(ctx, adapter.Command(), args...)
	cmd.Dir = workDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("worker.runCLI: stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("worker.runCLI: stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		_ = w.queue.MarkFailed(ctx, task.ID, fmt.Sprintf("start: %v", err))
		return fmt.Errorf("worker.runCLI: start: %w", err)
	}

	det := limiter.New(adapter.CLIType())
	var outputLines []string
	var rateLimitLine string
	hitLimit := false
	lastCheckpoint := time.Now()

	// Stream stdout line by line.
	reader := io.MultiReader(stdout, stderr)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			return fmt.Errorf("worker.runCLI: cancelled")
		default:
		}

		line := scanner.Text()
		outputLines = append(outputLines, line)

		level := "info"
		if strings.Contains(strings.ToLower(line), "error") {
			level = "error"
		}
		w.hub.BroadcastToWorker(w.dbWorker.ID, task.ID, line, level)
		w.database.WriteLog(&w.dbWorker.ID, &task.ID, level, line)

		// Detect rate limit.
		if det.DetectLimit(line) {
			hitLimit = true
			rateLimitLine = line
		}

		// Checkpoint every 60 seconds.
		if time.Since(lastCheckpoint) >= 60*time.Second {
			partial := strings.Join(outputLines, "\n")
			_ = w.queue.SaveCheckpoint(ctx, task.ID, partial, len(outputLines))
			lastCheckpoint = time.Now()
		}
	}

	_ = cmd.Wait()
	fullOutput := strings.Join(outputLines, "\n")

	if hitLimit {
		_ = w.queue.SaveCheckpoint(ctx, task.ID, fullOutput, len(outputLines))
		_ = w.queue.UpdateStatus(ctx, task.ID, "limit")
		w.hub.Broadcast(ws.WSMessage{
			Type:     ws.TypeRateLimit,
			WorkerID: w.dbWorker.ID,
			TaskID:   task.ID,
			Message:  rateLimitLine,
		})
		w.notify.Send("task.limit", map[string]interface{}{
			"worker": w.dbWorker.Name,
			"task":   task.Title,
			"line":   rateLimitLine,
		})
		return &limiter.ErrRateLimit{Line: rateLimitLine}
	}

	// Success.
	inputEst := tokenizer.EstimateTokens(fullOutput)
	outputEst := int(float64(inputEst) * 0.6)

	if err := w.queue.MarkDone(ctx, task.ID, fullOutput, "", inputEst, outputEst); err != nil {
		return fmt.Errorf("worker.runCLI: mark done: %w", err)
	}

	var wid, tid, pid *int
	wid2 := w.dbWorker.ID
	wid = &wid2
	tid2 := task.ID
	tid = &tid2
	if task.ProjectID.Valid {
		pid2 := int(task.ProjectID.Int64)
		pid = &pid2
	}
	_ = w.governor.RecordUsage(ctx, wid, tid, pid, inputEst, outputEst)

	w.hub.Broadcast(ws.WSMessage{
		Type:     ws.TypeTaskComplete,
		WorkerID: w.dbWorker.ID,
		TaskID:   task.ID,
		Message:  fmt.Sprintf("Task '%s' completed", task.Title),
	})
	w.notify.Send("task.complete", map[string]interface{}{
		"worker": w.dbWorker.Name,
		"task":   task.Title,
	})

	return nil
}

func (w *Worker) setStatus(ctx context.Context, status string) {
	_, _ = w.database.ExecContext(ctx, `UPDATE workers SET status=? WHERE id=?`, status, w.dbWorker.ID)
}

func (w *Worker) sleepOrExit(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
