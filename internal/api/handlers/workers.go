package handlers

import (
	"net/http"
	"strconv"

	"github.com/yourusername/clawdaemon/internal/db"
)

// ListWorkers handles GET /api/v1/workers.
func (h *Handler) ListWorkers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, name, cli_type, command, work_dir, max_parallel, status, project_id, created_at
		FROM workers ORDER BY id`)
	if err != nil {
		fail(w, http.StatusInternalServerError, "query: "+err.Error())
		return
	}
	defer rows.Close()

	var workers []db.Worker
	for rows.Next() {
		var w db.Worker
		if err := rows.Scan(&w.ID, &w.Name, &w.CLIType, &w.Command,
			&w.WorkDir, &w.MaxParallel, &w.Status, &w.ProjectID, &w.CreatedAt); err != nil {
			continue
		}
		workers = append(workers, w)
	}
	ok(w, workers)
}

// CreateWorker handles POST /api/v1/workers.
func (h *Handler) CreateWorker(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		CLIType     string `json:"cli_type"`
		Command     string `json:"command"`
		WorkDir     string `json:"work_dir"`
		MaxParallel int    `json:"max_parallel"`
		ProjectID   *int   `json:"project_id"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Name == "" || req.CLIType == "" {
		fail(w, http.StatusBadRequest, "name and cli_type are required")
		return
	}
	if req.MaxParallel == 0 {
		req.MaxParallel = 1
	}
	command := req.Command
	if command == "" {
		command = req.CLIType
	}

	res, err := h.db.ExecContext(r.Context(), `
		INSERT INTO workers (name, cli_type, command, work_dir, max_parallel, project_id)
		VALUES (?,?,?,?,?,?)`,
		req.Name, req.CLIType, command, req.WorkDir, req.MaxParallel, nullableInt(req.ProjectID),
	)
	if err != nil {
		fail(w, http.StatusInternalServerError, "insert: "+err.Error())
		return
	}
	id, _ := res.LastInsertId()
	ok(w, map[string]int64{"id": id})
}

// GetWorker handles GET /api/v1/workers/{id}.
func (h *Handler) GetWorker(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(pathID(r, "id"))
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}
	var wo db.Worker
	if err := h.db.QueryRowContext(r.Context(), `
		SELECT id, name, cli_type, command, work_dir, max_parallel, status, project_id, created_at
		FROM workers WHERE id=?`, id,
	).Scan(&wo.ID, &wo.Name, &wo.CLIType, &wo.Command,
		&wo.WorkDir, &wo.MaxParallel, &wo.Status, &wo.ProjectID, &wo.CreatedAt); err != nil {
		fail(w, http.StatusNotFound, "worker not found")
		return
	}
	ok(w, wo)
}

// UpdateWorker handles PUT /api/v1/workers/{id}.
func (h *Handler) UpdateWorker(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(pathID(r, "id"))
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		Name        string `json:"name"`
		CLIType     string `json:"cli_type"`
		Command     string `json:"command"`
		WorkDir     string `json:"work_dir"`
		MaxParallel int    `json:"max_parallel"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if _, err := h.db.ExecContext(r.Context(), `
		UPDATE workers SET name=?, cli_type=?, command=?, work_dir=?, max_parallel=? WHERE id=?`,
		req.Name, req.CLIType, req.Command, req.WorkDir, req.MaxParallel, id); err != nil {
		fail(w, http.StatusInternalServerError, "update: "+err.Error())
		return
	}
	ok(w, map[string]string{"message": "updated"})
}

// DeleteWorker handles DELETE /api/v1/workers/{id}.
func (h *Handler) DeleteWorker(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(pathID(r, "id"))
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := h.db.ExecContext(r.Context(), `DELETE FROM workers WHERE id=?`, id); err != nil {
		fail(w, http.StatusInternalServerError, "delete: "+err.Error())
		return
	}
	ok(w, map[string]string{"message": "deleted"})
}

// WorkerHealth handles POST /api/v1/workers/{id}/health.
func (h *Handler) WorkerHealth(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(pathID(r, "id"))
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}
	var cliType string
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT cli_type FROM workers WHERE id=?`, id).Scan(&cliType); err != nil {
		fail(w, http.StatusNotFound, "worker not found")
		return
	}

	// Simple health response — full health check runs in the pool via restart.
	ok(w, map[string]interface{}{
		"worker_id": id,
		"cli_type":  cliType,
		"healthy":   true,
		"message":   "Use restart to reload worker",
	})
}

func nullableInt(v *int) interface{} {
	if v == nil {
		return nil
	}
	return *v
}
