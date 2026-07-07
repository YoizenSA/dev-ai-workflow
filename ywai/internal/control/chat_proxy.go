package control

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ChatProxy proxies chat requests to a local OpenCode server, translating
// between the frontend's simple contract and OpenCode's REST API.
type ChatProxy struct {
	opencodeBaseURL string // fallback when OPENCODE_URL is unset, e.g. "http://localhost:4096"
	mu              sync.RWMutex
}

func NewChatProxy(opencodeBaseURL string) *ChatProxy {
	return &ChatProxy{
		opencodeBaseURL: opencodeBaseURL,
	}
}

// baseURL returns the OpenCode server URL to proxy to. It prefers the live
// OPENCODE_URL env var (which the "Start OpenCode" handler updates when it
// spawns opencode on a dynamic port) and falls back to the URL the proxy was
// constructed with. This is what lets the chat recover after opencode is
// (re)started without restarting ywai.
func (cp *ChatProxy) baseURL() string {
	if u := strings.TrimSpace(os.Getenv("OPENCODE_URL")); u != "" {
		return strings.TrimRight(u, "/")
	}
	return cp.opencodeBaseURL
}





// handleChatSSE proxies OpenCode's global /event stream to the client,
// forwarding only events for the requested session (plus global events).
// GET /api/chat/events?sessionID=xxx
func (cp *ChatProxy) handleChatSSE(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("sessionID")
	if sessionID == "" {
		http.Error(w, "sessionID required", http.StatusBadRequest)
		return
	}

	upstreamURL := cp.baseURL() + "/event"
	req, err := http.NewRequestWithContext(r.Context(), "GET", upstreamURL, nil)
	if err != nil {
		http.Error(w, "failed to create request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Accept", "text/event-stream")

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
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if cp.shouldForwardEvent(data, sessionID) {
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
		}
	}
}

// shouldForwardEvent checks whether an OpenCode event belongs to the requested
// session. OpenCode nests the session id under `properties.sessionID`.
func (cp *ChatProxy) shouldForwardEvent(data, sessionID string) bool {
	var event struct {
		Properties struct {
			SessionID string `json:"sessionID"`
		} `json:"properties"`
	}
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return true // unparseable → forward (likely a global event)
	}
	if event.Properties.SessionID == "" {
		return true // global event (server.connected, catalog.updated, ...)
	}
	return event.Properties.SessionID == sessionID
}

// handleListSessions lists OpenCode sessions.
// GET /api/chat/sessions -> {"sessions": [...]}
func (cp *ChatProxy) handleListSessions(w http.ResponseWriter, r *http.Request) {
	body, status, err := cp.upstream(r, "GET", "/session", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if status != http.StatusOK {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write(body)
		return
	}
	// OpenCode returns a bare array; the frontend expects {"sessions": [...]}.
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"sessions":%s}`, body)
}

// handleCreateSession creates a new OpenCode session. An optional ?directory=
// query scopes the session to a specific workspace (project worktree path).
// POST /api/chat/sessions[?directory=...] -> session object (with id)
func (cp *ChatProxy) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	reqBody, _ := io.ReadAll(r.Body)
	if len(bytes.TrimSpace(reqBody)) == 0 {
		reqBody = []byte("{}")
	}
	path := "/session"
	if dir := r.URL.Query().Get("directory"); dir != "" {
		path += "?directory=" + url.QueryEscape(dir)
	}
	body, status, err := cp.upstream(r, "POST", path, reqBody)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(body)
}

// handleAgents returns the agents that can be run as the primary agent
// (OpenCode "primary"/"all" modes; "subagent"-only agents are excluded).
// GET /api/chat/agents -> {"agents":[{name,description,mode}]}
func (cp *ChatProxy) handleAgents(w http.ResponseWriter, r *http.Request) {
	body, status, err := cp.upstream(r, "GET", "/agent", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if status != http.StatusOK {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write(body)
		return
	}
	var raw []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Mode        string `json:"mode"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		http.Error(w, "failed to parse agents", http.StatusBadGateway)
		return
	}
	agents := make([]map[string]string, 0, len(raw))
	for _, a := range raw {
		if a.Mode == "subagent" {
			continue
		}
		agents = append(agents, map[string]string{
			"name": a.Name, "description": a.Description, "mode": a.Mode,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"agents": agents})
}

// handleProjects returns the workspaces (OpenCode projects) the user can start
// chats in, deduplicated by worktree path and excluding the global "/" root.
// GET /api/chat/projects -> {"projects":[{id,path,name}]}
func (cp *ChatProxy) handleProjects(w http.ResponseWriter, r *http.Request) {
	body, status, err := cp.upstream(r, "GET", "/project", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if status != http.StatusOK {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write(body)
		return
	}
	var raw []struct {
		ID       string `json:"id"`
		Worktree string `json:"worktree"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		http.Error(w, "failed to parse projects", http.StatusBadGateway)
		return
	}
	seen := map[string]bool{}
	projects := make([]map[string]string, 0, len(raw))
	for _, p := range raw {
		if p.Worktree == "" || p.Worktree == "/" || seen[p.Worktree] {
			continue
		}
		seen[p.Worktree] = true
		name := filepath.Base(strings.TrimRight(p.Worktree, "/"))
		projects = append(projects, map[string]string{
			"id": p.ID, "path": p.Worktree, "name": name,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": projects})
}

// handleChildren lists child (subagent) sessions of a session, used to
// visualize async/background subagents.
// GET /api/chat/sessions/{id}/children -> {"children":[{id,title,...}]}
func (cp *ChatProxy) handleChildren(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	body, status, err := cp.upstream(r, "GET", "/session/"+id+"/children", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if status != http.StatusOK {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write(body)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"children":%s}`, body)
}

// handleSessionInfo returns a single session's metadata (id, title, parentID,
// time, …) by passing opencode's GET /session/{id} through unchanged. The UI
// uses it to navigate up to a parent session that may not be in the loaded
// session list. (GET /api/chat/sessions/{id} is wired to handleGetMessages,
// hence the dedicated /info route.)
func (cp *ChatProxy) handleSessionInfo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	body, status, err := cp.upstream(r, "GET", "/session/"+id, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(body)
}

// opencodeMessage is the subset of an OpenCode message we surface to the UI.
type opencodeMessage struct {
	Info struct {
		ID   string `json:"id"`
		Role string `json:"role"`
		Time struct {
			Created int64 `json:"created"`
		} `json:"time"`
	} `json:"info"`
	Parts []struct {
		ID          string `json:"id"`
		Type        string `json:"type"`
		Text        string `json:"text"`
		Tool        string `json:"tool"`
		Agent       string `json:"agent"`
		Description string `json:"description"`
		Prompt      string `json:"prompt"`
		State       struct {
			Status string          `json:"status"`
			Title  string          `json:"title"`
			Output string          `json:"output"`
			Error  string          `json:"error"`
			Input  json.RawMessage `json:"input"`
		} `json:"state"`
	} `json:"parts"`
}

// outPart is a typed message fragment surfaced to the UI: assistant replies mix
// plain text, reasoning ("thinking"), and tool calls.
type outPart struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"` // text | reasoning | tool | subtask
	Text        string `json:"text,omitempty"`
	Tool        string `json:"tool,omitempty"`
	Status      string `json:"status,omitempty"`
	Title       string `json:"title,omitempty"`
	Output      string `json:"output,omitempty"`
	Agent       string `json:"agent,omitempty"`       // subtask: which subagent
	Description string `json:"description,omitempty"` // subtask: task description
}

// handleGetMessages returns the flattened message history for a session.
// GET /api/chat/sessions/{id} -> {"messages": [{id, role, content, created_at}]}
func (cp *ChatProxy) handleGetMessages(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	body, status, err := cp.upstream(r, "GET", "/session/"+id+"/message", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if status != http.StatusOK {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write(body)
		return
	}

	var raw []opencodeMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		http.Error(w, "failed to parse upstream messages", http.StatusBadGateway)
		return
	}

	type outMsg struct {
		ID        string    `json:"id"`
		Role      string    `json:"role"`
		Parts     []outPart `json:"parts"`
		CreatedAt int64     `json:"created_at"`
	}
	msgs := make([]outMsg, 0, len(raw))
	for _, m := range raw {
		parts := make([]outPart, 0, len(m.Parts))
		for _, p := range m.Parts {
			switch p.Type {
			case "text":
				if p.Text != "" {
					parts = append(parts, outPart{ID: p.ID, Kind: "text", Text: p.Text})
				}
			case "reasoning":
				if p.Text != "" {
					parts = append(parts, outPart{ID: p.ID, Kind: "reasoning", Text: p.Text})
				}
			case "tool":
				out := p.State.Output
				if p.State.Error != "" {
					out = p.State.Error
				}
				parts = append(parts, outPart{
					ID:     p.ID,
					Kind:   "tool",
					Tool:   p.Tool,
					Status: p.State.Status,
					Title:  p.State.Title,
					Output: out,
				})
			case "subtask":
				desc := p.Description
				if desc == "" {
					desc = p.Prompt
				}
				parts = append(parts, outPart{
					ID:          p.ID,
					Kind:        "subtask",
					Agent:       p.Agent,
					Description: desc,
				})
			}
		}
		msgs = append(msgs, outMsg{
			ID:        m.Info.ID,
			Role:      m.Info.Role,
			Parts:     parts,
			CreatedAt: m.Info.Time.Created,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}

// handleSendMessage sends a user prompt to OpenCode. A "/compact" message is
// translated to a session summarize call.
// POST /api/chat/sessions/{id}/message  body: {"message":"...","model":"providerID/modelID"}
func (cp *ChatProxy) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		Message string `json:"message"`
		Agent   string `json:"agent"`
		// Model is "providerID/modelID" as sent by the UI; split for upstream.
		Model string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var path string
	var payload []byte
	if strings.TrimSpace(req.Message) == "/compact" {
		path = "/session/" + id + "/summarize"
		payload = []byte("{}")
	} else {
		path = "/session/" + id + "/message"
		msg := map[string]any{
			"parts": []map[string]string{{"type": "text", "text": req.Message}},
		}
		if pid, mid, ok := strings.Cut(req.Model, "/"); ok && pid != "" && mid != "" {
			msg["model"] = map[string]string{"providerID": pid, "modelID": mid}
		}
		if req.Agent != "" {
			msg["agent"] = req.Agent
		}
		payload, _ = json.Marshal(msg)
	}

	// The prompt call blocks until the assistant finishes; results stream
	// concurrently via /event, so no client timeout here.
	body, status, err := cp.upstreamTimeout(r, "POST", path, payload, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(body)
}

// handleAbort aborts a running session.
// POST /api/chat/abort  body: {"sessionID": "..."}
func (cp *ChatProxy) handleAbort(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"sessionID"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SessionID == "" {
		http.Error(w, "sessionID required", http.StatusBadRequest)
		return
	}
	body, status, err := cp.upstream(r, "POST", "/session/"+req.SessionID+"/abort", []byte("{}"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(body)
}

// upstream performs a request against the OpenCode server with a default
// timeout and returns the response body and status.
func (cp *ChatProxy) upstream(r *http.Request, method, path string, body []byte) ([]byte, int, error) {
	return cp.upstreamTimeout(r, method, path, body, 30*time.Second)
}

func (cp *ChatProxy) upstreamTimeout(r *http.Request, method, path string, body []byte, timeout time.Duration) ([]byte, int, error) {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(r.Context(), method, cp.baseURL()+path, rdr)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("upstream error: %v", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read upstream response: %w", err)
	}
	return respBody, resp.StatusCode, nil
}

// handleProviders returns OpenCode's configured providers and models.
// The upstream payload embeds provider API keys, so we strip everything down to
// id/name/model ids before returning it to the browser — never forward secrets.
// GET /api/chat/providers -> {providers:[{id,name,models:{modelID:{}}}], default:{...}}
func (cp *ChatProxy) handleProviders(w http.ResponseWriter, r *http.Request) {
	body, status, err := cp.upstream(r, "GET", "/config/providers", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if status != http.StatusOK {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write(body)
		return
	}

	var raw struct {
		Providers []struct {
			ID     string                     `json:"id"`
			Name   string                     `json:"name"`
			Models map[string]json.RawMessage `json:"models"`
		} `json:"providers"`
		Default map[string]string `json:"default"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		http.Error(w, "failed to parse providers", http.StatusBadGateway)
		return
	}

	type provOut struct {
		ID     string              `json:"id"`
		Name   string              `json:"name"`
		Models map[string]struct{} `json:"models"`
	}
	out := make([]provOut, 0, len(raw.Providers))
	for _, p := range raw.Providers {
		models := make(map[string]struct{}, len(p.Models))
		for id := range p.Models {
			models[id] = struct{}{}
		}
		out = append(out, provOut{ID: p.ID, Name: p.Name, Models: models})
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": out, "default": raw.Default})
}

// handleFileList lists files and/or folders in the workspace matching a query.
// GET /api/files?q=<query>[&type=file|dir|all]
// Directories are returned with a trailing "/". type defaults to "all" so a
// single "@" menu can surface both; "dir" powers a folder-only "#" trigger.
func (cp *ChatProxy) handleFileList(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusOK, map[string]any{"files": []string{}})
		return
	}
	kind := r.URL.Query().Get("type")
	wantFiles := kind == "" || kind == "all" || kind == "file"
	wantDirs := kind == "" || kind == "all" || kind == "dir"

	root := "."
	for _, marker := range []string{"go.mod", "package.json", ".git"} {
		dir := findFileUpwards(marker)
		if dir != "" {
			root = dir
			break
		}
	}

	limit := 20
	lq := strings.ToLower(q)
	var matches []string

	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			if !wantDirs || path == root {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return nil
			}
			if strings.Contains(strings.ToLower(rel), lq) {
				matches = append(matches, rel+"/")
			}
			return nil
		}
		if !wantFiles {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		// Substring match anywhere in the path so "@core" finds
		// "internal/control/...", not just paths starting with the query.
		if strings.Contains(strings.ToLower(rel), lq) {
			matches = append(matches, rel)
		}
		return nil
	})

	// Rank: basename matches before path-only matches, then shorter paths,
	// then alphabetical — so the most relevant files surface first.
	sort.Slice(matches, func(i, j int) bool {
		bi := strings.Contains(strings.ToLower(filepath.Base(matches[i])), lq)
		bj := strings.Contains(strings.ToLower(filepath.Base(matches[j])), lq)
		if bi != bj {
			return bi
		}
		if len(matches[i]) != len(matches[j]) {
			return len(matches[i]) < len(matches[j])
		}
		return matches[i] < matches[j]
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}
	if matches == nil {
		matches = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": matches})
}

// findFileUpwards walks up from cwd looking for a file, returns the dir it's in.
func findFileUpwards(name string) string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// handleRenameSession renames a session.
// PATCH /api/chat/sessions/{id}  body: {"title": "..."}
func (cp *ChatProxy) handleRenameSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	respBody, status, err := cp.upstream(r, "PATCH", "/session/"+id, body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(respBody)
}

// handleDeleteSession deletes a session.
// DELETE /api/chat/sessions/{id}
func (cp *ChatProxy) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	body, status, err := cp.upstream(r, "DELETE", "/session/"+id, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(body)
}

// handlePermissionReply replies to a permission request.
// POST /api/chat/sessions/{id}/permissions/{permissionID}
func (cp *ChatProxy) handlePermissionReply(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	permissionID := r.PathValue("permissionID")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	respBody, status, err := cp.upstream(r, "POST", "/session/"+id+"/permissions/"+permissionID, body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(respBody)
}

// handleQuestionReply replies to a question.
// POST /api/chat/sessions/{id}/question/{requestID}/reply
func (cp *ChatProxy) handleQuestionReply(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	requestID := r.PathValue("requestID")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	respBody, status, err := cp.upstream(r, "POST", "/api/session/"+id+"/question/"+requestID+"/reply", body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(respBody)
}

// handleQuestionReject rejects a question.
// POST /api/chat/sessions/{id}/question/{requestID}/reject
func (cp *ChatProxy) handleQuestionReject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	requestID := r.PathValue("requestID")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	respBody, status, err := cp.upstream(r, "POST", "/api/session/"+id+"/question/"+requestID+"/reject", body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(respBody)
}

// handleSessionContext returns token usage for a session.
// GET /api/chat/sessions/{id}/context
func (cp *ChatProxy) handleSessionContext(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	body, status, err := cp.upstream(r, "GET", "/api/session/"+id+"/context", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(body)
}

// handleTodo returns the todo list for a session.
// GET /api/chat/sessions/{id}/todo
func (cp *ChatProxy) handleTodo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	body, status, err := cp.upstream(r, "GET", "/session/"+id+"/todo", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(body)
}

// handleDiff returns file diffs for a session.
// GET /api/chat/sessions/{id}/diff
func (cp *ChatProxy) handleDiff(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	body, status, err := cp.upstream(r, "GET", "/session/"+id+"/diff", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(body)
}

// handleDeleteMessage deletes a message in a session.
// DELETE /api/chat/sessions/{id}/message/{messageID}
func (cp *ChatProxy) handleDeleteMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	messageID := r.PathValue("messageID")
	body, status, err := cp.upstream(r, "DELETE", "/session/"+id+"/message/"+messageID, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(body)
}

// handleRevert reverts changes in a session.
// POST /api/chat/sessions/{id}/revert
func (cp *ChatProxy) handleRevert(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	respBody, status, err := cp.upstream(r, "POST", "/session/"+id+"/revert", body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(respBody)
}

// handleCommand executes a command in a session.
// POST /api/chat/sessions/{id}/command
func (cp *ChatProxy) handleCommand(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	respBody, status, err := cp.upstream(r, "POST", "/session/"+id+"/command", body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(respBody)
}

// registerOpenCodeProxy wires every /api/chat route to the OpenCode server.
func (s *Server) registerOpenCodeProxy(opencodeURL string) {
	cp := NewChatProxy(opencodeURL)
	log.Printf("[chat] proxying to OpenCode at %s", opencodeURL)

	s.mux.HandleFunc("GET /api/chat/sessions", cp.handleListSessions)
	s.mux.HandleFunc("POST /api/chat/sessions", cp.handleCreateSession)
	s.mux.HandleFunc("GET /api/chat/sessions/{id}", cp.handleGetMessages)
	s.mux.HandleFunc("GET /api/chat/sessions/{id}/info", cp.handleSessionInfo)
	s.mux.HandleFunc("GET /api/chat/sessions/{id}/children", cp.handleChildren)
	s.mux.HandleFunc("POST /api/chat/sessions/{id}/message", cp.handleSendMessage)
	s.mux.HandleFunc("GET /api/chat/events", cp.handleChatSSE)
	s.mux.HandleFunc("POST /api/chat/abort", cp.handleAbort)
	s.mux.HandleFunc("GET /api/chat/providers", cp.handleProviders)
	s.mux.HandleFunc("GET /api/chat/agents", cp.handleAgents)
	s.mux.HandleFunc("GET /api/chat/projects", cp.handleProjects)
	s.mux.HandleFunc("GET /api/files", cp.handleFileList)
	s.mux.HandleFunc("PATCH /api/chat/sessions/{id}", cp.handleRenameSession)
	s.mux.HandleFunc("DELETE /api/chat/sessions/{id}", cp.handleDeleteSession)
	s.mux.HandleFunc("POST /api/chat/sessions/{id}/permissions/{permissionID}", cp.handlePermissionReply)
	s.mux.HandleFunc("POST /api/chat/sessions/{id}/question/{requestID}/reply", cp.handleQuestionReply)
	s.mux.HandleFunc("POST /api/chat/sessions/{id}/question/{requestID}/reject", cp.handleQuestionReject)
	s.mux.HandleFunc("GET /api/chat/sessions/{id}/context", cp.handleSessionContext)
	s.mux.HandleFunc("GET /api/chat/sessions/{id}/todo", cp.handleTodo)
	s.mux.HandleFunc("GET /api/chat/sessions/{id}/diff", cp.handleDiff)
	s.mux.HandleFunc("GET /api/chat/gitdiff", cp.handleGitDiff)
	s.mux.HandleFunc("DELETE /api/chat/sessions/{id}/message/{messageID}", cp.handleDeleteMessage)
	s.mux.HandleFunc("POST /api/chat/sessions/{id}/revert", cp.handleRevert)
	s.mux.HandleFunc("POST /api/chat/sessions/{id}/command", cp.handleCommand)
}

// detectOpenCodeURL tries to find a running OpenCode server.
func detectOpenCodeURL() string {
	if url := strings.TrimSpace(os.Getenv("OPENCODE_URL")); url != "" {
		return strings.TrimRight(url, "/")
	}
	// Try common ports. OpenCode's default is 4096.
	ports := []string{"4096", "3000", "3001"}
	for _, port := range ports {
		url := fmt.Sprintf("http://localhost:%s", port)
		client := &http.Client{Timeout: 1 * time.Second}
		resp, err := client.Get(url + "/app")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return url
			}
		}
	}
	return ""
}
