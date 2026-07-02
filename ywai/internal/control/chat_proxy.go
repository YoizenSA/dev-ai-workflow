package control

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// ChatProxy proxies SSE connections to a local OpenCode server.
type ChatProxy struct {
	opencodeBaseURL string // e.g. "http://localhost:3000"
}

func NewChatProxy(opencodeBaseURL string) *ChatProxy {
	return &ChatProxy{opencodeBaseURL: opencodeBaseURL}
}

// handleChatSSE proxies the SSE stream from OpenCode to the client.
// GET /api/chat/events?sessionID=xxx
func (cp *ChatProxy) handleChatSSE(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("sessionID")
	if sessionID == "" {
		http.Error(w, "sessionID required", http.StatusBadRequest)
		return
	}

	upstreamURL := fmt.Sprintf("%s/event", cp.opencodeBaseURL)
	req, err := http.NewRequestWithContext(r.Context(), "GET", upstreamURL, nil)
	if err != nil {
		http.Error(w, "failed to create request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	client := &http.Client{Timeout: 0} // no timeout for SSE
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("upstream error: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("upstream returned %d", resp.StatusCode), resp.StatusCode)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer

	for scanner.Scan() {
		line := scanner.Text()

		// Filter events for the requested session
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if cp.shouldForwardEvent(data, sessionID) {
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
		} else if strings.HasPrefix(line, "event: ") || line == "" {
			fmt.Fprintf(w, "%s\n", line)
			flusher.Flush()
		}
	}
}

// shouldForwardEvent checks if an SSE event belongs to the requested session.
func (cp *ChatProxy) shouldForwardEvent(data, sessionID string) bool {
	// Try to parse as JSON and check sessionID
	var event struct {
		SessionID string `json:"sessionID"`
		Type      string `json:"type"`
	}
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		// If we can't parse, forward it (might be a global event)
		return true
	}

	// Forward global events (no sessionID)
	if event.SessionID == "" {
		return true
	}

	return event.SessionID == sessionID
}

// handleChatSend proxies a message send to OpenCode.
// POST /api/chat/send
func (cp *ChatProxy) handleChatSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	upstreamURL := fmt.Sprintf("%s/session.message", cp.opencodeBaseURL)
	req, err := http.NewRequestWithContext(r.Context(), "POST", upstreamURL, bytes.NewReader(body))
	if err != nil {
		http.Error(w, "failed to create request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("upstream error: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Forward response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// handleChatSessions proxies session listing to OpenCode.
// GET /api/chat/sessions
func (cp *ChatProxy) handleChatSessions(w http.ResponseWriter, r *http.Request) {
	upstreamURL := fmt.Sprintf("%s/session.list", cp.opencodeBaseURL)
	req, err := http.NewRequestWithContext(r.Context(), "GET", upstreamURL, nil)
	if err != nil {
		http.Error(w, "failed to create request", http.StatusInternalServerError)
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("upstream error: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// handleChatCreateSession creates a new session on OpenCode.
// POST /api/chat/sessions
func (cp *ChatProxy) handleChatCreateSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	upstreamURL := fmt.Sprintf("%s/session.create", cp.opencodeBaseURL)
	req, err := http.NewRequestWithContext(r.Context(), "POST", upstreamURL, bytes.NewReader(body))
	if err != nil {
		http.Error(w, "failed to create request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("upstream error: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// handleChatMessages proxies message listing to OpenCode.
// GET /api/chat/messages?sessionID=xxx
func (cp *ChatProxy) handleChatMessages(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("sessionID")
	if sessionID == "" {
		http.Error(w, "sessionID required", http.StatusBadRequest)
		return
	}

	upstreamURL := fmt.Sprintf("%s/session.messages?id=%s", cp.opencodeBaseURL, sessionID)
	req, err := http.NewRequestWithContext(r.Context(), "GET", upstreamURL, nil)
	if err != nil {
		http.Error(w, "failed to create request", http.StatusInternalServerError)
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("upstream error: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// handleChatAbort aborts a running session.
// POST /api/chat/abort
func (cp *ChatProxy) handleChatAbort(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	upstreamURL := fmt.Sprintf("%s/session.abort", cp.opencodeBaseURL)
	req, err := http.NewRequestWithContext(r.Context(), "POST", upstreamURL, bytes.NewReader(body))
	if err != nil {
		http.Error(w, "failed to create request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("upstream error: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// RegisterChatRoutes registers all chat proxy routes on the mux.
func (s *Server) registerChatRoutes() {
	// Auto-detect OpenCode server URL
	opencodeURL := detectOpenCodeURL()
	if opencodeURL == "" {
		log.Printf("[chat] no OpenCode server detected, chat disabled")
		return
	}

	cp := NewChatProxy(opencodeURL)
	log.Printf("[chat] proxying to OpenCode at %s", opencodeURL)

	s.mux.HandleFunc("GET /api/chat/events", cp.handleChatSSE)
	s.mux.HandleFunc("GET /api/chat/sessions", cp.handleChatSessions)
	s.mux.HandleFunc("POST /api/chat/sessions", cp.handleChatCreateSession)
	s.mux.HandleFunc("GET /api/chat/messages", cp.handleChatMessages)
	s.mux.HandleFunc("POST /api/chat/send", cp.handleChatSend)
	s.mux.HandleFunc("POST /api/chat/abort", cp.handleChatAbort)
}

// detectOpenCodeURL tries to find a running OpenCode server.
func detectOpenCodeURL() string {
	// Try common ports
	ports := []string{"3000", "3001", "4096"}
	for _, port := range ports {
		url := fmt.Sprintf("http://localhost:%s", port)
		client := &http.Client{Timeout: 1 * time.Second}
		resp, err := client.Get(url + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return url
			}
		}
	}

	// Check environment variable
	if url := strings.TrimSpace(getenvFallback("OPENCODE_URL")); url != "" {
		return url
	}

	return ""
}

func getenvFallback(key string) string {
	return os.Getenv(key)
}
