package web

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/engram"
)

// requireEngram returns the configured engram client or writes a 503.
func (h *Handlers) requireEngram(w http.ResponseWriter) (engram.Client, bool) {
	if h.engramClient == nil {
		writeError(w, http.StatusServiceUnavailable, "engram client not configured")
		return nil, false
	}
	return h.engramClient, true
}

// queryLimit parses ?limit= with a default and a ceiling.
func queryLimit(r *http.Request, def, max int) int {
	q := r.URL.Query().Get("limit")
	if q == "" {
		return def
	}
	n, err := strconv.Atoi(q)
	if err != nil || n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}

// decodeJSONBody is a tiny helper used by the write handlers.
func decodeJSONBody(r *http.Request, target any) error {
	return json.NewDecoder(r.Body).Decode(target)
}

// ─── Engram status + observations ───────────────────────────────────────────

// EngramStatus reports whether the engram server is reachable.
func (h *Handlers) EngramStatus(w http.ResponseWriter, r *http.Request) {
	if h.engramClient == nil {
		writeJSON(w, http.StatusOK, map[string]any{"connected": false})
		return
	}
	st, err := h.engramClient.Status(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"connected": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// ListObservations → GET /api/engram/observations?limit=
func (h *Handlers) ListObservations(w http.ResponseWriter, r *http.Request) {
	c, ok := h.requireEngram(w)
	if !ok {
		return
	}
	obs, err := c.RecentObservations(r.Context(), queryLimit(r, 50, 500))
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"observations": obs})
}

// GetObservation → GET /api/engram/observations/{id}
func (h *Handlers) GetObservation(w http.ResponseWriter, r *http.Request) {
	c, ok := h.requireEngram(w)
	if !ok {
		return
	}
	obs, err := c.GetObservation(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, obs)
}

// UpdateObservation → PATCH /api/engram/observations/{id}
func (h *Handlers) UpdateObservation(w http.ResponseWriter, r *http.Request) {
	c, ok := h.requireEngram(w)
	if !ok {
		return
	}
	var req engram.UpdateRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	obs, err := c.UpdateObservation(r.Context(), r.PathValue("id"), req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, obs)
}

// DeleteObservation → DELETE /api/engram/observations/{id}
func (h *Handlers) DeleteObservation(w http.ResponseWriter, r *http.Request) {
	c, ok := h.requireEngram(w)
	if !ok {
		return
	}
	if err := c.DeleteObservation(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}

// SaveObservation → POST /api/engram/save
func (h *Handlers) SaveObservation(w http.ResponseWriter, r *http.Request) {
	c, ok := h.requireEngram(w)
	if !ok {
		return
	}
	var req engram.SaveRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}
	obs, err := c.Save(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, obs)
}

// SearchObservations → GET /api/engram/search?q=&limit=&type=
func (h *Handlers) SearchObservations(w http.ResponseWriter, r *http.Request) {
	c, ok := h.requireEngram(w)
	if !ok {
		return
	}
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusBadRequest, "q is required")
		return
	}
	obs, err := c.Search(r.Context(), engram.SearchRequest{
		Query: q,
		Limit: queryLimit(r, 50, 500),
		Type:  r.URL.Query().Get("type"),
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"observations": obs})
}

// EngramStats → GET /api/engram/stats
func (h *Handlers) EngramStats(w http.ResponseWriter, r *http.Request) {
	c, ok := h.requireEngram(w)
	if !ok {
		return
	}
	stats, err := c.GetStats(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// ListEngramSessions → GET /api/engram/sessions?limit=
func (h *Handlers) ListEngramSessions(w http.ResponseWriter, r *http.Request) {
	c, ok := h.requireEngram(w)
	if !ok {
		return
	}
	sessions, err := c.RecentSessions(r.Context(), queryLimit(r, 20, 200))
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": sessions})
}

// DeleteEngramSession → DELETE /api/engram/sessions/{id}
func (h *Handlers) DeleteEngramSession(w http.ResponseWriter, r *http.Request) {
	c, ok := h.requireEngram(w)
	if !ok {
		return
	}
	if err := c.DeleteSession(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}

// ListEngramPrompts → GET /api/engram/prompts?limit=
func (h *Handlers) ListEngramPrompts(w http.ResponseWriter, r *http.Request) {
	c, ok := h.requireEngram(w)
	if !ok {
		return
	}
	prompts, err := c.RecentPrompts(r.Context(), queryLimit(r, 50, 500))
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"prompts": prompts})
}

// DeleteEngramPrompt → DELETE /api/engram/prompts/{id}
func (h *Handlers) DeleteEngramPrompt(w http.ResponseWriter, r *http.Request) {
	c, ok := h.requireEngram(w)
	if !ok {
		return
	}
	if err := c.DeletePrompt(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}

// ExportEngram → GET /api/engram/export
// Returns the raw export blob from engram (sessions+observations+prompts).
func (h *Handlers) ExportEngram(w http.ResponseWriter, r *http.Request) {
	c, ok := h.requireEngram(w)
	if !ok {
		return
	}
	blob, err := c.Export(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="engram-export.json"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(blob)
}

// ImportEngram → POST /api/engram/import
// Body is the raw export JSON; we proxy it straight to engram.
func (h *Handlers) ImportEngram(w http.ResponseWriter, r *http.Request) {
	c, ok := h.requireEngram(w)
	if !ok {
		return
	}
	// Cap at 50 MiB to avoid runaway uploads.
	body := http.MaxBytesReader(w, r.Body, 50<<20)
	defer func() { _ = body.Close() }()
	data, err := io.ReadAll(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body: "+err.Error())
		return
	}
	result, err := c.Import(r.Context(), bytes.NewReader(data))
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// MergeEngramProjects → POST /api/engram/projects/merge {source, target}
func (h *Handlers) MergeEngramProjects(w http.ResponseWriter, r *http.Request) {
	c, ok := h.requireEngram(w)
	if !ok {
		return
	}
	var req struct {
		Source string `json:"source"`
		Target string `json:"target"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := c.MergeProjects(r.Context(), req.Source, req.Target)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// EngramTimeline → GET /api/engram/timeline?observation_id=&limit=
func (h *Handlers) EngramTimeline(w http.ResponseWriter, r *http.Request) {
	c, ok := h.requireEngram(w)
	if !ok {
		return
	}
	obsID := r.URL.Query().Get("observation_id")
	events, err := c.Timeline(r.Context(), engram.TimelineRequest{ObservationID: obsID, Limit: queryLimit(r, 50, 500)})
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

// EngramContext → GET /api/engram/context?q=&limit=
func (h *Handlers) EngramContext(w http.ResponseWriter, r *http.Request) {
	c, ok := h.requireEngram(w)
	if !ok {
		return
	}
	result, err := c.GetContext(r.Context(), engram.ContextRequest{
		Query: r.URL.Query().Get("q"),
		Limit: queryLimit(r, 100, 500),
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// UpdateEngramContext → PUT /api/engram/context
func (h *Handlers) UpdateEngramContext(w http.ResponseWriter, r *http.Request) {
	c, ok := h.requireEngram(w)
	if !ok {
		return
	}
	var req struct {
		Context string `json:"context"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	result, err := c.UpdateContext(r.Context(), req.Context)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ─── Consolidation handlers ─────────────────────────────────────────────────

// StartConsolidation → POST /api/engram/consolidations {model, agent, topic_key?, project?}
func (h *Handlers) StartConsolidation(w http.ResponseWriter, r *http.Request) {
	if h.consolidations == nil {
		writeError(w, http.StatusServiceUnavailable, "consolidation manager not configured")
		return
	}
	var req struct {
		Model    string `json:"model"`
		Agent    string `json:"agent"`
		TopicKey string `json:"topic_key,omitempty"`
		Project  string `json:"project,omitempty"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	// Require the opencode server to be up (sessions() returns nil otherwise).
	if h.opencodeClient == nil || h.opencodeClient.Sessions() == nil {
		writeError(w, http.StatusServiceUnavailable, "opencode server not running")
		return
	}
	scope := ScopeFilter{TopicKey: req.TopicKey, Project: req.Project}
	id, err := h.consolidations.Start(r.Context(), req.Model, req.Agent, scope)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"run_id": id, "status": "running"})
}

// GetConsolidation → GET /api/engram/consolidations/{id}
func (h *Handlers) GetConsolidation(w http.ResponseWriter, r *http.Request) {
	if h.consolidations == nil {
		writeError(w, http.StatusServiceUnavailable, "consolidation manager not configured")
		return
	}
	run, ok := h.consolidations.Get(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "consolidation run not found")
		return
	}
	writeJSON(w, http.StatusOK, run)
}

// ApplyConsolidation → POST /api/engram/consolidations/{id}/apply
func (h *Handlers) ApplyConsolidation(w http.ResponseWriter, r *http.Request) {
	if h.consolidations == nil {
		writeError(w, http.StatusServiceUnavailable, "consolidation manager not configured")
		return
	}
	var sel ApplySelection
	if err := decodeJSONBody(r, &sel); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := h.consolidations.Apply(r.Context(), r.PathValue("id"), sel); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "applied"})
}

// DiscardConsolidation → POST /api/engram/consolidations/{id}/discard
func (h *Handlers) DiscardConsolidation(w http.ResponseWriter, r *http.Request) {
	if h.consolidations == nil {
		writeError(w, http.StatusServiceUnavailable, "consolidation manager not configured")
		return
	}
	if err := h.consolidations.Discard(r.PathValue("id")); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "discarded"})
}

// HandleEngramWebSocket upgrades the connection and registers it with the hub.
// Consolidation events are broadcast via hub.BroadcastEvent, so this handler
// only needs the standard client lifecycle (read/write pumps owned by Hub).
func (h *Handlers) HandleEngramWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := &Client{
		hub:  h.hub,
		conn: conn,
		send: make(chan []byte, 256),
	}
	h.hub.Register(client)
	go client.writePump()
	go client.readPump()
}
