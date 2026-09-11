// Package toolsapi serves the shared tool endpoints that outlive the missions
// feature: OpenCode models/agents/status/start, filesystem browse/mkdir, goal
// refinement, and the whole Engram memory API (REST + WebSocket).
//
// RegisterRoutes mounts these routes directly on the control server's mux, in
// the same flat pattern space as every other API. The /missions URL prefix this
// package used to be mounted behind is gone; the missions name in it was
// historical and had no consumers left.
package toolsapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/engram"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/opencode"
)

// Handlers holds the collaborators the tool endpoints need.
type Handlers struct {
	hub            *Hub
	opencodeClient opencode.Client
	engramClient   engram.Client
	consolidations *ConsolidationManager

	// modelCache memoizes the slow opencode model lookup (see model_cache.go).
	modelCache modelCache
}

// NewHandlers builds the tool API handlers: hub, consolidation manager, and a
// warm-up of the model cache so the first Settings open never blocks on the
// multi-second `opencode models` CLI.
func NewHandlers() *Handlers {
	hub := NewHub()
	oc := opencode.DefaultClient(context.Background())
	engramClient := engram.DefaultClient()

	h := &Handlers{
		hub:            hub,
		opencodeClient: oc,
		engramClient:   engramClient,
	}
	h.consolidations = NewConsolidationManager(
		engramClient,
		func() opencode.SessionAPI { return h.opencodeClient.Sessions() },
		func(et string, payload any) { hub.BroadcastEvent(et, payload) },
	)
	h.WarmModels()

	return h
}

// Hub returns the hub these handlers broadcast on. New already starts its event
// loop; callers only need this to fan out their own events.
func (h *Handlers) Hub() *Hub {
	return h.hub
}

// RegisterRoutes wires the tool API routes onto mux.
func RegisterRoutes(mux *http.ServeMux, h *Handlers) {
	// Filesystem browser
	mux.HandleFunc("GET /api/fs/browse", h.BrowseFS)
	mux.HandleFunc("POST /api/fs/mkdir", h.MkdirFS)

	// OpenCode config
	mux.HandleFunc("GET /api/opencode/models", h.ListModels)
	mux.HandleFunc("GET /api/opencode/agents", h.ListAgents)
	mux.HandleFunc("GET /api/opencode/status", h.OpenCodeStatus)
	mux.HandleFunc("POST /api/opencode/start", h.StartOpencode)

	// Engram memory API
	mux.HandleFunc("GET /api/engram/status", h.EngramStatus)
	mux.HandleFunc("GET /api/engram/observations", h.ListObservations)
	mux.HandleFunc("GET /api/engram/observations/{id}", h.GetObservation)
	mux.HandleFunc("PATCH /api/engram/observations/{id}", h.UpdateObservation)
	mux.HandleFunc("DELETE /api/engram/observations/{id}", h.DeleteObservation)
	mux.HandleFunc("POST /api/engram/save", h.SaveObservation)
	mux.HandleFunc("GET /api/engram/search", h.SearchObservations)
	mux.HandleFunc("GET /api/engram/stats", h.EngramStats)
	mux.HandleFunc("GET /api/engram/sessions", h.ListEngramSessions)
	mux.HandleFunc("DELETE /api/engram/sessions/{id}", h.DeleteEngramSession)
	mux.HandleFunc("GET /api/engram/prompts", h.ListEngramPrompts)
	mux.HandleFunc("DELETE /api/engram/prompts/{id}", h.DeleteEngramPrompt)
	mux.HandleFunc("GET /api/engram/timeline", h.EngramTimeline)
	mux.HandleFunc("GET /api/engram/context", h.EngramContext)
	mux.HandleFunc("PUT /api/engram/context", h.UpdateEngramContext)
	mux.HandleFunc("GET /api/engram/export", h.ExportEngram)
	mux.HandleFunc("POST /api/engram/import", h.ImportEngram)
	mux.HandleFunc("POST /api/engram/projects/merge", h.MergeEngramProjects)
	mux.HandleFunc("POST /api/engram/memory-evals", h.RunMemoryEval)

	// Engram WebSocket
	mux.HandleFunc("GET /api/engram/ws", h.HandleEngramWebSocket)

	// Consolidations
	mux.HandleFunc("POST /api/engram/consolidations", h.StartConsolidation)
	mux.HandleFunc("GET /api/engram/consolidations/{id}", h.GetConsolidation)
	mux.HandleFunc("POST /api/engram/consolidations/{id}/apply", h.ApplyConsolidation)
	mux.HandleFunc("POST /api/engram/consolidations/{id}/discard", h.DiscardConsolidation)

	// AI refinement
	mux.HandleFunc("POST /api/refine", h.RefineGoal)
}

// —— JSON Helpers ———————————————————————————————————————————————

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
