package handlers

import (
	"net/http"
	"time"
)

type usageRow struct {
	Date         string `json:"date"`
	WorkerID     *int64 `json:"worker_id,omitempty"`
	ProjectID    *int64 `json:"project_id,omitempty"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	TotalTokens  int    `json:"total_tokens"`
}

// GetUsage handles GET /api/v1/usage.
// Query params: period=daily|weekly|monthly, worker_id, project_id.
func (h *Handler) GetUsage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	period := q.Get("period")
	if period == "" {
		period = "daily"
	}

	var since string
	now := time.Now()
	switch period {
	case "weekly":
		since = now.AddDate(0, 0, -7).Format("2006-01-02")
	case "monthly":
		since = now.AddDate(0, -1, 0).Format("2006-01-02")
	default:
		since = now.Format("2006-01-02")
	}

	query := `SELECT date, worker_id, project_id,
		SUM(input_tokens), SUM(output_tokens), SUM(input_tokens+output_tokens)
		FROM token_usage WHERE date >= ?`
	args := []interface{}{since}

	if v := q.Get("worker_id"); v != "" {
		query += " AND worker_id=?"
		args = append(args, v)
	}
	if v := q.Get("project_id"); v != "" {
		query += " AND project_id=?"
		args = append(args, v)
	}
	query += " GROUP BY date, worker_id, project_id ORDER BY date DESC"

	rows, err := h.db.QueryContext(ctx, query, args...)
	if err != nil {
		fail(w, http.StatusInternalServerError, "query: "+err.Error())
		return
	}
	defer rows.Close()

	var results []usageRow
	for rows.Next() {
		var u usageRow
		if err := rows.Scan(&u.Date, &u.WorkerID, &u.ProjectID,
			&u.InputTokens, &u.OutputTokens, &u.TotalTokens); err != nil {
			continue
		}
		results = append(results, u)
	}
	ok(w, map[string]interface{}{
		"period": period,
		"since":  since,
		"rows":   results,
	})
}
