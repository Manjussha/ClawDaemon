// Package handlers provides HTTP handler implementations for the ClawDaemon REST API.
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/yourusername/clawdaemon/internal/character"
	"github.com/yourusername/clawdaemon/internal/config"
	"github.com/yourusername/clawdaemon/internal/db"
	"github.com/yourusername/clawdaemon/internal/notify"
	"github.com/yourusername/clawdaemon/internal/queue"
	"github.com/yourusername/clawdaemon/internal/scheduler"
	"github.com/yourusername/clawdaemon/internal/webhook"
	"github.com/yourusername/clawdaemon/internal/worker"
	"github.com/yourusername/clawdaemon/internal/ws"
)

// Handler holds all shared dependencies for API handler methods.
type Handler struct {
	db        *db.DB
	config    *config.Config
	queue     *queue.Queue
	pool      *worker.Pool
	hub       *ws.Hub
	notify    *notify.Dispatcher
	webhook   *webhook.Dispatcher
	scheduler *scheduler.Engine
	loader    *character.Loader
}

// New creates a Handler with all dependencies.
func New(
	database *db.DB,
	cfg *config.Config,
	q *queue.Queue,
	pool *worker.Pool,
	hub *ws.Hub,
	notifier *notify.Dispatcher,
	wh *webhook.Dispatcher,
	sched *scheduler.Engine,
	loader *character.Loader,
) *Handler {
	return &Handler{
		db:        database,
		config:    cfg,
		queue:     q,
		pool:      pool,
		hub:       hub,
		notify:    notifier,
		webhook:   wh,
		scheduler: sched,
		loader:    loader,
	}
}

// ── Response helpers ──────────────────────────────────────────────────────────

type response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type paginatedResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Meta    pageMeta    `json:"meta"`
}

type pageMeta struct {
	Total int `json:"total"`
	Page  int `json:"page"`
	Limit int `json:"limit"`
}

func ok(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response{Success: true, Data: data})
}

func okPaginated(w http.ResponseWriter, data interface{}, total, page, limit int) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(paginatedResponse{
		Success: true,
		Data:    data,
		Meta:    pageMeta{Total: total, Page: page, Limit: limit},
	})
}

func fail(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(response{Success: false, Error: msg})
}

func decode(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func pathID(r *http.Request, name string) string {
	return r.PathValue(name)
}
