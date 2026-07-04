package control

import (
	"log"
	"net/http"
)

// registerChatRoutes wires the chat API. When a local OpenCode server is
// detected, all routes proxy to it (see chat_proxy.go). Otherwise the chat is
// unavailable and every endpoint returns a clear 503 so the UI can surface it.
func (s *Server) registerChatRoutes() {
	if url := detectOpenCodeURL(); url != "" {
		s.registerOpenCodeProxy(url)
		return
	}

	log.Printf("[chat] no OpenCode server detected — chat disabled (start `opencode serve` or set OPENCODE_URL)")
	unavailable := func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "OpenCode server not running", http.StatusServiceUnavailable)
	}
	s.mux.HandleFunc("/api/chat/", unavailable)
}
