package handlers

import (
	"net/http"
	"strconv"

	"github.com/Manjussha/clawdaemon/internal/db"
)

// ListSchedules handles GET /api/v1/schedules.
func (h *Handler) ListSchedules(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, name, cron_expr, task_title, task_prompt, project_id, worker_id,
		       enabled, next_run, last_run, created_at
		FROM schedules ORDER BY id`)
	if err != nil {
		fail(w, http.StatusInternalServerError, "query: "+err.Error())
		return
	}
	defer rows.Close()

	var schedules []db.Schedule
	for rows.Next() {
		var s db.Schedule
		if err := rows.Scan(&s.ID, &s.Name, &s.CronExpr, &s.TaskTitle, &s.TaskPrompt,
			&s.ProjectID, &s.WorkerID, &s.Enabled, &s.NextRun, &s.LastRun, &s.CreatedAt); err != nil {
			continue
		}
		schedules = append(schedules, s)
	}
	ok(w, schedules)
}

// CreateSchedule handles POST /api/v1/schedules.
func (h *Handler) CreateSchedule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name       string `json:"name"`
		CronExpr   string `json:"cron_expr"`
		TaskTitle  string `json:"task_title"`
		TaskPrompt string `json:"task_prompt"`
		ProjectID  *int   `json:"project_id"`
		WorkerID   *int   `json:"worker_id"`
		Enabled    bool   `json:"enabled"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Name == "" || req.CronExpr == "" || req.TaskPrompt == "" {
		fail(w, http.StatusBadRequest, "name, cron_expr, and task_prompt are required")
		return
	}
	enabled := 1
	if !req.Enabled {
		enabled = 0
	}
	res, err := h.db.ExecContext(r.Context(), `
		INSERT INTO schedules (name, cron_expr, task_title, task_prompt, project_id, worker_id, enabled)
		VALUES (?,?,?,?,?,?,?)`,
		req.Name, req.CronExpr, req.TaskTitle, req.TaskPrompt,
		nullableInt(req.ProjectID), nullableInt(req.WorkerID), enabled,
	)
	if err != nil {
		fail(w, http.StatusInternalServerError, "insert: "+err.Error())
		return
	}
	id, _ := res.LastInsertId()

	// Register with cron engine.
	if h.scheduler != nil {
		_ = h.scheduler.AddJob(r.Context(), int(id))
	}
	ok(w, map[string]int64{"id": id})
}

// GetSchedule handles GET /api/v1/schedules/{id}.
func (h *Handler) GetSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(pathID(r, "id"))
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}
	var s db.Schedule
	if err := h.db.QueryRowContext(r.Context(), `
		SELECT id, name, cron_expr, task_title, task_prompt, project_id, worker_id,
		       enabled, next_run, last_run, created_at
		FROM schedules WHERE id=?`, id,
	).Scan(&s.ID, &s.Name, &s.CronExpr, &s.TaskTitle, &s.TaskPrompt,
		&s.ProjectID, &s.WorkerID, &s.Enabled, &s.NextRun, &s.LastRun, &s.CreatedAt); err != nil {
		fail(w, http.StatusNotFound, "schedule not found")
		return
	}
	ok(w, s)
}

// UpdateSchedule handles PUT /api/v1/schedules/{id}.
func (h *Handler) UpdateSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(pathID(r, "id"))
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		Name       string `json:"name"`
		CronExpr   string `json:"cron_expr"`
		TaskTitle  string `json:"task_title"`
		TaskPrompt string `json:"task_prompt"`
		Enabled    bool   `json:"enabled"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	enabled := 0
	if req.Enabled {
		enabled = 1
	}
	if _, err := h.db.ExecContext(r.Context(), `
		UPDATE schedules SET name=?, cron_expr=?, task_title=?, task_prompt=?, enabled=? WHERE id=?`,
		req.Name, req.CronExpr, req.TaskTitle, req.TaskPrompt, enabled, id); err != nil {
		fail(w, http.StatusInternalServerError, "update: "+err.Error())
		return
	}
	// Re-register.
	if h.scheduler != nil {
		h.scheduler.RemoveJob(id)
		if req.Enabled {
			_ = h.scheduler.AddJob(r.Context(), id)
		}
	}
	ok(w, map[string]string{"message": "updated"})
}

// DeleteSchedule handles DELETE /api/v1/schedules/{id}.
func (h *Handler) DeleteSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(pathID(r, "id"))
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}
	if h.scheduler != nil {
		h.scheduler.RemoveJob(id)
	}
	if _, err := h.db.ExecContext(r.Context(), `DELETE FROM schedules WHERE id=?`, id); err != nil {
		fail(w, http.StatusInternalServerError, "delete: "+err.Error())
		return
	}
	ok(w, map[string]string{"message": "deleted"})
}
