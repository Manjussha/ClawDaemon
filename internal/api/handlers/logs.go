package handlers

import (
	"net/http"
	"strconv"

	"github.com/yourusername/clawdaemon/internal/db"
)

// ListLogs handles GET /api/v1/logs.
// Query params: task_id, worker_id, level, limit, page.
func (h *Handler) ListLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	limit := 100
	page := 1
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	if v := q.Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	offset := (page - 1) * limit

	where := " WHERE 1=1"
	args := []interface{}{}

	if v := q.Get("task_id"); v != "" {
		if id, err := strconv.Atoi(v); err == nil {
			where += " AND task_id=?"
			args = append(args, id)
		}
	}
	if v := q.Get("worker_id"); v != "" {
		if id, err := strconv.Atoi(v); err == nil {
			where += " AND worker_id=?"
			args = append(args, id)
		}
	}
	if v := q.Get("level"); v != "" {
		where += " AND level=?"
		args = append(args, v)
	}

	query := "SELECT id, worker_id, task_id, level, message, created_at FROM logs" +
		where + " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := h.db.QueryContext(ctx, query, args...)
	if err != nil {
		fail(w, http.StatusInternalServerError, "query: "+err.Error())
		return
	}
	defer rows.Close()

	var logs []db.Log
	for rows.Next() {
		var l db.Log
		if err := rows.Scan(&l.ID, &l.WorkerID, &l.TaskID, &l.Level, &l.Message, &l.CreatedAt); err != nil {
			continue
		}
		logs = append(logs, l)
	}

	var total int
	countQ := "SELECT COUNT(*) FROM logs" + where
	_ = h.db.QueryRowContext(ctx, countQ, args[:len(args)-2]...).Scan(&total)

	okPaginated(w, logs, total, page, limit)
}
