package handlers

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/Manjussha/clawdaemon/internal/platform"
)

// Status handles GET /api/v1/status.
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	today := time.Now().Format("2006-01-02")

	var activeWorkers, pendingTasks, completedToday, rateLimitsToday int

	_ = h.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM workers WHERE status='running'`).Scan(&activeWorkers)
	_ = h.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tasks WHERE status='pending'`).Scan(&pendingTasks)
	_ = h.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tasks WHERE status='done' AND DATE(updated_at)=?`, today).Scan(&completedToday)
	_ = h.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tasks WHERE status='limit' AND DATE(updated_at)=?`, today).Scan(&rateLimitsToday)

	ok(w, map[string]interface{}{
		"active_workers":    activeWorkers,
		"pending_tasks":     pendingTasks,
		"completed_today":   completedToday,
		"rate_limits_today": rateLimitsToday,
		"ws_clients":        h.hub.ClientCount(),
		"uptime":            time.Now().Format(time.RFC3339),
	})
}

// DaemonStart handles POST /api/v1/daemon/start.
func (h *Handler) DaemonStart(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := h.pool.LoadWorkers(ctx); err != nil {
		fail(w, http.StatusInternalServerError, "start workers: "+err.Error())
		return
	}
	ok(w, map[string]string{"message": "daemon started"})
}

// DaemonStop handles POST /api/v1/daemon/stop.
func (h *Handler) DaemonStop(w http.ResponseWriter, r *http.Request) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = ctx
		h.pool.StopAll()
	}()
	ok(w, map[string]string{"message": "daemon stopping"})
}

// DaemonRestart handles POST /api/v1/daemon/restart.
// It stops all workers gracefully, then re-executes the binary.
func (h *Handler) DaemonRestart(w http.ResponseWriter, r *http.Request) {
	ok(w, map[string]string{"message": "restarting"})

	go func() {
		// Give the HTTP response time to flush to the client.
		time.Sleep(500 * time.Millisecond)
		h.pool.StopAll()
		log.Println("Restarting ClawDaemon…")
		if err := platform.Restart(); err != nil {
			log.Printf("platform.Restart: %v", err)
		}
	}()
}
