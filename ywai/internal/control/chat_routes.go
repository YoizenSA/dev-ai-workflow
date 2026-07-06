package control

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// registerChatRoutes wires the chat API. When a local OpenCode server is
// detected at startup, all routes proxy to it (see chat_proxy.go). Otherwise
// the chat is unavailable and every endpoint returns a clear 503 so the UI can
// surface it.
//
// When ywai starts before opencode, the catch-all handler below re-detects the
// opencode URL on every request. The UI's "Start OpenCode" button spawns
// `opencode serve` on a dynamic port and sets OPENCODE_URL; once the handler
// finds it, it builds a ChatProxy lazily and routes all /api/chat/* requests
// through it — no ywai restart needed.
func (s *Server) registerChatRoutes() {
	// ywai-local endpoints (pins, prompt templates) work with or without OpenCode.
	s.registerChatLocalStores()

	if url := detectOpenCodeURL(); url != "" {
		s.registerOpenCodeProxy(url)
		return
	}

	log.Printf("[chat] no OpenCode server detected — chat disabled (start `opencode serve` or set OPENCODE_URL)")

	var (
		mu    sync.Mutex
		proxy *ChatProxy
	)
	// Catch-all: resolves the proxy lazily on first successful detection, then
	// dispatches every /api/chat/* + /api/files request through it.
	handler := func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		if proxy == nil {
			opencodeURL := detectOpenCodeURL()
			piURL := detectPiURL()

			if opencodeURL == "" && piURL == "" {
				mu.Unlock()
				log.Println("[chat] no OpenCode or PI.dev server detected — chat disabled")
				http.Error(w, "no chat server available", http.StatusServiceUnavailable)
				return
			}

			proxy = NewChatProxy(opencodeURL)
			if piURL != "" {
				proxy.piBaseURL = piURL
				log.Printf("[chat] PI.dev server detected at %s", piURL)
			}
			if opencodeURL != "" {
				log.Printf("[chat] OpenCode server detected at %s", opencodeURL)
			}
		}
		p := proxy
		mu.Unlock()

		if p == nil {
			http.Error(w, "OpenCode server not running", http.StatusServiceUnavailable)
			return
		}
		dispatchChat(p, w, r)
	}
	s.mux.HandleFunc("/api/chat/", handler)
	s.mux.HandleFunc("/api/files", handler)
}

// dispatchChat routes a request to the right ChatProxy method based on method +
// path, extracting {id}/{messageID}/{permissionID}/{requestID} path params via
// r.SetPathValue so the handlers' r.PathValue() calls work without the mux's
// pattern matcher.
func dispatchChat(cp *ChatProxy, w http.ResponseWriter, r *http.Request) {
	m := r.Method
	rawPath := r.URL.Path

	// /api/files → handleFileList (note: no /chat prefix)
	if rawPath == "/api/files" && m == "GET" {
		cp.handleFileList(w, r)
		return
	}

	// Everything else is under /api/chat/.
	p := strings.TrimPrefix(rawPath, "/api/chat")
	switch {
	case p == "/sessions" && m == "GET":
		cp.handleListSessions(w, r)
	case p == "/sessions" && m == "POST":
		cp.handleCreateSession(w, r)
	case p == "/events" && m == "GET":
		cp.handleChatSSE(w, r)
	case p == "/abort" && m == "POST":
		cp.handleAbort(w, r)
	case p == "/providers" && m == "GET":
		cp.handleProviders(w, r)
	case p == "/agents" && m == "GET":
		cp.handleAgents(w, r)
	case p == "/projects" && m == "GET":
		cp.handleProjects(w, r)
	case p == "/gitdiff" && m == "GET":
		cp.handleGitDiff(w, r)
	case p == "/target" && m == "GET":
		cp.handleGetTarget(w, r)
	case p == "/target" && m == "POST":
		cp.handleSetTarget(w, r)
	case p == "/status" && m == "GET":
		cp.handleStatus(w, r)
	case strings.HasPrefix(p, "/sessions/"):
		dispatchSessionRoute(cp, w, r, strings.TrimPrefix(p, "/sessions/"), m)
	default:
		http.NotFound(w, r)
	}
}

// dispatchSessionRoute handles /api/chat/sessions/{id}[/tail].
func dispatchSessionRoute(cp *ChatProxy, w http.ResponseWriter, r *http.Request, rest, m string) {
	parts := strings.SplitN(rest, "/", 2)
	id := parts[0]
	tail := ""
	if len(parts) == 2 {
		tail = parts[1]
	}
	r.SetPathValue("id", id)

	switch {
	case tail == "" && m == "GET":
		cp.handleGetMessages(w, r)
	case tail == "" && m == "PATCH":
		cp.handleRenameSession(w, r)
	case tail == "" && m == "DELETE":
		cp.handleDeleteSession(w, r)
	case tail == "info" && m == "GET":
		cp.handleSessionInfo(w, r)
	case tail == "children" && m == "GET":
		cp.handleChildren(w, r)
	case tail == "messages" && m == "POST":
		cp.handleSendMessage(w, r)
	case tail == "context" && m == "GET":
		cp.handleSessionContext(w, r)
	case tail == "todo" && m == "GET":
		cp.handleTodo(w, r)
	case tail == "diff" && m == "GET":
		cp.handleDiff(w, r)
	case tail == "revert" && m == "POST":
		cp.handleRevert(w, r)
	case tail == "command" && m == "POST":
		cp.handleCommand(w, r)
	case strings.HasPrefix(tail, "message/"):
		r.SetPathValue("messageID", strings.TrimPrefix(tail, "message/"))
		if m == "DELETE" {
			cp.handleDeleteMessage(w, r)
		}
	case strings.HasPrefix(tail, "permissions/"):
		r.SetPathValue("permissionID", strings.TrimPrefix(tail, "permissions/"))
		if m == "POST" {
			cp.handlePermissionReply(w, r)
		}
	case strings.HasPrefix(tail, "question/"):
		qRest := strings.TrimPrefix(tail, "question/")
		qParts := strings.SplitN(qRest, "/", 2)
		if len(qParts) == 2 {
			r.SetPathValue("requestID", qParts[0])
			switch {
			case qParts[1] == "reply" && m == "POST":
				cp.handleQuestionReply(w, r)
			case qParts[1] == "reject" && m == "POST":
				cp.handleQuestionReject(w, r)
			}
		}
	default:
		http.NotFound(w, r)
	}
}

// detectPiURL tries to find a running PI.dev server.
func detectPiURL() string {
	// Check env var first
	if u := strings.TrimSpace(os.Getenv("PI_URL")); u != "" {
		return strings.TrimRight(u, "/")
	}

	// Try common PI.dev ports
	ports := []int{5173, 3000, 4000, 8080}
	client := &http.Client{Timeout: 1 * time.Second}
	for _, port := range ports {
		url := fmt.Sprintf("http://localhost:%d", port)
		resp, err := client.Get(url + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return url
			}
		}
		// Also try /api/health
		resp, err = client.Get(url + "/api/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return url
			}
		}
	}
	return ""
}

// handleStatus returns connection status for the frontend.
func (cp *ChatProxy) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"target":    cp.Target(),
		"opencode":  cp.opencodeBaseURL != "",
		"pi":        cp.piBaseURL != "",
		"connected": cp.opencodeBaseURL != "" || cp.piBaseURL != "",
	})
}
