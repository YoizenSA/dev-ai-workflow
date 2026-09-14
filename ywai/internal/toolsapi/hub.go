package toolsapi

import (
	"net/http"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/wshub"
)

// Hub is the Engram/consolidation WebSocket fan-out hub.
type Hub = wshub.Hub

// NewHub creates the tool API hub: a running wshub.Hub with the Engram
// WebSocket's 4 KiB read limit.
func NewHub() *Hub {
	return wshub.New(wshub.Options{ReadLimit: 4096})
}

// serveEngramWS upgrades the request and attaches the connection to h for
// the lifetime of the WebSocket.
func serveEngramWS(h *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := wshub.NewClient(h, conn)
	h.Register(client)
	client.Serve()
}
