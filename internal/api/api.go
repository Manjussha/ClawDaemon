// Package api sets up the HTTP routes and middleware for ClawDaemon's REST API.
package api

import (
	"net/http"

	"github.com/Manjussha/clawdaemon/internal/api/handlers"
	"github.com/Manjussha/clawdaemon/internal/auth"
	"github.com/Manjussha/clawdaemon/internal/character"
	"github.com/Manjussha/clawdaemon/internal/config"
	"github.com/Manjussha/clawdaemon/internal/db"
	"github.com/Manjussha/clawdaemon/internal/notify"
	"github.com/Manjussha/clawdaemon/internal/queue"
	"github.com/Manjussha/clawdaemon/internal/scheduler"
	"github.com/Manjussha/clawdaemon/internal/webhook"
	"github.com/Manjussha/clawdaemon/internal/worker"
	"github.com/Manjussha/clawdaemon/internal/ws"
)

// Deps holds all dependencies injected into the API handlers.
type Deps struct {
	DB        *db.DB
	Config    *config.Config
	Queue     *queue.Queue
	Pool      *worker.Pool
	Hub       *ws.Hub
	Notify    *notify.Dispatcher
	Webhook   *webhook.Dispatcher
	Scheduler *scheduler.Engine
	Loader    *character.Loader
	Injector  *character.Injector
}

// SetupRoutes registers all HTTP routes on the given ServeMux.
// Uses Go 1.22 method+pattern routing syntax.
func SetupRoutes(mux *http.ServeMux, deps *Deps) {
	h := handlers.New(deps.DB, deps.Config, deps.Queue, deps.Pool, deps.Hub,
		deps.Notify, deps.Webhook, deps.Scheduler, deps.Loader)

	requireAuth := func(next http.Handler) http.Handler {
		return auth.RequireAPIKey(deps.DB, next)
	}

	// ── Setup routes (no auth — runs before first login) ─────────────────────
	mux.HandleFunc("GET /api/setup/status", h.SetupStatus)
	mux.HandleFunc("GET /api/setup/ports", h.ScanPorts)
	mux.HandleFunc("GET /api/setup/listening", h.ListeningPorts)
	mux.HandleFunc("POST /api/setup/save", h.SaveSetup)

	// ── Public routes ────────────────────────────────────────────────────────
	mux.HandleFunc("POST /api/v1/auth/login", h.Login)
	mux.HandleFunc("POST /api/v1/auth/logout", h.Logout)
	mux.HandleFunc("POST /api/v1/auth/2fa/verify", h.Verify2FA)

	// ── Protected routes ─────────────────────────────────────────────────────
	// Auth
	mux.Handle("GET /api/v1/auth/me", requireAuth(http.HandlerFunc(h.Me)))

	// Daemon status
	mux.Handle("GET /api/v1/status", requireAuth(http.HandlerFunc(h.Status)))
	mux.Handle("POST /api/v1/daemon/start", requireAuth(csrfGuard(http.HandlerFunc(h.DaemonStart))))
	mux.Handle("POST /api/v1/daemon/stop", requireAuth(csrfGuard(http.HandlerFunc(h.DaemonStop))))
	mux.Handle("POST /api/v1/daemon/restart", requireAuth(csrfGuard(http.HandlerFunc(h.DaemonRestart))))

	// Tasks
	mux.Handle("GET /api/v1/tasks", requireAuth(http.HandlerFunc(h.ListTasks)))
	mux.Handle("POST /api/v1/tasks", requireAuth(csrfGuard(http.HandlerFunc(h.CreateTask))))
	mux.Handle("GET /api/v1/tasks/{id}", requireAuth(http.HandlerFunc(h.GetTask)))
	mux.Handle("DELETE /api/v1/tasks/{id}", requireAuth(csrfGuard(http.HandlerFunc(h.DeleteTask))))
	mux.Handle("GET /api/v1/tasks/{id}/output", requireAuth(http.HandlerFunc(h.GetTaskOutput)))
	mux.Handle("GET /api/v1/tasks/{id}/diff", requireAuth(http.HandlerFunc(h.GetTaskDiff)))
	mux.Handle("POST /api/v1/tasks/{id}/reorder", requireAuth(csrfGuard(http.HandlerFunc(h.ReorderTask))))

	// Workers
	mux.Handle("GET /api/v1/workers", requireAuth(http.HandlerFunc(h.ListWorkers)))
	mux.Handle("POST /api/v1/workers", requireAuth(csrfGuard(http.HandlerFunc(h.CreateWorker))))
	mux.Handle("GET /api/v1/workers/{id}", requireAuth(http.HandlerFunc(h.GetWorker)))
	mux.Handle("PUT /api/v1/workers/{id}", requireAuth(csrfGuard(http.HandlerFunc(h.UpdateWorker))))
	mux.Handle("DELETE /api/v1/workers/{id}", requireAuth(csrfGuard(http.HandlerFunc(h.DeleteWorker))))
	mux.Handle("POST /api/v1/workers/{id}/health", requireAuth(csrfGuard(http.HandlerFunc(h.WorkerHealth))))

	// Projects
	mux.Handle("GET /api/v1/projects", requireAuth(http.HandlerFunc(h.ListProjects)))
	mux.Handle("POST /api/v1/projects", requireAuth(csrfGuard(http.HandlerFunc(h.CreateProject))))
	mux.Handle("GET /api/v1/projects/{id}", requireAuth(http.HandlerFunc(h.GetProject)))
	mux.Handle("PUT /api/v1/projects/{id}", requireAuth(csrfGuard(http.HandlerFunc(h.UpdateProject))))
	mux.Handle("DELETE /api/v1/projects/{id}", requireAuth(csrfGuard(http.HandlerFunc(h.DeleteProject))))
	mux.Handle("GET /api/v1/projects/{id}/memory", requireAuth(http.HandlerFunc(h.GetProjectMemory)))
	mux.Handle("PUT /api/v1/projects/{id}/memory", requireAuth(csrfGuard(http.HandlerFunc(h.UpdateProjectMemory))))

	// Character
	mux.Handle("GET /api/v1/character/{file}", requireAuth(http.HandlerFunc(h.GetCharacterFile)))
	mux.Handle("PUT /api/v1/character/{file}", requireAuth(csrfGuard(http.HandlerFunc(h.UpdateCharacterFile))))
	mux.Handle("GET /api/v1/character/skills", requireAuth(http.HandlerFunc(h.ListSkills)))
	mux.Handle("POST /api/v1/character/skills", requireAuth(csrfGuard(http.HandlerFunc(h.CreateSkill))))
	mux.Handle("PUT /api/v1/character/skills/{name}", requireAuth(csrfGuard(http.HandlerFunc(h.UpdateSkill))))
	mux.Handle("DELETE /api/v1/character/skills/{name}", requireAuth(csrfGuard(http.HandlerFunc(h.DeleteSkill))))

	// Schedules
	mux.Handle("GET /api/v1/schedules", requireAuth(http.HandlerFunc(h.ListSchedules)))
	mux.Handle("POST /api/v1/schedules", requireAuth(csrfGuard(http.HandlerFunc(h.CreateSchedule))))
	mux.Handle("GET /api/v1/schedules/{id}", requireAuth(http.HandlerFunc(h.GetSchedule)))
	mux.Handle("PUT /api/v1/schedules/{id}", requireAuth(csrfGuard(http.HandlerFunc(h.UpdateSchedule))))
	mux.Handle("DELETE /api/v1/schedules/{id}", requireAuth(csrfGuard(http.HandlerFunc(h.DeleteSchedule))))

	// Webhooks
	mux.Handle("GET /api/v1/webhooks", requireAuth(http.HandlerFunc(h.ListWebhooks)))
	mux.Handle("POST /api/v1/webhooks", requireAuth(csrfGuard(http.HandlerFunc(h.CreateWebhook))))
	mux.Handle("GET /api/v1/webhooks/{id}", requireAuth(http.HandlerFunc(h.GetWebhook)))
	mux.Handle("PUT /api/v1/webhooks/{id}", requireAuth(csrfGuard(http.HandlerFunc(h.UpdateWebhook))))
	mux.Handle("DELETE /api/v1/webhooks/{id}", requireAuth(csrfGuard(http.HandlerFunc(h.DeleteWebhook))))
	mux.Handle("POST /api/v1/webhooks/{id}/test", requireAuth(csrfGuard(http.HandlerFunc(h.TestWebhook))))

	// Logs
	mux.Handle("GET /api/v1/logs", requireAuth(http.HandlerFunc(h.ListLogs)))

	// Usage
	mux.Handle("GET /api/v1/usage", requireAuth(http.HandlerFunc(h.GetUsage)))

	// Settings
	mux.Handle("GET /api/v1/settings", requireAuth(http.HandlerFunc(h.ListSettings)))
	mux.Handle("PUT /api/v1/settings/{key}", requireAuth(csrfGuard(http.HandlerFunc(h.UpdateSetting))))

	// Templates
	mux.Handle("GET /api/v1/templates", requireAuth(http.HandlerFunc(h.ListTemplates)))
	mux.Handle("POST /api/v1/templates", requireAuth(csrfGuard(http.HandlerFunc(h.CreateTemplate))))
	mux.Handle("GET /api/v1/templates/{id}", requireAuth(http.HandlerFunc(h.GetTemplate)))
	mux.Handle("PUT /api/v1/templates/{id}", requireAuth(csrfGuard(http.HandlerFunc(h.UpdateTemplate))))
	mux.Handle("DELETE /api/v1/templates/{id}", requireAuth(csrfGuard(http.HandlerFunc(h.DeleteTemplate))))
}

// csrfGuard enforces X-CSRF-Token header on mutating requests.
func csrfGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-CSRF-Token") == "" {
			http.Error(w, `{"success":false,"error":"missing CSRF token"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
