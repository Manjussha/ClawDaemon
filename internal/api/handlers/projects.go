package handlers

import (
	"net/http"
	"os"
	"strconv"

	"github.com/Manjussha/clawdaemon/internal/db"
)

// ListProjects handles GET /api/v1/projects.
func (h *Handler) ListProjects(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, name, path, claude_md_path, memory_path, created_at FROM projects ORDER BY id`)
	if err != nil {
		fail(w, http.StatusInternalServerError, "query: "+err.Error())
		return
	}
	defer rows.Close()

	var projects []db.Project
	for rows.Next() {
		var p db.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Path, &p.ClaudeMDPath, &p.MemoryPath, &p.CreatedAt); err != nil {
			continue
		}
		projects = append(projects, p)
	}
	ok(w, projects)
}

// CreateProject handles POST /api/v1/projects.
func (h *Handler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string `json:"name"`
		Path         string `json:"path"`
		ClaudeMDPath string `json:"claude_md_path"`
		MemoryPath   string `json:"memory_path"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Name == "" || req.Path == "" {
		fail(w, http.StatusBadRequest, "name and path are required")
		return
	}
	res, err := h.db.ExecContext(r.Context(), `
		INSERT INTO projects (name, path, claude_md_path, memory_path)
		VALUES (?,?,?,?)`,
		req.Name, req.Path, req.ClaudeMDPath, req.MemoryPath,
	)
	if err != nil {
		fail(w, http.StatusInternalServerError, "insert: "+err.Error())
		return
	}
	id, _ := res.LastInsertId()
	ok(w, map[string]int64{"id": id})
}

// GetProject handles GET /api/v1/projects/{id}.
func (h *Handler) GetProject(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(pathID(r, "id"))
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}
	var p db.Project
	if err := h.db.QueryRowContext(r.Context(), `
		SELECT id, name, path, claude_md_path, memory_path, created_at FROM projects WHERE id=?`, id,
	).Scan(&p.ID, &p.Name, &p.Path, &p.ClaudeMDPath, &p.MemoryPath, &p.CreatedAt); err != nil {
		fail(w, http.StatusNotFound, "project not found")
		return
	}
	ok(w, p)
}

// UpdateProject handles PUT /api/v1/projects/{id}.
func (h *Handler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(pathID(r, "id"))
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		Name         string `json:"name"`
		Path         string `json:"path"`
		ClaudeMDPath string `json:"claude_md_path"`
		MemoryPath   string `json:"memory_path"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if _, err := h.db.ExecContext(r.Context(), `
		UPDATE projects SET name=?, path=?, claude_md_path=?, memory_path=? WHERE id=?`,
		req.Name, req.Path, req.ClaudeMDPath, req.MemoryPath, id); err != nil {
		fail(w, http.StatusInternalServerError, "update: "+err.Error())
		return
	}
	ok(w, map[string]string{"message": "updated"})
}

// DeleteProject handles DELETE /api/v1/projects/{id}.
func (h *Handler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(pathID(r, "id"))
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := h.db.ExecContext(r.Context(), `DELETE FROM projects WHERE id=?`, id); err != nil {
		fail(w, http.StatusInternalServerError, "delete: "+err.Error())
		return
	}
	ok(w, map[string]string{"message": "deleted"})
}

// GetProjectMemory handles GET /api/v1/projects/{id}/memory.
func (h *Handler) GetProjectMemory(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(pathID(r, "id"))
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}
	var memoryPath string
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT memory_path FROM projects WHERE id=?`, id).Scan(&memoryPath); err != nil {
		fail(w, http.StatusNotFound, "project not found")
		return
	}
	content := ""
	if memoryPath != "" {
		b, _ := os.ReadFile(memoryPath)
		content = string(b)
	}
	ok(w, map[string]string{"content": content})
}

// UpdateProjectMemory handles PUT /api/v1/projects/{id}/memory.
func (h *Handler) UpdateProjectMemory(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(pathID(r, "id"))
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	var memoryPath string
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT memory_path FROM projects WHERE id=?`, id).Scan(&memoryPath); err != nil {
		fail(w, http.StatusNotFound, "project not found")
		return
	}
	if memoryPath == "" {
		fail(w, http.StatusBadRequest, "project has no memory_path configured")
		return
	}
	if err := os.WriteFile(memoryPath, []byte(req.Content), 0644); err != nil {
		fail(w, http.StatusInternalServerError, "write: "+err.Error())
		return
	}
	ok(w, map[string]string{"message": "updated"})
}
