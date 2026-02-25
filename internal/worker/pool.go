package worker

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/yourusername/clawdaemon/internal/character"
	"github.com/yourusername/clawdaemon/internal/cli"
	"github.com/yourusername/clawdaemon/internal/db"
	"github.com/yourusername/clawdaemon/internal/notify"
	"github.com/yourusername/clawdaemon/internal/queue"
	"github.com/yourusername/clawdaemon/internal/tokenizer"
	"github.com/yourusername/clawdaemon/internal/ws"
)

// Pool manages a set of concurrent Worker goroutines.
type Pool struct {
	mu       sync.Mutex
	cancels  map[int]context.CancelFunc
	wg       sync.WaitGroup
	database *db.DB
	queue    *queue.Queue
	registry *cli.Registry
	injector *character.Injector
	governor *tokenizer.Governor
	hub      *ws.Hub
	notify   *notify.Dispatcher
}

// NewPool creates a Pool with all the shared dependencies.
func NewPool(
	database *db.DB,
	q *queue.Queue,
	registry *cli.Registry,
	injector *character.Injector,
	governor *tokenizer.Governor,
	hub *ws.Hub,
	notifier *notify.Dispatcher,
) *Pool {
	return &Pool{
		cancels:  make(map[int]context.CancelFunc),
		database: database,
		queue:    q,
		registry: registry,
		injector: injector,
		governor: governor,
		hub:      hub,
		notify:   notifier,
	}
}

// StartAll launches goroutines for all provided workers.
// Each worker slot runs max_parallel goroutines.
func (p *Pool) StartAll(ctx context.Context, workers []db.Worker) {
	for _, dbWorker := range workers {
		for i := 0; i < dbWorker.MaxParallel; i++ {
			p.startOne(ctx, dbWorker)
		}
	}
}

// startOne launches a single worker goroutine.
func (p *Pool) startOne(ctx context.Context, dbWorker db.Worker) {
	workerCtx, cancel := context.WithCancel(ctx)
	w := New(dbWorker, p.database, p.queue, p.registry, p.injector, p.governor, p.hub, p.notify)

	p.mu.Lock()
	p.cancels[dbWorker.ID] = cancel
	p.mu.Unlock()

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("worker[%d]: panic recovered: %v", dbWorker.ID, r)
			}
		}()
		w.Run(workerCtx)
	}()
}

// StopAll cancels all running workers and waits for them to finish.
func (p *Pool) StopAll() {
	p.mu.Lock()
	for _, cancel := range p.cancels {
		cancel()
	}
	p.mu.Unlock()
	p.wg.Wait()
}

// RestartWorker stops the worker with the given ID and starts a new one.
func (p *Pool) RestartWorker(ctx context.Context, workerID int) error {
	p.mu.Lock()
	if cancel, ok := p.cancels[workerID]; ok {
		cancel()
		delete(p.cancels, workerID)
	}
	p.mu.Unlock()

	var dbWorker db.Worker
	err := p.database.QueryRowContext(ctx, `
		SELECT id, name, cli_type, command, work_dir, max_parallel, status, project_id, created_at
		FROM workers WHERE id=?`, workerID,
	).Scan(&dbWorker.ID, &dbWorker.Name, &dbWorker.CLIType, &dbWorker.Command,
		&dbWorker.WorkDir, &dbWorker.MaxParallel, &dbWorker.Status,
		&dbWorker.ProjectID, &dbWorker.CreatedAt)
	if err != nil {
		return fmt.Errorf("pool.RestartWorker: %w", err)
	}

	p.startOne(ctx, dbWorker)
	return nil
}

// LoadWorkers reads active workers from the database and starts them.
func (p *Pool) LoadWorkers(ctx context.Context) error {
	rows, err := p.database.QueryContext(ctx, `
		SELECT id, name, cli_type, command, work_dir, max_parallel, status, project_id, created_at
		FROM workers`)
	if err != nil {
		return fmt.Errorf("pool.LoadWorkers: %w", err)
	}
	defer rows.Close()

	var workers []db.Worker
	for rows.Next() {
		var w db.Worker
		if err := rows.Scan(&w.ID, &w.Name, &w.CLIType, &w.Command,
			&w.WorkDir, &w.MaxParallel, &w.Status, &w.ProjectID, &w.CreatedAt); err != nil {
			log.Printf("pool.LoadWorkers: scan: %v", err)
			continue
		}
		workers = append(workers, w)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("pool.LoadWorkers: rows: %w", err)
	}

	p.StartAll(ctx, workers)
	return nil
}
