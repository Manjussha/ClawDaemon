// Package webhook fires outbound webhook events to registered URLs.
package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Manjussha/clawdaemon/internal/db"
)

// Dispatcher fires webhooks stored in the database.
type Dispatcher struct {
	database *db.DB
	client   *http.Client
}

// New creates a Dispatcher with a default HTTP client.
func New(database *db.DB) *Dispatcher {
	return &Dispatcher{
		database: database,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

// Payload is the JSON body sent to webhook URLs.
type Payload struct {
	Event     string      `json:"event"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// Fire sends an event to all matching enabled webhook URLs.
// Retries 3x with exponential backoff (500ms, 1s, 2s).
func (d *Dispatcher) Fire(event string, data interface{}) {
	rows, err := d.database.Query(
		`SELECT id, url FROM webhooks WHERE enabled=1`)
	if err != nil {
		log.Printf("webhook.Fire: query: %v", err)
		return
	}
	defer rows.Close()

	payload := Payload{Event: event, Timestamp: time.Now(), Data: data}
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("webhook.Fire: marshal: %v", err)
		return
	}

	for rows.Next() {
		var id int
		var url string
		if err := rows.Scan(&id, &url); err != nil {
			continue
		}
		// Filter by event subscription stored in DB.
		var events string
		_ = d.database.QueryRow(`SELECT events FROM webhooks WHERE id=?`, id).Scan(&events)
		if events != "" && !matchesEvent(events, event) {
			continue
		}
		go d.fireOne(id, url, body)
	}
}

func (d *Dispatcher) fireOne(id int, url string, body []byte) {
	delays := []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second}
	var lastStatus int
	for i, delay := range delays {
		if i > 0 {
			time.Sleep(delay)
		}
		status, err := d.post(url, body)
		lastStatus = status
		if err == nil && status < 400 {
			break
		}
		log.Printf("webhook.fireOne: attempt %d to %s: status=%d err=%v", i+1, url, status, err)
	}
	_, _ = d.database.Exec(
		`UPDATE webhooks SET last_status=?, last_fired=? WHERE id=?`,
		lastStatus, time.Now(), id,
	)
}

func (d *Dispatcher) post(url string, body []byte) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("webhook.post: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("webhook.post: do: %w", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

func matchesEvent(events, event string) bool {
	for _, e := range strings.Split(events, ",") {
		if strings.TrimSpace(e) == event {
			return true
		}
	}
	return false
}

// TestWebhook fires a test payload to a single webhook by ID.
func (d *Dispatcher) TestWebhook(ctx context.Context, id int) error {
	var url string
	if err := d.database.QueryRowContext(ctx,
		`SELECT url FROM webhooks WHERE id=?`, id).Scan(&url); err != nil {
		return fmt.Errorf("webhook.TestWebhook: %w", err)
	}
	body, _ := json.Marshal(Payload{
		Event:     "webhook.test",
		Timestamp: time.Now(),
		Data:      map[string]string{"message": "This is a test from ClawDaemon"},
	})
	status, err := d.post(url, body)
	if err != nil {
		return fmt.Errorf("webhook.TestWebhook: post: %w", err)
	}
	if status >= 400 {
		return fmt.Errorf("webhook.TestWebhook: server returned %d", status)
	}
	return nil
}