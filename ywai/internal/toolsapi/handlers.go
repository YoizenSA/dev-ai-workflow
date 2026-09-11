package toolsapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/opencode"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/serverutil"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

// WarmModels seeds memory from disk (instant) then kicks a background CLI
// refresh when the list is missing or stale. Safe to call more than once.
func (h *Handlers) WarmModels() {
	if h == nil || h.opencodeClient == nil {
		return
	}
	h.modelCache.seedFromDisk()

	h.modelCache.mu.Lock()
	fresh := !h.modelCache.fetchedAt.IsZero() &&
		len(h.modelCache.models) > 0 &&
		time.Since(h.modelCache.fetchedAt) < modelCacheTTL
	h.modelCache.mu.Unlock()
	if fresh {
		return
	}
	h.modelCache.kickRefresh(h.opencodeClient.ListModels)
}

// —— Refine Goal ————————————————————————————————————————————————

// RefineGoal uses the opencode CLI (not the HTTP server, which has known issues
// processing prompts via REST) to refine a user goal into a structured
// description with scope, out-of-scope, and acceptance criteria.
func (h *Handlers) RefineGoal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Goal    string `json:"goal"`
		Context string `json:"context,omitempty"`
		Model   string `json:"model,omitempty"`
		Agent   string `json:"agent,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if strings.TrimSpace(req.Goal) == "" {
		writeError(w, http.StatusBadRequest, "goal is required")
		return
	}

	refined := RefineGoalWithOpencode(req.Goal, req.Context, req.Model, req.Agent)
	if refined == "" {
		writeError(w, http.StatusInternalServerError, "no response from AI")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"refined": refined,
	})
}

// —— OpenCode Config —————————————————————————————————————————————

// ListModels returns available opencode models from config.
// Pass ?refresh=1 to force a background re-fetch of the CLI catalog even when
// the in-memory cache is still within TTL (Settings open uses this so the UI
// always revalidates without blocking on the multi-second CLI).
func (h *Handlers) ListModels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.URL.Query().Get("refresh") == "1" {
		h.modelCache.kickRefresh(h.opencodeClient.ListModels)
	}
	models, err := h.modelCache.get(ctx, h.opencodeClient.ListModels)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"models": []string{}})
		return
	}

	// Get connected providers from opencode status
	status, err := h.opencodeClient.Status(ctx)
	connectedProviders := make(map[string]bool)
	if err == nil && status.ConnectedProviders != nil {
		for _, p := range status.ConnectedProviders {
			connectedProviders[p] = true
		}
	}

	// Group models by provider. When the opencode server is reachable, filter by
	// connected providers so the list only shows usable models. When the server
	// is down (LocalClient), connectedProviders is empty — in that case show all
	// models rather than none, since the CLI can still use them.
	modelsByProvider := make(map[string][]map[string]interface{})
	for _, m := range models {
		if len(connectedProviders) > 0 && m.Provider != "" && !connectedProviders[m.Provider] {
			continue
		}
		provider := m.Provider
		if provider == "" {
			provider = "default"
		}
		modelsByProvider[provider] = append(modelsByProvider[provider], map[string]interface{}{
			"id":       m.ID,
			"name":     m.Name,
			"provider": m.Provider,
		})
	}

	var defaultModel string
	if len(models) > 0 {
		defaultModel = models[0].ID
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"modelsByProvider": modelsByProvider,
		"default":          defaultModel,
	})
}

// ListAgents returns available opencode agent profiles.
func (h *Handlers) ListAgents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	agents, err := h.opencodeClient.ListAgents(ctx)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"agents": []string{}})
		return
	}

	ids := make([]string, len(agents))
	for i, a := range agents {
		ids[i] = a.ID
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"agents": ids,
	})
}

// OpenCodeStatus returns the opencode server connection status.
func (h *Handlers) OpenCodeStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	status, err := h.opencodeClient.Status(ctx)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"connected": false,
			"error":     err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// StartOpencode starts the opencode server if not already running.
func (h *Handlers) StartOpencode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// "Already running" means the opencode SERVER is reachable and serving
	// sessions. A LocalClient reports Connected=true just because opencode.json
	// exists, but it does not support sessions — so we must require Source=="server".
	status, err := h.opencodeClient.Status(ctx)
	if err == nil && status.Connected && status.Source == "server" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":  "already_running",
			"message": "opencode server is already connected",
		})
		return
	}

	// Resolve the opencode binary (PATH, well-known dirs, login-shell which)
	// so binaries installed via nvm/asdf/etc. are found even though the ywai
	// process does not load the user's shell profile.
	binPath, _ := agent.FindOpenCode()
	if binPath == "" {
		writeError(w, http.StatusInternalServerError,
			"opencode binary not found. Install opencode or start it manually, then retry.")
		return
	}

	// Determine the starting port from OPENCODE_URL (default 4096) and find a
	// free one if it's busy. Another server on 4096 would make
	// opencode fail to bind silently, leaving the UI stuck on "Starting…".
	startPort := 4096
	if u := os.Getenv("OPENCODE_URL"); u != "" {
		host := strings.TrimPrefix(strings.TrimPrefix(u, "http://"), "https://")
		if _, p, e := net.SplitHostPort(host); e == nil && p != "" {
			if n, e := strconv.Atoi(p); e == nil {
				startPort = n
			}
		}
	}
	// Reuse an already-running opencode server before spawning a new one, so
	// restarts of ywai don't accumulate orphan instances (one per restart).
	// Probe /status AND /app so a non-opencode server squatting a port (e.g.
	// Kilo on 4096) isn't mistaken for one.
	for p := startPort; p < startPort+20; p++ {
		candidate := "http://127.0.0.1:" + strconv.Itoa(p)
		pctx, pcancel := context.WithTimeout(ctx, 500*time.Millisecond)
		ok, _ := opencode.ProbeServer(pctx, candidate)
		if ok {
			req, _ := http.NewRequestWithContext(pctx, http.MethodGet, candidate+"/app", nil)
			resp, err := (&http.Client{Timeout: 500 * time.Millisecond}).Do(req)
			if err != nil || resp.StatusCode != http.StatusOK {
				ok = false
			}
			if resp != nil {
				resp.Body.Close()
			}
		}
		pcancel()
		if ok {
			os.Setenv("OPENCODE_URL", candidate)
			if h.trySwitchToServerClient(ctx) {
				log.Printf("opencode start: reusing running server at %s", candidate)
				writeJSON(w, http.StatusOK, map[string]interface{}{
					"status":  "already_running",
					"message": "reusing running opencode server",
					"url":     candidate,
				})
				return
			}
		}
	}

	port := serverutil.FindFreePort(startPort)
	if port != startPort {
		log.Printf("opencode start: port %d busy, using %d", startPort, port)
	}
	chosenURL := "http://127.0.0.1:" + strconv.Itoa(port)

	cmd := exec.Command(binPath, "serve", "--port", strconv.Itoa(port))
	cmd.Env = append(os.Environ(), "OPENCODE_SERVER_PASSWORD=")
	if err := cmd.Start(); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to start opencode (%s): %v", binPath, err))
		return
	}

	// Export the chosen URL so trySwitchToServerClient (and the chat proxy)
	// both point at the instance we just started.
	os.Setenv("OPENCODE_URL", chosenURL)

	// Wait for opencode to bind (poll /status up to ~6s). This is what unblocks
	// the UI — instead of returning "starting" and leaving the button spinning,
	// we confirm readiness before responding.
	ready := false
	for i := 0; i < 30; i++ {
		pctx, pcancel := context.WithTimeout(ctx, 500*time.Millisecond)
		ok, _ := opencode.ProbeServer(pctx, chosenURL)
		pcancel()
		if ok {
			ready = true
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	if ready && h.trySwitchToServerClient(ctx) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":  "started",
			"message": "opencode server started successfully",
			"pid":     cmd.Process.Pid,
			"url":     chosenURL,
		})
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"status":  "starting",
		"message": "opencode server is starting, please wait a moment and try again",
		"pid":     cmd.Process.Pid,
		"url":     chosenURL,
	})
}

// trySwitchToServerClient probes the opencode server at the configured URL and,
// if reachable, replaces h.opencodeClient with a ServerClient. Returns true if
// the switch happened. This lets the UI recover after starting opencode serve,
// without restarting the ywai server.
func (h *Handlers) trySwitchToServerClient(ctx context.Context) bool {
	url := os.Getenv("OPENCODE_URL")
	if url == "" {
		url = "http://127.0.0.1:4096"
	}
	sc := opencode.NewServerClient(url)
	st, err := sc.Status(ctx)
	if err != nil || !st.Connected || st.Source != "server" {
		return false
	}
	h.opencodeClient = sc
	return true
}

// —— Filesystem Browser ——————————————————————————————————————————

// BrowseFS lists directories and files at the given path.
func (h *Handlers) BrowseFS(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			writeError(w, http.StatusBadRequest, "path is required")
			return
		}
		path = home
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("cannot read path: %v", err))
		return
	}

	type Entry struct {
		Name    string `json:"name"`
		IsDir   bool   `json:"isDir"`
		Size    int64  `json:"size,omitempty"`
		ModTime string `json:"modTime"`
	}

	var result []Entry
	for _, e := range entries {
		info, err := e.Info()
		size := int64(0)
		modTime := ""
		if err == nil {
			size = info.Size()
			modTime = info.ModTime().Format(time.RFC3339)
		}
		result = append(result, Entry{
			Name:    e.Name(),
			IsDir:   e.IsDir(),
			Size:    size,
			ModTime: modTime,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"path":    path,
		"entries": result,
	})
}

// MkdirFS creates a single new directory inside parentPath. It is the write-side
// counterpart to BrowseFS. The name is sanitized to a single path segment: nested
// paths and traversal attempts are rejected.
func (h *Handlers) MkdirFS(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ParentPath string `json:"parentPath"`
		Name       string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// A folder name must be a single segment: reject anything containing a path
	// separator or traversal attempt before any normalization.
	rawName := strings.TrimSpace(req.Name)
	if rawName == "" || rawName == "." || rawName == ".." {
		writeError(w, http.StatusBadRequest, "folder name is required")
		return
	}
	if strings.ContainsAny(rawName, `/\`) || strings.Contains(rawName, "..") {
		writeError(w, http.StatusBadRequest, "folder name must be a single path segment")
		return
	}
	// Reject control characters / null bytes.
	if strings.ContainsAny(rawName, "\x00\r\n") {
		writeError(w, http.StatusBadRequest, "folder name contains invalid characters")
		return
	}

	name := rawName

	parent := req.ParentPath
	if strings.TrimSpace(parent) == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			writeError(w, http.StatusBadRequest, "parentPath is required and home dir unavailable")
			return
		}
		parent = home
	}
	parent = filepath.Clean(parent)

	full := filepath.Join(parent, name)
	// Second line of defense: the joined result must sit directly under parent.
	if filepath.Dir(full) != parent {
		writeError(w, http.StatusBadRequest, "invalid folder name")
		return
	}

	if err := os.Mkdir(full, 0755); err != nil {
		if os.IsExist(err) {
			writeError(w, http.StatusConflict, "folder already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("cannot create folder: %v", err))
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"path": full,
	})
}
