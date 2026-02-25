package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"

	"github.com/yourusername/clawdaemon/internal/db"
)

type taskInput struct {
	Title      string `json:"title"`
	Prompt     string `json:"prompt"`
	ProjectID  *int   `json:"project_id"`
	WorkerID   *int   `json:"worker_id"`
	TemplateID *int   `json:"template_id"`
	Priority   int    `json:"priority"`
}

// ListTasks handles GET /api/v1/tasks.
func (h *Handler) ListTasks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	status := r.URL.Query().Get("status")
	limit := 50
	page := 1
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	offset := (page - 1) * limit

	query := `SELECT id, title, prompt, project_id, worker_id, priority, status,
		output, diff, checkpoint_data, progress, input_tokens, output_tokens,
		template_id, error_message, created_at, updated_at FROM tasks`
	args := []interface{}{}
	if status != "" {
		query += ` WHERE status=?`
		args = append(args, status)
	}
	query += fmt.Sprintf(` ORDER BY priority ASC, created_at DESC LIMIT %d OFFSET %d`, limit, offset)

	rows, err := h.db.QueryContext(ctx, query, args...)
	if err != nil {
		fail(w, http.StatusInternalServerError, "query: "+err.Error())
		return
	}
	defer rows.Close()

	var tasks []db.Task
	for rows.Next() {
		var t db.Task
		if err := rows.Scan(&t.ID, &t.Title, &t.Prompt, &t.ProjectID, &t.WorkerID,
			&t.Priority, &t.Status, &t.Output, &t.Diff, &t.CheckpointData,
			&t.Progress, &t.InputTokens, &t.OutputTokens, &t.TemplateID,
			&t.ErrorMessage, &t.CreatedAt, &t.UpdatedAt); err != nil {
			continue
		}
		tasks = append(tasks, t)
	}

	var total int
	countQ := `SELECT COUNT(*) FROM tasks`
	if status != "" {
		countQ += ` WHERE status=?`
		_ = h.db.QueryRowContext(ctx, countQ, status).Scan(&total)
	} else {
		_ = h.db.QueryRowContext(ctx, countQ).Scan(&total)
	}

	okPaginated(w, tasks, total, page, limit)
}

// CreateTask handles POST /api/v1/tasks.
func (h *Handler) CreateTask(w http.ResponseWriter, r *http.Request) {
	var req taskInput
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Prompt == "" {
		fail(w, http.StatusBadRequest, "prompt is required")
		return
	}
	if req.Priority == 0 {
		req.Priority = 5
	}

	task := &db.Task{
		Title:    req.Title,
		Prompt:   req.Prompt,
		Priority: req.Priority,
	}
	if req.ProjectID != nil {
		task.ProjectID = sql.NullInt64{Int64: int64(*req.ProjectID), Valid: true}
	}
	if req.WorkerID != nil {
		task.WorkerID = sql.NullInt64{Int64: int64(*req.WorkerID), Valid: true}
	}
	if req.TemplateID != nil {
		task.TemplateID = sql.NullInt64{Int64: int64(*req.TemplateID), Valid: true}
	}

	id, err := h.queue.Enqueue(r.Context(), task)
	if err != nil {
		fail(w, http.StatusInternalServerError, "enqueue: "+err.Error())
		return
	}
	ok(w, map[string]int64{"id": id})
}

// GetTask handles GET /api/v1/tasks/{id}.
func (h *Handler) GetTask(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(pathID(r, "id"))
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}
	task, err := h.queue.GetTask(r.Context(), id)
	if err != nil {
		fail(w, http.StatusNotFound, "task not found")
		return
	}
	ok(w, task)
}

// DeleteTask handles DELETE /api/v1/tasks/{id}.
func (h *Handler) DeleteTask(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(pathID(r, "id"))
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := h.db.ExecContext(r.Context(), `DELETE FROM tasks WHERE id=?`, id); err != nil {
		fail(w, http.StatusInternalServerError, "delete: "+err.Error())
		return
	}
	ok(w, map[string]string{"message": "deleted"})
}

// GetTaskOutput handles GET /api/v1/tasks/{id}/output.
func (h *Handler) GetTaskOutput(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(pathID(r, "id"))
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}
	var output string
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT output FROM tasks WHERE id=?`, id).Scan(&output); err != nil {
		fail(w, http.StatusNotFound, "task not found")
		return
	}
	ok(w, map[string]string{"output": output})
}

// GetTaskDiff handles GET /api/v1/tasks/{id}/diff.
func (h *Handler) GetTaskDiff(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(pathID(r, "id"))
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}
	var diff string
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT diff FROM tasks WHERE id=?`, id).Scan(&diff); err != nil {
		fail(w, http.StatusNotFound, "task not found")
		return
	}
	ok(w, map[string]string{"diff": diff})
}

// ReorderTask handles POST /api/v1/tasks/{id}/reorder.
func (h *Handler) ReorderTask(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(pathID(r, "id"))
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		Priority int `json:"priority"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if _, err := h.db.ExecContext(r.Context(),
		`UPDATE tasks SET priority=? WHERE id=?`, req.Priority, id); err != nil {
		fail(w, http.StatusInternalServerError, "update: "+err.Error())
		return
	}
	ok(w, map[string]string{"message": "reordered"})
}
