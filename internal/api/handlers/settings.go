package handlers

import "net/http"

// ListSettings handles GET /api/v1/settings.
func (h *Handler) ListSettings(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `SELECT key, value FROM settings WHERE key != 'schema_version' ORDER BY key`)
	if err != nil {
		fail(w, http.StatusInternalServerError, "query: "+err.Error())
		return
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			continue
		}
		settings[k] = v
	}
	ok(w, settings)
}

// UpdateSetting handles PUT /api/v1/settings/{key}.
func (h *Handler) UpdateSetting(w http.ResponseWriter, r *http.Request) {
	key := pathID(r, "key")
	if key == "" || key == "schema_version" {
		fail(w, http.StatusBadRequest, "invalid key")
		return
	}
	var req struct {
		Value string `json:"value"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := h.db.SetSetting(key, req.Value); err != nil {
		fail(w, http.StatusInternalServerError, "set: "+err.Error())
		return
	}
	ok(w, map[string]string{"key": key, "value": req.Value})
}
