package handlers

import (
	"net/http"
	"strconv"

	"github.com/Manjussha/clawdaemon/internal/db"
)

// ListTemplates handles GET /api/v1/templates.
func (h *Handler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, name, prompt, cli_type, project_id, created_at FROM templates ORDER BY id`)
	if err != nil {
		fail(w, http.StatusInternalServerError, "query: "+err.Error())
		return
	}
	defer rows.Close()

	var templates []db.Template
	for rows.Next() {
		var t db.Template
		if err := rows.Scan(&t.ID, &t.Name, &t.Prompt, &t.CLIType, &t.ProjectID, &t.CreatedAt); err != nil {
			continue
		}
		templates = append(templates, t)
	}
	ok(w, templates)
}

// CreateTemplate handles POST /api/v1/templates.
func (h *Handler) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string `json:"name"`
		Prompt    string `json:"prompt"`
		CLIType   string `json:"cli_type"`
		ProjectID *int   `json:"project_id"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Name == "" || req.Prompt == "" {
		fail(w, http.StatusBadRequest, "name and prompt are required")
		return
	}
	cliType := req.CLIType
	if cliType == "" {
		cliType = "claude"
	}
	res, err := h.db.ExecContext(r.Context(), `
		INSERT INTO templates (name, prompt, cli_type, project_id) VALUES (?,?,?,?)`,
		req.Name, req.Prompt, cliType, nullableInt(req.ProjectID),
	)
	if err != nil {
		fail(w, http.StatusInternalServerError, "insert: "+err.Error())
		return
	}
	id, _ := res.LastInsertId()
	ok(w, map[string]int64{"id": id})
}

// GetTemplate handles GET /api/v1/templates/{id}.
func (h *Handler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(pathID(r, "id"))
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}
	var t db.Template
	if err := h.db.QueryRowContext(r.Context(), `
		SELECT id, name, prompt, cli_type, project_id, created_at FROM templates WHERE id=?`, id,
	).Scan(&t.ID, &t.Name, &t.Prompt, &t.CLIType, &t.ProjectID, &t.CreatedAt); err != nil {
		fail(w, http.StatusNotFound, "template not found")
		return
	}
	ok(w, t)
}

// UpdateTemplate handles PUT /api/v1/templates/{id}.
func (h *Handler) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(pathID(r, "id"))
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		Name    string `json:"name"`
		Prompt  string `json:"prompt"`
		CLIType string `json:"cli_type"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if _, err := h.db.ExecContext(r.Context(), `
		UPDATE templates SET name=?, prompt=?, cli_type=? WHERE id=?`,
		req.Name, req.Prompt, req.CLIType, id); err != nil {
		fail(w, http.StatusInternalServerError, "update: "+err.Error())
		return
	}
	ok(w, map[string]string{"message": "updated"})
}

// DeleteTemplate handles DELETE /api/v1/templates/{id}.
func (h *Handler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(pathID(r, "id"))
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := h.db.ExecContext(r.Context(), `DELETE FROM templates WHERE id=?`, id); err != nil {
		fail(w, http.StatusInternalServerError, "delete: "+err.Error())
		return
	}
	ok(w, map[string]string{"message": "deleted"})
}
