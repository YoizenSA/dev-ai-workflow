package web

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	wsWriteWait      = 10 * time.Second
	wsPongWait       = 60 * time.Second
	wsPingPeriod     = (wsPongWait * 9) / 10
	maxMessageSize   = 4096
	closeNormal      = 1000
	closeShutdown    = 1001
)

// Client represents a single WebSocket connection.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
	mu   sync.Mutex
}

// write sends bytes to the client's send channel.
func (c *Client) write(data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	select {
	case c.send <- data:
	default:
		// Buffer full, drop message
	}
}

// readPump reads messages from the WebSocket connection.
func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister(c)
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(wsPongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// writePump writes messages from the hub to the WebSocket connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(wsPingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				// Hub closed the channel
				_ = c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(closeShutdown, "server shutdown"))
				return
			}
			_ = c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Hub manages WebSocket connections and broadcasts messages to all clients.
type Hub struct {
	mu        sync.RWMutex
	clients   map[*Client]bool
	broadcast chan []byte
	done      chan struct{}
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	h := &Hub{
		clients:   make(map[*Client]bool),
		broadcast: make(chan []byte, 256),
		done:      make(chan struct{}),
	}
	go h.run()
	return h
}

// run starts the hub's event loop.
func (h *Hub) run() {
	for {
		select {
		case msg := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- msg:
				default:
					// Client's send buffer is full; drop it.
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		case <-h.done:
			return
		}
	}
}

// Broadcast enqueues a message to be sent to all connected clients.
func (h *Hub) Broadcast(msg []byte) {
	select {
	case h.broadcast <- msg:
	default:
		log.Println("WebSocket broadcast channel full, dropping message")
	}
}

// BroadcastEvent broadcasts a structured event to all clients.
func (h *Hub) BroadcastEvent(eventType string, payload interface{}) {
	msg, err := json.Marshal(map[string]interface{}{
		"type":    eventType,
		"payload": payload,
	})
	if err != nil {
		log.Printf("WebSocket broadcast marshal error: %v", err)
		return
	}
	h.Broadcast(msg)
}

// Register adds a client to the hub.
func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = true
}

// Unregister removes a client from the hub.
func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
}

// Shutdown stops the hub.
func (h *Hub) Shutdown() {
	select {
	case <-h.done:
	default:
		close(h.done)
	}
}
