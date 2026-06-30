package control

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Workflow events streamed to the UI during a run. Mirrors the missions hub's
// BroadcastEvent shape: each message is {"type": <event>, "payload": <data>}.
const (
	eventRunStarted = "workflow_run_started" // payload: RunStartedEvent
	eventRunOutput  = "workflow_run_output"  // payload: RunOutputEvent (one line/chunk)
	eventRunDone    = "workflow_run_done"    // payload: RunDoneEvent
)

// RunStartedEvent is sent when a workflow run begins.
type RunStartedEvent struct {
	Workflow string `json:"workflow"`
	RunID    string `json:"runId"`
}

// RunOutputEvent carries one chunk of the orchestrator's stdout/stderr.
type RunOutputEvent struct {
	Workflow string `json:"workflow"`
	RunID    string `json:"runId"`
	Stream   string `json:"stream"` // "stdout" | "stderr"
	Text     string `json:"text"`
}

// RunDoneEvent is sent when a run finishes (success or failure).
type RunDoneEvent struct {
	Workflow string `json:"workflow"`
	RunID    string `json:"runId"`
	ExitCode int    `json:"exitCode"`
	Error    string `json:"error,omitempty"`
}

// wsHub mirrors missions/web/hub.go: a fan-out broadcaster for workflow run
// events. Kept small and self-contained in the control package so workflow
// streaming stays isolated from the missions/kanban hubs.
type wsHub struct {
	mu        sync.RWMutex
	clients   map[*wsClient]bool
	broadcast chan []byte
	done      chan struct{}
}

const (
	wfWsWriteWait     = 10 * time.Second
	wfWsPongWait      = 60 * time.Second
	wfWsPingPeriod    = (wfWsPongWait * 9) / 10
	wfWsMaxMessage    = 4096
	wfWsCloseShutdown = 1001
)

func newWsHub() *wsHub {
	h := &wsHub{
		clients:   make(map[*wsClient]bool),
		broadcast: make(chan []byte, 256),
		done:      make(chan struct{}),
	}
	go h.run()
	return h
}

func (h *wsHub) run() {
	for {
		select {
		case msg := <-h.broadcast:
			h.mu.RLock()
			for c := range h.clients {
				select {
				case c.send <- msg:
				default:
					// Buffer full: drop the client.
					close(c.send)
					delete(h.clients, c)
				}
			}
			h.mu.RUnlock()
		case <-h.done:
			return
		}
	}
}

func (h *wsHub) broadcastEvent(eventType string, payload any) {
	if h == nil {
		return
	}
	msg, err := json.Marshal(map[string]any{"type": eventType, "payload": payload})
	if err != nil {
		log.Printf("workflow hub marshal error: %v", err)
		return
	}
	select {
	case h.broadcast <- msg:
	default:
		log.Println("workflow hub broadcast channel full, dropping message")
	}
}

func (h *wsHub) register(c *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = true
}

func (h *wsHub) unregister(c *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
}

func (h *wsHub) shutdown() {
	select {
	case <-h.done:
	default:
		close(h.done)
	}
}

// wsClient is one WebSocket connection subscribed to workflow run events.
type wsClient struct {
	hub  *wsHub
	conn *websocket.Conn
	send chan []byte
}

func (c *wsClient) readPump() {
	defer func() {
		c.hub.unregister(c)
		_ = c.conn.Close()
	}()
	c.conn.SetReadLimit(wfWsMaxMessage)
	_ = c.conn.SetReadDeadline(time.Now().Add(wfWsPongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(wfWsPongWait))
		return nil
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (c *wsClient) writePump() {
	ticker := time.NewTicker(wfWsPingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(wfWsWriteWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(wfWsCloseShutdown, "server shutdown"))
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(wfWsWriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// upgrader is the WebSocket upgrader for the workflows endpoint. Same limits as
// missions/web's upgrader.
var wfUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}
