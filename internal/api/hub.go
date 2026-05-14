package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// Event is the envelope sent to all connected WebSocket clients.
type Event struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

const (
	EventSpanCreated    = "span.created"
	EventSessionUpdated = "session.updated"
)

// Hub manages the set of active WebSocket clients and broadcasts events to them.
type Hub struct {
	mu      sync.RWMutex
	clients map[*wsClient]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: make(map[*wsClient]struct{})}
}

// Broadcast sends an event to every connected client. Non-blocking: slow
// clients are dropped rather than letting them stall the ingest path.
func (h *Hub) Broadcast(eventType string, payload any) {
	msg, err := json.Marshal(Event{Type: eventType, Payload: payload})
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- msg:
		default:
			// client too slow — will be cleaned up by its write pump
		}
	}
}

func (h *Hub) add(c *wsClient) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) remove(c *wsClient) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
}

// ─── WebSocket client ─────────────────────────────────────────────────────────

type wsClient struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // dev: allow all origins
}

// ServeWS upgrades the HTTP connection and starts the client pumps.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("ws upgrade", "err", err)
		return
	}

	c := &wsClient{hub: h, conn: conn, send: make(chan []byte, 32)}
	h.add(c)

	go c.writePump()
	go c.readPump()
}

// readPump drains incoming frames and cleans up when the connection closes.
func (c *wsClient) readPump() {
	defer func() {
		c.hub.remove(c)
		c.conn.Close()
	}()
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			break
		}
	}
}

// writePump fans out buffered messages to the client.
func (c *wsClient) writePump() {
	defer c.conn.Close()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			break
		}
	}
}
