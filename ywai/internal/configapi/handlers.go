package configapi

import (
	"encoding/json"
	"log"
	"net/http"
	"regexp"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/opencode"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/wshub"
	"github.com/gorilla/websocket"
)

// --- WebSocket Hub ---

// Hub manages WebSocket connections and broadcasts messages to all clients.
// It is the shared wshub.Hub; the MCP job manager broadcasts install events
// through it.
type Hub = wshub.Hub

// NewHub creates the config API hub: a running wshub.Hub with the config
// WebSocket's 512-byte read limit.
func NewHub() *Hub {
	return wshub.New(wshub.Options{ReadLimit: 512})
}

// --- Handlers ---

// Handlers holds references to the hub and clients for HTTP handlers.
type Handlers struct {
	hub            *Hub
	opencodeClient opencode.Client

	// toolCache memoizes the slow /api/config/tools assembly (MCP + plugin
	// discovery spawns subprocesses and hits the network) with a
	// stale-while-revalidate policy. The zero value is ready to use.
	toolCache toolCache
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for POC
	},
}

// --- WebSocket handler ---

func (h *Handlers) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := wshub.NewClient(h.hub, conn)
	h.hub.Register(client)
	client.Serve()
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("Error writing JSON response: %v", err)
	}
}

func isValidName(name string) bool {
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, name)
	return matched
}
