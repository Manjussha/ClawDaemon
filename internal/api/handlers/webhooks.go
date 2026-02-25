package handlers

import (
	"net/http"
	"strconv"

	"github.com/Manjussha/clawdaemon/internal/db"
)

// ListWebhooks handles GET /api/v1/webhooks.
func (h *Handler) ListWebhooks(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, name, url, events, secret, enabled, last_status, last_fired, created_at
		FROM webhooks ORDER BY id`)
	if err != nil {
		fail(w, http.StatusInternalServerError, "query: "+err.Error())
		return
	}
	defer rows.Close()

	var hooks []db.Webhook
	for rows.Next() {
		var wh db.Webhook
		if err := rows.Scan(&wh.ID, &wh.Name, &wh.URL, &wh.Events, &wh.Secret,
			&wh.Enabled, &wh.LastStatus, &wh.LastFired, &wh.CreatedAt); err != nil {
			continue
		}
		hooks = append(hooks, wh)
	}
	ok(w, hooks)
}

// CreateWebhook handles POST /api/v1/webhooks.
func (h *Handler) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string `json:"name"`
		URL     string `json:"url"`
		Events  string `json:"events"`
		Secret  string `json:"secret"`
		Enabled bool   `json:"enabled"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Name == "" || req.URL == "" {
		fail(w, http.StatusBadRequest, "name and url are required")
		return
	}
	enabled := 0
	if req.Enabled {
		enabled = 1
	}
	res, err := h.db.ExecContext(r.Context(), `
		INSERT INTO webhooks (name, url, events, secret, enabled) VALUES (?,?,?,?,?)`,
		req.Name, req.URL, req.Events, req.Secret, enabled,
	)
	if err != nil {
		fail(w, http.StatusInternalServerError, "insert: "+err.Error())
		return
	}
	id, _ := res.LastInsertId()
	ok(w, map[string]int64{"id": id})
}

// GetWebhook handles GET /api/v1/webhooks/{id}.
func (h *Handler) GetWebhook(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(pathID(r, "id"))
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}
	var wh db.Webhook
	if err := h.db.QueryRowContext(r.Context(), `
		SELECT id, name, url, events, secret, enabled, last_status, last_fired, created_at
		FROM webhooks WHERE id=?`, id,
	).Scan(&wh.ID, &wh.Name, &wh.URL, &wh.Events, &wh.Secret,
		&wh.Enabled, &wh.LastStatus, &wh.LastFired, &wh.CreatedAt); err != nil {
		fail(w, http.StatusNotFound, "webhook not found")
		return
	}
	ok(w, wh)
}

// UpdateWebhook handles PUT /api/v1/webhooks/{id}.
func (h *Handler) UpdateWebhook(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(pathID(r, "id"))
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		Name    string `json:"name"`
		URL     string `json:"url"`
		Events  string `json:"events"`
		Secret  string `json:"secret"`
		Enabled bool   `json:"enabled"`
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
		UPDATE webhooks SET name=?, url=?, events=?, secret=?, enabled=? WHERE id=?`,
		req.Name, req.URL, req.Events, req.Secret, enabled, id); err != nil {
		fail(w, http.StatusInternalServerError, "update: "+err.Error())
		return
	}
	ok(w, map[string]string{"message": "updated"})
}

// DeleteWebhook handles DELETE /api/v1/webhooks/{id}.
func (h *Handler) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(pathID(r, "id"))
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}
	if _, err := h.db.ExecContext(r.Context(), `DELETE FROM webhooks WHERE id=?`, id); err != nil {
		fail(w, http.StatusInternalServerError, "delete: "+err.Error())
		return
	}
	ok(w, map[string]string{"message": "deleted"})
}

// TestWebhook handles POST /api/v1/webhooks/{id}/test.
func (h *Handler) TestWebhook(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(pathID(r, "id"))
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}
	if h.webhook == nil {
		fail(w, http.StatusServiceUnavailable, "webhook dispatcher not initialized")
		return
	}
	if err := h.webhook.TestWebhook(r.Context(), id); err != nil {
		fail(w, http.StatusBadGateway, "test failed: "+err.Error())
		return
	}
	ok(w, map[string]string{"message": "test delivered"})
}
