// Package wshub implements the WebSocket fan-out hub shared by the tool API,
// workflow-run streaming, and config API servers. It registers client
// connections, runs per-connection read/write pumps with ping/pong
// keepalive, and broadcasts JSON events to every connected client.
package wshub

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// writeWait bounds each write: data messages and pings alike.
	writeWait = 10 * time.Second
	// pongWait is how long a connection may go without a pong before the
	// read side drops it.
	pongWait = 60 * time.Second
	// pingPeriod must be shorter than pongWait, so each ping resets the
	// peer's read deadline before it expires.
	pingPeriod = (pongWait * 9) / 10

	// sendBuffer sizes the hub broadcast queue and each client send queue.
	sendBuffer = 256

	// closeCode is the status code sent when the hub drops a client.
	closeCode = 1001 // going away
)

// Options tunes the per-server limits a hub enforces.
type Options struct {
	// ReadLimit caps the size of messages a client may send; 0 means 4096.
	ReadLimit int64
}

// Hub fans every broadcast out to all registered clients. Create one with
// New; the event loop starts immediately.
type Hub struct {
	mu        sync.RWMutex
	clients   map[*Client]bool
	broadcast chan []byte
	readLimit int64
	loopOnce  sync.Once
}

// New creates a hub and starts its event loop.
func New(opts Options) *Hub {
	h := &Hub{
		clients:   make(map[*Client]bool),
		broadcast: make(chan []byte, sendBuffer),
		readLimit: opts.ReadLimit,
	}
	if h.readLimit <= 0 {
		h.readLimit = 4096
	}
	h.Run()
	return h
}

// Run starts the hub event loop if no loop is running yet. New already
// starts one, so calling Run on a fresh hub does nothing. It exists for
// callers that receive a hub and want to guarantee a loop is running.
func (h *Hub) Run() {
	h.loopOnce.Do(func() {
		go h.loop()
	})
}

// loop drains the broadcast queue and fans each message out to every
// client. It exits when the broadcast channel closes.
func (h *Hub) loop() {
	for msg := range h.broadcast {
		h.mu.RLock()
		for c := range h.clients {
			select {
			case c.send <- msg:
			default:
				// Client's send buffer is full; drop it.
				close(c.send)
				delete(h.clients, c)
			}
		}
		h.mu.RUnlock()
	}
}

// Broadcast enqueues msg for every connected client. It never blocks: if
// the hub queue is full the message is dropped.
func (h *Hub) Broadcast(msg []byte) {
	select {
	case h.broadcast <- msg:
	default:
		log.Println("wshub: broadcast queue full, dropping message")
	}
}

// BroadcastEvent marshals {"type": eventType, "payload": payload} and
// broadcasts it. It is safe to call on a nil hub, which drops the event.
func (h *Hub) BroadcastEvent(eventType string, payload any) {
	if h == nil {
		return
	}
	msg, err := json.Marshal(map[string]any{"type": eventType, "payload": payload})
	if err != nil {
		log.Printf("wshub: event marshal error: %v", err)
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

// Unregister removes a client from the hub and closes its send queue.
func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
}

// Client is a single WebSocket connection subscribed to hub broadcasts.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

// NewClient wraps conn for delivery through h. Register the client with
// h.Register and drive it with Serve.
func NewClient(h *Hub, conn *websocket.Conn) *Client {
	return &Client{hub: h, conn: conn, send: make(chan []byte, sendBuffer)}
}

// Serve runs the connection until the client disconnects: the write pump
// on its own goroutine, the read pump on the caller's.
func (c *Client) Serve() {
	go c.writePump()
	c.readPump()
}

// readPump drains incoming messages (clients never send data) and
// unregisters the client when the connection dies.
func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister(c)
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(c.hub.readLimit)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

// writePump delivers queued broadcasts and periodic pings. It closes the
// connection when the hub drops the client or a write fails.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(closeCode, "server shutdown"))
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
