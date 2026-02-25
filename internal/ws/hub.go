// Package ws provides the WebSocket hub for streaming live updates to the dashboard.
package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WSMessage is the envelope for all WebSocket messages.
type WSMessage struct {
	Type      string      `json:"type"`
	WorkerID  int         `json:"worker_id,omitempty"`
	TaskID    int         `json:"task_id,omitempty"`
	Level     string      `json:"level,omitempty"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// Message type constants.
const (
	TypeLogLine      = "log_line"
	TypeWorkerStatus = "worker_status"
	TypeTaskUpdate   = "task_update"
	TypeTaskComplete = "task_complete"
	TypeRateLimit    = "rate_limit"
	TypeSystemStatus = "system_status"
	TypeBudgetWarn   = "budget_warning"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type client struct {
	conn *websocket.Conn
	send chan []byte
}

// Hub manages all connected WebSocket clients.
type Hub struct {
	mu         sync.RWMutex
	clients    map[*client]struct{}
	broadcast  chan []byte
	register   chan *client
	unregister chan *client
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*client]struct{}),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *client, 8),
		unregister: make(chan *client, 8),
	}
}

// Run starts the hub event loop. Must be run in a goroutine.
func (h *Hub) Run(ctx interface{ Done() <-chan struct{} }) {
	for {
		select {
		case <-ctx.Done():
			return
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = struct{}{}
			h.mu.Unlock()
		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}
			h.mu.Unlock()
		case msg := <-h.broadcast:
			h.mu.RLock()
			for c := range h.clients {
				select {
				case c.send <- msg:
				default:
					// Drop slow clients.
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast sends a WSMessage to all connected clients.
func (h *Hub) Broadcast(msg WSMessage) {
	msg.Timestamp = time.Now()
	b, err := json.Marshal(msg)
	if err != nil {
		return
	}
	select {
	case h.broadcast <- b:
	default:
	}
}

// BroadcastToWorker sends a log_line message for a specific worker.
func (h *Hub) BroadcastToWorker(workerID, taskID int, line, level string) {
	h.Broadcast(WSMessage{
		Type:     TypeLogLine,
		WorkerID: workerID,
		TaskID:   taskID,
		Level:    level,
		Message:  line,
	})
}

// ServeWS handles the WebSocket upgrade and starts pump goroutines.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws.ServeWS: upgrade: %v", err)
		return
	}
	c := &client{conn: conn, send: make(chan []byte, 64)}
	h.register <- c
	go h.writePump(c)
	go h.readPump(c)
}

func (h *Hub) writePump(c *client) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (h *Hub) readPump(c *client) {
	defer func() {
		h.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(512)
	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})
	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
	}
}

// ClientCount returns the number of connected WebSocket clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

