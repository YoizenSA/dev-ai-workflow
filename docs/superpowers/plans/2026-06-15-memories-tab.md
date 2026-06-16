# Memories Tab Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a "Memories" tab to the control UI that views/captures/edits/deletes engram memories and runs opencode-driven consolidation (with model + agent selection) that proposes changes for user review before applying.

**Architecture:** A new `internal/engram/` HTTP client package (mirrors `internal/opencode/`) talks to the engram binary on port 7437. Handlers are added to the existing `missions/web` mux under `/api/engram/*`. A consolidation manager (in-memory, goroutine-per-run) drives an opencode `SessionAPI` session and broadcasts progress over a WebSocket. A dedicated `memory` agent (read-only engram tools) produces a structured JSON plan; the backend applies accepted changes after user review. The frontend adds a Memories route, store, and components following the Missions pattern.

**Tech Stack:** Go 1.22+ (net/http ServeMux), React 18 + TypeScript + Zustand, engram REST API (:7437), opencode SessionAPI (:4096), Vite, gorilla/websocket.

**Reference spec:** `docs/superpowers/specs/2026-06-15-memories-tab-design.md`

**Key path convention:** The missions/web mux registers routes like `/api/engram/*` and `/engram/ws`; the control server mounts it under `/missions`, so the browser calls `/missions/api/engram/*` and `/missions/engram/ws`. The Vite `/missions` proxy rule already exists — no proxy change needed.

---

## File Structure

### Backend (Go)

| File | Responsibility |
|------|----------------|
| **Create** `ywai/internal/engram/models.go` | DTO types: `Observation`, `Session`, `Stats`, `TimelineEvent`, `ContextResult`, request types, `Status`. |
| **Create** `ywai/internal/engram/client.go` | `Client` interface + `ErrEngramUnavailable`. |
| **Create** `ywai/internal/engram/http_client.go` | `HTTPClient` implementing `Client` against `:7437`. |
| **Create** `ywai/internal/engram/factory.go` | `DefaultClient()`, `ProbeServer()`, `ENGRAM_URL` env. |
| **Create** `ywai/internal/engram/plan.go` | `ConsolidationPlan` + item types (shared by backend + manager). |
| **Create** `ywai/internal/engram/http_client_test.go` | `httptest`-based tests for every client method + unavailable case. |
| **Create** `ywai/internal/engram/factory_test.go` | `ProbeServer` reachable/unreachable tests. |
| **Modify** `ywai/internal/missions/web/handlers.go` | Add `engramClient` + `consolidations` fields to `Handlers`; add engram + consolidation handler methods. |
| **Modify** `ywai/internal/missions/web/server.go` | Wire `engramClient` + consolidation manager in `New()`; register `/api/engram/*` + `/engram/ws` routes. |
| **Create** `ywai/internal/missions/web/consolidation.go` | `ConsolidationManager` (in-memory runs map + goroutine driver using `opencode.SessionAPI`). |
| **Create** `ywai/internal/missions/web/consolidation_test.go` | Manager tests with a fake `SessionAPI`. |
| **Create** `ywai/internal/missions/web/engram_handlers_test.go` | `httptest` handler tests with a fake `engram.Client`. |
| **Modify** `ywai/internal/control/server.go` | Add `/memories` to SPA route whitelist (`serveSPA`). |

### Agent

| File | Responsibility |
|------|----------------|
| **Create** `ywai/agents/core/memory/AGENT.md` | System prompt for the memory-consolidation specialist. |
| **Create** `ywai/agents/core/memory/permissions.json` | Tool permissions: read tools allow, engram write tools deny, mcp read allow. |
| **Modify** `ywai/agents/groups.json` | Add `"memory"` to the `core` group's `agents` array. |

### Frontend

| File | Responsibility |
|------|----------------|
| **Modify** `ywai/internal/control/web/src/api/types.ts` | Add `// ─── Memories Types ───` section. |
| **Modify** `ywai/internal/control/web/src/api/client.ts` | Add `memoriesApi` exported object. |
| **Create** `ywai/internal/control/web/src/stores/memoriesStore.ts` | Zustand store (state + actions + `handleWSMessage`). |
| **Create** `ywai/internal/control/web/src/components/memories/Memories.tsx` | Page (default export) with 4 internal sub-tabs. |
| **Create** `ywai/internal/control/web/src/components/memories/Memories.css` | Page-scoped styles (tokens only). |
| **Create** `ywai/internal/control/web/src/components/memories/MemoryCard.tsx` | Observation card. |
| **Create** `ywai/internal/control/web/src/components/memories/MemoryDetail.tsx` | Split-view detail panel. |
| **Create** `ywai/internal/control/web/src/components/memories/CaptureMemoryModal.tsx` | Manual capture form. |
| **Create** `ywai/internal/control/web/src/components/memories/ConsolidationModal.tsx` | Start consolidation + live progress. |
| **Create** `ywai/internal/control/web/src/components/memories/ConsolidationPlanReview.tsx` | Selective plan review/apply. |
| **Modify** `ywai/internal/control/web/src/App.tsx` | Add `<Route path="/memories" ...>`. |
| **Modify** `ywai/internal/control/web/src/components/layout/Sidebar.tsx` | Add Memories `NAV_ITEMS` entry. |

---

## Task 1: engram DTO types

**Files:**
- Create: `ywai/internal/engram/models.go`

- [ ] **Step 1: Create the models file**

```go
// Package engram provides a client for the engram memory server REST API
// (default http://127.0.0.1:7437).
package engram

import "time"

// Status reports engram server connectivity.
type Status struct {
	Connected bool   `json:"connected"`
	Source    string `json:"source,omitempty"` // "server"
	Version   string `json:"version,omitempty"`
}

// Observation is a single engram memory record.
type Observation struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type,omitempty"`          // save, observation, summary, topic...
	Content      string                 `json:"content,omitempty"`
	Importance   float64                `json:"importance,omitempty"`
	Topic        string                 `json:"topic,omitempty"`
	SessionID    string                 `json:"sessionID,omitempty"`
	Tags         []string               `json:"tags,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt    time.Time              `json:"createdAt,omitempty"`
	LastAccessed time.Time              `json:"lastAccessed,omitempty"`
}

// Session is an engram session summary.
type Session struct {
	ID        string    `json:"id"`
	Title     string    `json:"title,omitempty"`
	Summary   string    `json:"summary,omitempty"`
	Start     time.Time `json:"start,omitempty"`
	End       time.Time `json:"end,omitempty"`
	Topics    []string  `json:"topics,omitempty"`
}

// Stats holds aggregate memory counts.
type Stats struct {
	Total         int             `json:"total"`
	ByType        map[string]int  `json:"byType,omitempty"`
	ByImportance  map[string]int  `json:"byImportance,omitempty"`
	AvgImportance float64         `json:"avgImportance,omitempty"`
}

// TimelineEvent is one entry in the memory timeline.
type TimelineEvent struct {
	ID        string    `json:"id"`
	Type      string    `json:"type,omitempty"`
	Content   string    `json:"content,omitempty"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
}

// ContextResult is what engram returns for GET /context — what the system
// currently "knows". Engram may return either an array of observations or a
// structured object; we decode into the most useful shape and pass it through.
type ContextResult struct {
	Summary      string        `json:"summary,omitempty"`
	Observations []Observation `json:"observations,omitempty"`
	Raw          interface{}   `json:"raw,omitempty"`
}

// ─── Request types ──────────────────────────────────────────────────────────

// SaveRequest is the body for POST /save.
type SaveRequest struct {
	Type       string                 `json:"type"`
	Content    string                 `json:"content"`
	Importance float64                `json:"importance,omitempty"`
	Topic      string                 `json:"topic,omitempty"`
	Tags       []string               `json:"tags,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// UpdateRequest is the body for PATCH /observations/{id}. All fields optional.
type UpdateRequest struct {
	Content    *string                `json:"content,omitempty"`
	Importance *float64               `json:"importance,omitempty"`
	Topic      *string                `json:"topic,omitempty"`
	Tags       []string               `json:"tags,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// SearchRequest drives GET /search.
type SearchRequest struct {
	Query string
	Limit int
	Type  string // optional filter
}

// TimelineRequest drives GET /timeline.
type TimelineRequest struct {
	Limit int
}

// ContextRequest drives GET /context.
type ContextRequest struct {
	Query string
	Limit int
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd ywai && go build ./internal/engram/`
Expected: no output (success).

- [ ] **Step 3: Commit**

```bash
git add ywai/internal/engram/models.go
git commit -m "feat(engram): add DTO types for engram client"
```

---

## Task 2: engram Client interface + HTTP client

**Files:**
- Create: `ywai/internal/engram/client.go`
- Create: `ywai/internal/engram/http_client.go`

- [ ] **Step 1: Write the interface + error (client.go)**

```go
package engram

import (
	"context"
	"errors"
)

// ErrEngramUnavailable is returned when engram is not reachable.
var ErrEngramUnavailable = errors.New("engram server is not running")

// Client provides access to the engram memory server REST API.
type Client interface {
	Status(ctx context.Context) (Status, error)
	RecentObservations(ctx context.Context, limit int) ([]Observation, error)
	GetObservation(ctx context.Context, id string) (Observation, error)
	UpdateObservation(ctx context.Context, id string, req UpdateRequest) (Observation, error)
	DeleteObservation(ctx context.Context, id string) error
	Save(ctx context.Context, req SaveRequest) (Observation, error)
	Search(ctx context.Context, req SearchRequest) ([]Observation, error)
	GetStats(ctx context.Context) (Stats, error)
	RecentSessions(ctx context.Context, limit int) ([]Session, error)
	Timeline(ctx context.Context, req TimelineRequest) ([]TimelineEvent, error)
	GetContext(ctx context.Context, req ContextRequest) (ContextResult, error)
}
```

- [ ] **Step 2: Write the HTTP client (http_client.go)**

```go
package engram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const defaultEngramTimeout = 5 * time.Second

// HTTPClient implements Client against the engram REST API.
type HTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewHTTPClient creates a client targeting baseURL (e.g. "http://127.0.0.1:7437").
func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: defaultEngramTimeout,
		},
	}
}

// ─── helpers ────────────────────────────────────────────────────────────────

func (c *HTTPClient) getJSON(ctx context.Context, path string, query url.Values, target interface{}) error {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("engram: create request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("engram: %s: %w", path, err)
	}
	return decodeJSON(resp, target)
}

func (c *HTTPClient) sendJSON(ctx context.Context, method, path string, body, target interface{}) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("engram: marshal body: %w", err)
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("engram: create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("engram: %s: %w", path, err)
	}
	if method == http.MethodDelete && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		resp.Body.Close()
		return nil
	}
	return decodeJSON(resp, target)
}

func decodeJSON(resp *http.Response, target interface{}) error {
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("engram: %s returned %d: %s", resp.Request.URL.Path, resp.StatusCode, string(body))
	}
	if target == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

// engram responses may be a bare array or wrapped in {"data": [...]} /
// {"observations": [...]}. unwrapList finds the array in either shape.
func unwrapList(raw json.RawMessage) json.RawMessage {
	// Try bare array first.
	trim := bytes.TrimSpace(raw)
	if len(trim) > 0 && trim[0] == '[' {
		return raw
	}
	// Try known wrapper keys.
	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal(raw, &wrapper); err == nil {
		for _, key := range []string{"data", "observations", "sessions", "events", "timeline"} {
			if v, ok := wrapper[key]; ok {
				return v
			}
		}
	}
	return raw
}

// ─── Client interface implementation ────────────────────────────────────────

func (c *HTTPClient) Status(ctx context.Context) (Status, error) {
	var st Status
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return st, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return st, ErrEngramUnavailable
	}
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return st, ErrEngramUnavailable
	}
	return Status{Connected: true, Source: "server"}, nil
}

func (c *HTTPClient) RecentObservations(ctx context.Context, limit int) ([]Observation, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var raw json.RawMessage
	if err := c.getJSON(ctx, "/observations/recent", q, &raw); err != nil {
		return nil, err
	}
	var obs []Observation
	if err := json.Unmarshal(unwrapList(raw), &obs); err != nil {
		return nil, fmt.Errorf("engram: decode observations: %w", err)
	}
	return obs, nil
}

func (c *HTTPClient) GetObservation(ctx context.Context, id string) (Observation, error) {
	var obs Observation
	if err := c.getJSON(ctx, "/observations/"+url.PathEscape(id), nil, &obs); err != nil {
		return Observation{}, err
	}
	return obs, nil
}

func (c *HTTPClient) UpdateObservation(ctx context.Context, id string, req UpdateRequest) (Observation, error) {
	var obs Observation
	if err := c.sendJSON(ctx, http.MethodPatch, "/observations/"+url.PathEscape(id), req, &obs); err != nil {
		return Observation{}, err
	}
	return obs, nil
}

func (c *HTTPClient) DeleteObservation(ctx context.Context, id string) error {
	return c.sendJSON(ctx, http.MethodDelete, "/observations/"+url.PathEscape(id), nil, nil)
}

func (c *HTTPClient) Save(ctx context.Context, req SaveRequest) (Observation, error) {
	var obs Observation
	if err := c.sendJSON(ctx, http.MethodPost, "/save", req, &obs); err != nil {
		return Observation{}, err
	}
	return obs, nil
}

func (c *HTTPClient) Search(ctx context.Context, req SearchRequest) ([]Observation, error) {
	q := url.Values{}
	q.Set("q", req.Query)
	if req.Limit > 0 {
		q.Set("limit", strconv.Itoa(req.Limit))
	}
	if req.Type != "" {
		q.Set("type", req.Type)
	}
	var raw json.RawMessage
	if err := c.getJSON(ctx, "/search", q, &raw); err != nil {
		return nil, err
	}
	var obs []Observation
	if err := json.Unmarshal(unwrapList(raw), &obs); err != nil {
		return nil, fmt.Errorf("engram: decode search: %w", err)
	}
	return obs, nil
}

func (c *HTTPClient) GetStats(ctx context.Context) (Stats, error) {
	var stats Stats
	if err := c.getJSON(ctx, "/stats", nil, &stats); err != nil {
		return Stats{}, err
	}
	return stats, nil
}

func (c *HTTPClient) RecentSessions(ctx context.Context, limit int) ([]Session, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var raw json.RawMessage
	if err := c.getJSON(ctx, "/sessions/recent", q, &raw); err != nil {
		return nil, err
	}
	var sessions []Session
	if err := json.Unmarshal(unwrapList(raw), &sessions); err != nil {
		return nil, fmt.Errorf("engram: decode sessions: %w", err)
	}
	return sessions, nil
}

func (c *HTTPClient) Timeline(ctx context.Context, req TimelineRequest) ([]TimelineEvent, error) {
	q := url.Values{}
	if req.Limit > 0 {
		q.Set("limit", strconv.Itoa(req.Limit))
	}
	var raw json.RawMessage
	if err := c.getJSON(ctx, "/timeline", q, &raw); err != nil {
		return nil, err
	}
	var events []TimelineEvent
	if err := json.Unmarshal(unwrapList(raw), &events); err != nil {
		return nil, fmt.Errorf("engram: decode timeline: %w", err)
	}
	return events, nil
}

func (c *HTTPClient) GetContext(ctx context.Context, req ContextRequest) (ContextResult, error) {
	q := url.Values{}
	if req.Query != "" {
		q.Set("q", req.Query)
	}
	if req.Limit > 0 {
		q.Set("limit", strconv.Itoa(req.Limit))
	}
	var result ContextResult
	if err := c.getJSON(ctx, "/context", q, &result); err != nil {
		return ContextResult{}, err
	}
	return result, nil
}
```

- [ ] **Step 3: Verify it compiles**

Run: `cd ywai && go build ./internal/engram/`
Expected: no output.

- [ ] **Step 4: Commit**

```bash
git add ywai/internal/engram/client.go ywai/internal/engram/http_client.go
git commit -m "feat(engram): add HTTP client implementing engram.Client"
```

---

## Task 3: engram factory (DefaultClient + ProbeServer)

**Files:**
- Create: `ywai/internal/engram/factory.go`

- [ ] **Step 1: Write factory.go**

```go
package engram

import (
	"context"
	"net/http"
	"os"
	"time"
)

const defaultProbeTimeout = 2 * time.Second

// DefaultClient returns an HTTPClient pointing at the engram server
// (http://127.0.0.1:7437 by default; override with ENGRAM_URL).
// It does NOT probe — callers should use Status() to check connectivity.
func DefaultClient() *HTTPClient {
	url := os.Getenv("ENGRAM_URL")
	if url == "" {
		url = "http://127.0.0.1:7437"
	}
	return NewHTTPClient(url)
}

// ProbeServer reports whether the engram server is reachable at baseURL.
func ProbeServer(ctx context.Context, baseURL string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/health", nil)
	if err != nil {
		return false
	}
	client := &http.Client{Timeout: defaultProbeTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}
```

- [ ] **Step 2: Verify build**

Run: `cd ywai && go build ./internal/engram/`
Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add ywai/internal/engram/factory.go
git commit -m "feat(engram): add DefaultClient + ProbeServer factory"
```

---

## Task 4: ConsolidationPlan types (shared)

**Files:**
- Create: `ywai/internal/engram/plan.go`

- [ ] **Step 1: Write plan.go**

This type is produced by the memory agent (as JSON) and consumed by the consolidation manager + handlers + frontend.

```go
package engram

// ConsolidationPlan is the structured output the memory agent produces when
// asked to consolidate memories. The user reviews it and selectively applies
// items via the backend (never directly by the agent).
type ConsolidationPlan struct {
	Updates      []PlanUpdate  `json:"updates,omitempty"`
	Deletes      []PlanDelete  `json:"deletes,omitempty"`
	NewSummaries []PlanSummary `json:"new_summaries,omitempty"`
	Digest       string        `json:"digest,omitempty"`
}

// PlanUpdate proposes modifying an existing observation.
type PlanUpdate struct {
	ObservationID  string  `json:"observation_id"`
	Reason         string  `json:"reason"`
	NewContent     string  `json:"new_content,omitempty"`
	NewImportance  float64 `json:"new_importance,omitempty"`
}

// PlanDelete proposes removing an observation (duplicate, obsolete).
type PlanDelete struct {
	ObservationID string `json:"observation_id"`
	Reason        string `json:"reason"`
}

// PlanSummary proposes creating a new summary/topic observation.
type PlanSummary struct {
	Type       string                 `json:"type"`
	Content    string                 `json:"content"`
	Importance float64                `json:"importance"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}
```

- [ ] **Step 2: Verify build**

Run: `cd ywai && go build ./internal/engram/`
Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add ywai/internal/engram/plan.go
git commit -m "feat(engram): add ConsolidationPlan types"
```

---

## Task 5: engram HTTP client tests

**Files:**
- Create: `ywai/internal/engram/http_client_test.go`
- Create: `ywai/internal/engram/factory_test.go`

These follow the exact style of `internal/opencode/server_client_test.go` (httptest + table-ish cases).

- [ ] **Step 1: Write http_client_test.go**

```go
package engram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPClient_Status_Connected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL)
	st, err := c.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if !st.Connected {
		t.Fatal("Expected Connected=true")
	}
}

func TestHTTPClient_Status_Unavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv.Close() // closed → connection refused

	c := NewHTTPClient(srv.URL)
	_, err := c.Status(context.Background())
	if err != ErrEngramUnavailable {
		t.Fatalf("Expected ErrEngramUnavailable, got %v", err)
	}
}

func TestHTTPClient_RecentObservations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/observations/recent" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("limit") != "5" {
			t.Errorf("expected limit=5, got %q", r.URL.Query().Get("limit"))
		}
		resp := []map[string]interface{}{
			{"id": "obs_1", "type": "save", "content": "first", "importance": 8.0},
			{"id": "obs_2", "type": "observation", "content": "second"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL)
	obs, err := c.RecentObservations(context.Background(), 5)
	if err != nil {
		t.Fatalf("RecentObservations() error: %v", err)
	}
	if len(obs) != 2 || obs[0].ID != "obs_1" {
		t.Fatalf("Unexpected observations: %+v", obs)
	}
}

func TestHTTPClient_RecentObservations_WrappedData(t *testing.T) {
	// engram may wrap arrays in {"data": [...]}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{{"id": "w1", "content": "wrapped"}},
		})
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL)
	obs, err := c.RecentObservations(context.Background(), 0)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(obs) != 1 || obs[0].ID != "w1" {
		t.Fatalf("Unexpected: %+v", obs)
	}
}

func TestHTTPClient_Save(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/save" || r.Method != http.MethodPost {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "obs_new", "type": "save", "content": "captured",
		})
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL)
	obs, err := c.Save(context.Background(), SaveRequest{
		Type: "save", Content: "captured", Importance: 7,
	})
	if err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	if obs.ID != "obs_new" {
		t.Fatalf("Unexpected: %+v", obs)
	}
	if gotBody["type"] != "save" || gotBody["content"] != "captured" {
		t.Fatalf("Unexpected body: %+v", gotBody)
	}
}

func TestHTTPClient_DeleteObservation(t *testing.T) {
	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/observations/obs_9" && r.Method == http.MethodDelete {
			called = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL)
	if err := c.DeleteObservation(context.Background(), "obs_9"); err != nil {
		t.Fatalf("DeleteObservation() error: %v", err)
	}
	if !called {
		t.Fatal("DELETE not called")
	}
}

func TestHTTPClient_Search(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("q") != "react" {
			t.Errorf("expected q=react, got %q", r.URL.Query().Get("q"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": "s1", "content": "react 19 stuff"},
		})
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL)
	obs, err := c.Search(context.Background(), SearchRequest{Query: "react", Limit: 10})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(obs) != 1 || obs[0].ID != "s1" {
		t.Fatalf("Unexpected: %+v", obs)
	}
}

func TestHTTPClient_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL)
	_, err := c.GetStats(context.Background())
	if err == nil {
		t.Fatal("Expected error for 500")
	}
}
```

- [ ] **Step 2: Write factory_test.go**

```go
package engram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestProbeServer_Reachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if !ProbeServer(context.Background(), srv.URL) {
		t.Fatal("Expected ProbeServer=true")
	}
}

func TestProbeServer_Unreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	if ProbeServer(context.Background(), srv.URL) {
		t.Fatal("Expected ProbeServer=false")
	}
}

func TestDefaultClient_EnvOverride(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv("ENGRAM_URL", srv.URL)
	c := DefaultClient()
	if c.baseURL != srv.URL {
		t.Fatalf("Expected baseURL=%s, got %s", srv.URL, c.baseURL)
	}
	_ = os.Unsetenv("ENGRAM_URL")
}
```

- [ ] **Step 3: Run tests**

Run: `cd ywai && go test ./internal/engram/... -v`
Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add ywai/internal/engram/http_client_test.go ywai/internal/engram/factory_test.go
git commit -m "test(engram): add HTTP client + factory tests"
```

---

## Task 6: ConsolidationPlan JSON parse test

**Files:**
- Modify: `ywai/internal/engram/http_client_test.go` (append a test) — or add to factory_test.go. To keep it focused, append to http_client_test.go.

- [ ] **Step 1: Append the plan parse test**

Add to `http_client_test.go`:

```go
func TestConsolidationPlan_Parse(t *testing.T) {
	raw := `{
		"updates": [{"observation_id":"o1","reason":"dup","new_content":"x","new_importance":9}],
		"deletes": [{"observation_id":"o2","reason":"obsolete"}],
		"new_summaries": [{"type":"summary","content":"topic Z","importance":8}],
		"digest": "system knows A,B"
	}`
	var plan ConsolidationPlan
	if err := json.Unmarshal([]byte(raw), &plan); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(plan.Updates) != 1 || plan.Updates[0].ObservationID != "o1" {
		t.Fatalf("Unexpected updates: %+v", plan.Updates)
	}
	if len(plan.Deletes) != 1 || plan.Deletes[0].ObservationID != "o2" {
		t.Fatalf("Unexpected deletes: %+v", plan.Deletes)
	}
	if len(plan.NewSummaries) != 1 || plan.NewSummaries[0].Type != "summary" {
		t.Fatalf("Unexpected summaries: %+v", plan.NewSummaries)
	}
	if plan.Digest != "system knows A,B" {
		t.Fatalf("Unexpected digest: %q", plan.Digest)
	}
}
```

- [ ] **Step 2: Run tests**

Run: `cd ywai && go test ./internal/engram/... -run TestConsolidationPlan -v`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add ywai/internal/engram/http_client_test.go
git commit -m "test(engram): add ConsolidationPlan parse test"
```

---

## Task 7: memory agent (AGENT.md + permissions.json + groups.json)

**Files:**
- Create: `ywai/agents/core/memory/AGENT.md`
- Create: `ywai/agents/core/memory/permissions.json`
- Modify: `ywai/agents/groups.json`

- [ ] **Step 1: Create AGENT.md**

```markdown
---
name: memory
description: >
  Memory consolidation specialist. Analyzes engram memories and produces a
  structured consolidation plan (updates, deletes, new summaries) for human
  review. Read-only: never writes memories directly.
role: memory
mode: all
---

# Memory Agent

You are a memory-consolidation specialist. You receive the current memory
context (observations + recent sessions) and produce a **single** consolidation
plan as a JSON object. You do NOT modify memories yourself — the backend applies
the user-approved plan.

## Core Principles

1. **Never invent content.** Only reorganize, summarize, or flag *existing*
   observations. A summary must faithfully reflect real source observations.
2. **Be conservative with deletes.** Only propose deleting an observation when
   it is an exact/near-duplicate of another, or demonstrably obsolete
   (superseded by a newer observation).
3. **Explain every change.** Each item in `updates`/`deletes`/`new_summaries`
   must include a short `reason`.
4. **Output JSON only.** Respond with exactly one JSON object matching the
   schema below — no prose before or after, no markdown fences.

## Output schema

```
{
  "updates": [
    {
      "observation_id": "<existing id>",
      "reason": "<why>",
      "new_content": "<optional rewritten content>",
      "new_importance": 0
    }
  ],
  "deletes": [
    { "observation_id": "<existing id>", "reason": "<duplicate | obsolete | ...>" }
  ],
  "new_summaries": [
    {
      "type": "summary" | "topic",
      "content": "<concise consolidated summary>",
      "importance": 1,
      "metadata": {}
    }
  ],
  "digest": "<one-paragraph executive summary of what the system currently knows>"
}
```

- Omit empty arrays (`"updates": []` is fine, or omit the key).
- `new_importance` is 0–10; reuse the source observation's importance unless the
  change warrants adjusting it.

## Boundaries

- ✅ Read memories via engram context/search tools.
- ✅ Propose a structured plan.
- ❌ Do NOT call engram write tools (`mem_save`, `mem_update`, `mem_session_*`).
- ❌ Do NOT edit project files.
- ❌ Do NOT output anything other than the JSON object.
```

- [ ] **Step 2: Create permissions.json**

```json
{
  "read": "allow",
  "glob": "allow",
  "grep": "allow",
  "code_search": "allow",
  "task": "allow",
  "question": "allow",
  "skill": "allow",
  "memory": "allow",
  "mcp": "allow",
  "mcp:read": "allow",
  "mcp:write": "deny",
  "mcp:admin": "deny",
  "edit": "deny",
  "write": "deny",
  "bash": "deny",
  "delegate": "ask"
}
```

> Note: engram tools are exposed via MCP, so `mcp:read` allow grants the agent
> its needed read tools (`engram_mem_context`, `engram_mem_search`,
> `engram_mem_get_observation`). `mcp:write` deny keeps it from calling the
> engram write tools directly — the backend applies changes after review.

- [ ] **Step 3: Add `memory` to the core group in groups.json**

In `ywai/agents/groups.json`, the `core.agents` array currently is:
```json
["orchestrator","ask","dev","qa","architect","reviewer","devops","finder"]
```
Append `"memory"` so it becomes:
```json
["orchestrator","ask","dev","qa","architect","reviewer","devops","finder","memory"]
```

- [ ] **Step 4: Validate JSON**

Run: `cd ywai && python3 -c "import json; json.load(open('agents/groups.json'))" && python3 -c "import json; json.load(open('agents/core/memory/permissions.json'))"`
Expected: no output (valid JSON).

- [ ] **Step 5: Commit**

```bash
git add ywai/agents/core/memory/ ywai/agents/groups.json
git commit -m "feat(agents): add memory consolidation agent (read-only)"
```

---

## Task 8: ConsolidationManager — in-memory runs + driver

**Files:**
- Create: `ywai/internal/missions/web/consolidation.go`

This file defines the `ConsolidationManager`, `ConsolidationRun`, and the runner
contract. The manager depends on:
- `engram.Client` (to fetch context + apply changes)
- `opencode.SessionAPI` (to run the memory agent)
- a broadcast callback (`func(eventType string, payload any)`)

- [ ] **Step 1: Write consolidation.go**

```go
package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/engram"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/opencode"
)

// Consolidation status values.
const (
	StatusRunning          = "running"
	StatusAwaitingReview   = "awaiting_review"
	StatusApplying         = "applying"
	StatusApplied          = "applied"
	StatusDiscarded        = "discarded"
	StatusFailed           = "failed"
)

// ConsolidationRun tracks one consolidation lifecycle.
type ConsolidationRun struct {
	ID        string                 `json:"id"`
	Model     string                 `json:"model"`
	Agent     string                 `json:"agent"`
	Status    string                 `json:"status"`
	Plan      *engram.ConsolidationPlan `json:"plan,omitempty"`
	Digest    string                 `json:"digest,omitempty"`
	SessionID string                 `json:"sessionID,omitempty"`
	Error     string                 `json:"error,omitempty"`
	StartedAt time.Time              `json:"startedAt"`
	UpdatedAt time.Time              `json:"updatedAt"`
}

// ApplySelection is the body of POST /consolidations/{id}/apply: the subsets of
// the plan the user accepted.
type ApplySelection struct {
	Updates      []engram.PlanUpdate  `json:"updates"`
	Deletes      []engram.PlanDelete  `json:"deletes"`
	NewSummaries []engram.PlanSummary `json:"new_summaries"`
}

// ConsolidationManager owns in-memory consolidation runs.
type ConsolidationManager struct {
	mu       sync.RWMutex
	runs     map[string]*ConsolidationRun
	engram   engram.Client
	sessions func() opencode.SessionAPI // resolved lazily so a late opencode start works
	broadcast func(eventType string, payload any)
}

// NewConsolidationManager creates a manager. sessions may return nil if the
// opencode server isn't up yet; runs will then fail with a clear error.
func NewConsolidationManager(e engram.Client, sessions func() opencode.SessionAPI, broadcast func(string, any)) *ConsolidationManager {
	return &ConsolidationManager{
		runs:     make(map[string]*ConsolidationRun),
		engram:   e,
		sessions: sessions,
		broadcast: broadcast,
	}
}

// Get returns a snapshot copy of a run.
func (m *ConsolidationManager) Get(id string) (ConsolidationRun, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.runs[id]
	if !ok {
		return ConsolidationRun{}, false
	}
	return *r, true
}

// Start creates a run and kicks off the driver goroutine. Returns the run ID.
func (m *ConsolidationManager) Start(ctx context.Context, model, agent string) (string, error) {
	if m.engram == nil {
		return "", errors.New("engram client not configured")
	}
	id := newRunID()
	run := &ConsolidationRun{
		ID:        id,
		Model:     model,
		Agent:     agent,
		Status:    StatusRunning,
		StartedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.mu.Lock()
	m.runs[id] = run
	m.mu.Unlock()

	m.emit(id, StatusRunning, nil)
	go m.drive(context.Background(), run) // detached; uses bg ctx so request can return
	return id, nil
}

// Apply applies a user-approved selection and transitions to applied.
func (m *ConsolidationManager) Apply(ctx context.Context, id string, sel ApplySelection) error {
	m.mu.Lock()
	r, ok := m.runs[id]
	if !ok {
		m.mu.Unlock()
		return errors.New("consolidation run not found")
	}
	if r.Status != StatusAwaitingReview {
		m.mu.Unlock()
		return fmt.Errorf("run is %s, not awaiting_review", r.Status)
	}
	r.Status = StatusApplying
	r.UpdatedAt = time.Now()
	m.mu.Unlock()
	m.emit(id, StatusApplying, nil)

	// Apply deletes first (so updates to surviving obs are clean), then updates,
	// then new summaries.
	for _, d := range sel.Deletes {
		if err := m.engram.DeleteObservation(ctx, d.ObservationID); err != nil {
			m.fail(id, fmt.Sprintf("delete %s: %v", d.ObservationID, err))
			return err
		}
	}
	for _, u := range sel.Updates {
		req := engram.UpdateRequest{}
		if u.NewContent != "" {
			req.Content = &u.NewContent
		}
		if u.NewImportance != 0 {
			req.Importance = &u.NewImportance
		}
		if _, err := m.engram.UpdateObservation(ctx, u.ObservationID, req); err != nil {
			m.fail(id, fmt.Sprintf("update %s: %v", u.ObservationID, err))
			return err
		}
	}
	for _, s := range sel.NewSummaries {
		if _, err := m.engram.Save(ctx, engram.SaveRequest{
			Type: s.Type, Content: s.Content, Importance: s.Importance, Metadata: s.Metadata,
		}); err != nil {
			m.fail(id, fmt.Sprintf("save summary: %v", err))
			return err
		}
	}

	m.mu.Lock()
	r.Status = StatusApplied
	r.UpdatedAt = time.Now()
	m.mu.Unlock()
	m.emit(id, StatusApplied, nil)
	return nil
}

// Discard transitions a run to discarded without touching engram.
func (m *ConsolidationManager) Discard(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.runs[id]
	if !ok {
		return errors.New("consolidation run not found")
	}
	r.Status = StatusDiscarded
	r.UpdatedAt = time.Now()
	m.emit(id, StatusDiscarded, nil)
	return nil
}

// drive runs the opencode session: fetch context → create session → prompt →
// wait → parse plan from last assistant message.
func (m *ConsolidationManager) drive(ctx context.Context, run *ConsolidationRun) {
	sess := m.sessions()
	if sess == nil {
		m.fail(run.ID, "opencode server is not running; start it and retry")
		return
	}

	// 1. Fetch current memory context.
	ctxt, err := m.engram.GetContext(ctx, engram.ContextRequest{Limit: 200})
	if err != nil {
		m.fail(run.ID, fmt.Sprintf("fetch context: %v", err))
		return
	}

	// 2. Build the prompt.
	prompt := buildConsolidationPrompt(ctxt)

	// 3. Create the opencode session.
	agent := run.Agent
	if agent == "" {
		agent = "memory"
	}
	opts := opencode.SessionCreateOpts{
		Title: "Consolidation " + run.ID,
		Agent: agent,
	}
	if run.Model != "" {
		opts.Model = parseModelInput(run.Model)
	}
	session, err := sess.Create(ctx, opts)
	if err != nil {
		m.fail(run.ID, fmt.Sprintf("create session: %v", err))
		return
	}
	m.setSessionID(run.ID, session.ID)
	m.emit(run.ID, StatusRunning, map[string]any{"stage": "session_created"})

	// 4. Prompt + wait.
	if _, err := sess.Prompt(ctx, session.ID, opencode.PromptInput{Text: prompt, Delivery: "immediate"}); err != nil {
		m.fail(run.ID, fmt.Sprintf("prompt: %v", err))
		return
	}
	m.emit(run.ID, StatusRunning, map[string]any{"stage": "agent_working"})
	if err := sess.Wait(ctx, session.ID); err != nil {
		m.fail(run.ID, fmt.Sprintf("wait: %v", err))
		return
	}

	// 5. Parse plan from last assistant message.
	msgs, err := sess.Messages(ctx, session.ID)
	if err != nil {
		m.fail(run.ID, fmt.Sprintf("messages: %v", err))
		return
	}
	plan, err := extractPlan(msgs)
	if err != nil {
		m.fail(run.ID, fmt.Sprintf("parse plan: %v", err))
		return
	}

	m.mu.Lock()
	run.Plan = plan
	run.Digest = plan.Digest
	run.Status = StatusAwaitingReview
	run.UpdatedAt = time.Now()
	m.mu.Unlock()
	m.emit(run.ID, StatusAwaitingReview, map[string]any{"plan": plan})
}

// fail marks a run failed and emits.
func (m *ConsolidationManager) fail(id, msg string) {
	m.mu.Lock()
	if r, ok := m.runs[id]; ok {
		r.Status = StatusFailed
		r.Error = msg
		r.UpdatedAt = time.Now()
	}
	m.mu.Unlock()
	m.emit(id, StatusFailed, map[string]any{"error": msg})
}

func (m *ConsolidationManager) setSessionID(id, sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.runs[id]; ok {
		r.SessionID = sessionID
	}
}

func (m *ConsolidationManager) emit(id, eventType string, extra map[string]any) {
	if m.broadcast == nil {
		return
	}
	payload := map[string]any{"run_id": id, "status": eventType}
	for k, v := range extra {
		payload[k] = v
	}
	m.broadcast("consolidation."+mapStageToEvent(eventType), payload)
}

// mapStageToEvent normalises internal status strings to event suffixes.
func mapStageToEvent(status string) string {
	switch status {
	case StatusRunning:
		return "progress"
	case StatusAwaitingReview:
		return "completed"
	case StatusApplied:
		return "applied"
	case StatusFailed:
		return "failed"
	case StatusDiscarded:
		return "discarded"
	default:
		return status
	}
}

// buildConsolidationPrompt renders the agent instructions + current memory.
func buildConsolidationPrompt(ctxt engram.ContextResult) string {
	var b strings.Builder
	b.WriteString("Analyze the following engram memories and produce a single ")
	b.WriteString("consolidation plan as JSON (updates, deletes, new_summaries, digest). ")
	b.WriteString("Output JSON only — no prose, no markdown fences.\n\n")
	b.WriteString("## Current memory context\n\n")
	if ctxt.Summary != "" {
		b.WriteString("Summary: " + ctxt.Summary + "\n\n")
	}
	if len(ctxt.Observations) > 0 {
		data, _ := json.MarshalIndent(ctxt.Observations, "", "  ")
		b.WriteString("Observations:\n")
		b.Write(data)
		b.WriteString("\n")
	}
	if ctxt.Raw != nil {
		data, _ := json.MarshalIndent(ctxt.Raw, "", "  ")
		b.WriteString("Raw context:\n")
		b.Write(data)
		b.WriteString("\n")
	}
	return b.String()
}

// extractPlan finds the last assistant message and parses its JSON plan.
func extractPlan(msgs []opencode.Message) (*engram.ConsolidationPlan, error) {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role != "assistant" || strings.TrimSpace(msgs[i].Text) == "" {
			continue
		}
		text := extractJSONBlock(msgs[i].Text)
		var plan engram.ConsolidationPlan
		if err := json.Unmarshal([]byte(text), &plan); err != nil {
			return nil, fmt.Errorf("invalid plan JSON: %w", err)
		}
		return &plan, nil
	}
	return nil, errors.New("no assistant message found")
}

// extractJSONBlock pulls the first {...} JSON object out of text that may be
// wrapped in markdown fences or surrounded by stray prose.
func extractJSONBlock(text string) string {
	text = strings.TrimSpace(text)
	// Strip ```json ... ``` fences if present.
	if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimSuffix(text, "```")
		text = strings.TrimSpace(text)
	}
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		return text[start : end+1]
	}
	return text
}

// parseModelInput converts a "provider/model" or "model" string into a
// SessionCreateOpts.Model. It reuses opencode's ModelInput shape.
func parseModelInput(s string) *opencode.ModelInput {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	mi := &opencode.ModelInput{ID: s}
	if idx := strings.Index(s, "/"); idx > 0 {
		mi.ProviderID = s[:idx]
		mi.ID = s[idx+1:]
	}
	return mi
}

func newRunID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return "con_" + hex.EncodeToString(b)
}
```

- [ ] **Step 2: Verify build**

Run: `cd ywai && go build ./internal/missions/web/`
Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add ywai/internal/missions/web/consolidation.go
git commit -m "feat(memories): add ConsolidationManager (in-memory runs + opencode driver)"
```

---

## Task 9: ConsolidationManager tests (fake SessionAPI)

**Files:**
- Create: `ywai/internal/missions/web/consolidation_test.go`

A fake `opencode.SessionAPI` + a fake `engram.Client` drive the manager deterministically.

- [ ] **Step 1: Write the fake session API + tests**

```go
package web

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/engram"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/opencode"
)

// fakeSessionAPI lets the test script a single consolidation run.
type fakeSessionAPI struct {
	mu       sync.Mutex
	prompted bool
msgs []opencode.Message
	createErr error
}

func (f *fakeSessionAPI) Create(ctx context.Context, opts opencode.SessionCreateOpts) (*opencode.Session, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	return &opencode.Session{ID: "sess_1", Title: opts.Title, Agent: opts.Agent}, nil
}
func (f *fakeSessionAPI) Get(ctx context.Context, id string) (*opencode.Session, error) {
	return &opencode.Session{ID: id}, nil
}
func (f *fakeSessionAPI) Status(ctx context.Context) (*opencode.SessionStatusResult, error) {
	return &opencode.SessionStatusResult{}, nil
}
func (f *fakeSessionAPI) Prompt(ctx context.Context, id string, in opencode.PromptInput) (*opencode.PromptResult, error) {
	f.mu.Lock()
	f.prompted = true
	f.mu.Unlock()
	return &opencode.PromptResult{MessageID: "msg_1", SessionID: id}, nil
}
func (f *fakeSessionAPI) Wait(ctx context.Context, id string) error { return nil }
func (f *fakeSessionAPI) Messages(ctx context.Context, id string) ([]opencode.Message, error) {
	return f.msgs, nil
}
func (f *fakeSessionAPI) Delete(ctx context.Context, id string) error { return nil }
func (f *fakeSessionAPI) ListQuestions(ctx context.Context) ([]opencode.Question, error) {
	return nil, nil
}
func (f *fakeSessionAPI) ReplyQuestion(ctx context.Context, qid, answer string) error { return nil }

// fakeEngramClient is a minimal engram.Client for manager tests.
type fakeEngramClient struct {
	deletedIDs  []string
	saved       []engram.SaveRequest
	updatedIDs  []string
	contextErr  error
}

func (f *fakeEngramClient) Status(ctx context.Context) (engram.Status, error) {
	return engram.Status{Connected: true}, nil
}
func (f *fakeEngramClient) RecentObservations(ctx context.Context, limit int) ([]engram.Observation, error) {
	return nil, nil
}
func (f *fakeEngramClient) GetObservation(ctx context.Context, id string) (engram.Observation, error) {
	return engram.Observation{ID: id}, nil
}
func (f *fakeEngramClient) UpdateObservation(ctx context.Context, id string, req engram.UpdateRequest) (engram.Observation, error) {
	f.updatedIDs = append(f.updatedIDs, id)
	return engram.Observation{ID: id}, nil
}
func (f *fakeEngramClient) DeleteObservation(ctx context.Context, id string) error {
	f.deletedIDs = append(f.deletedIDs, id)
	return nil
}
func (f *fakeEngramClient) Save(ctx context.Context, req engram.SaveRequest) (engram.Observation, error) {
	f.saved = append(f.saved, req)
	return engram.Observation{ID: "new", Type: req.Type}, nil
}
func (f *fakeEngramClient) Search(ctx context.Context, req engram.SearchRequest) ([]engram.Observation, error) {
	return nil, nil
}
func (f *fakeEngramClient) GetStats(ctx context.Context) (engram.Stats, error) {
	return engram.Stats{}, nil
}
func (f *fakeEngramClient) RecentSessions(ctx context.Context, limit int) ([]engram.Session, error) {
	return nil, nil
}
func (f *fakeEngramClient) Timeline(ctx context.Context, req engram.TimelineRequest) ([]engram.TimelineEvent, error) {
	return nil, nil
}
func (f *fakeEngramClient) GetContext(ctx context.Context, req engram.ContextRequest) (engram.ContextResult, error) {
	if f.contextErr != nil {
		return engram.ContextResult{}, f.contextErr
	}
	return engram.ContextResult{
		Observations: []engram.Observation{{ID: "o1", Content: "dup"}, {ID: "o2", Content: "dup"}},
	}, nil
}

func TestConsolidationManager_FullRun(t *testing.T) {
	fe := &fakeEngramClient{}
	fs := &fakeSessionAPI{
		msgs: []opencode.Message{
			{Role: "user", Text: "go"},
			{Role: "assistant", Text: "```json\n{\"deletes\":[{\"observation_id\":\"o1\",\"reason\":\"dup\"}],\"new_summaries\":[{\"type\":\"summary\",\"content\":\"one\",\"importance\":7}],\"digest\":\"x\"}\n```"},
		},
	}
	events := []string{}
	mgr := NewConsolidationManager(fe, func() opencode.SessionAPI { return fs },
		func(et string, payload any) { events = append(events, et) })

	id, err := mgr.Start(context.Background(), "anthropic/claude", "memory")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait for the driver to reach awaiting_review.
	waitForStatus(t, mgr, id, StatusAwaitingReview, 50)

	// Apply a subset.
	err = mgr.Apply(context.Background(), id, ApplySelection{
		Deletes:      []engram.PlanDelete{{ObservationID: "o1", Reason: "dup"}},
		NewSummaries: []engram.PlanSummary{{Type: "summary", Content: "one", Importance: 7}},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	run, _ := mgr.Get(id)
	if run.Status != StatusApplied {
		t.Fatalf("expected applied, got %s", run.Status)
	}
	if len(fe.deletedIDs) != 1 || fe.deletedIDs[0] != "o1" {
		t.Fatalf("expected delete o1, got %v", fe.deletedIDs)
	}
	if len(fe.saved) != 1 {
		t.Fatalf("expected 1 save, got %v", fe.saved)
	}
}

func TestConsolidationManager_NoSession(t *testing.T) {
	fe := &fakeEngramClient{}
	mgr := NewConsolidationManager(fe, func() opencode.SessionAPI { return nil }, nil)

	id, err := mgr.Start(context.Background(), "m", "memory")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForStatus(t, mgr, id, StatusFailed, 50)
	run, _ := mgr.Get(id)
	if run.Error == "" {
		t.Fatal("expected an error message")
	}
}

func TestExtractPlan_BadJSON(t *testing.T) {
	_, err := extractPlan([]opencode.Message{{Role: "assistant", Text: "not json"}})
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestParseModelInput(t *testing.T) {
	mi := parseModelInput("anthropic/claude-sonnet-4")
	if mi.ProviderID != "anthropic" || mi.ID != "claude-sonnet-4" {
		t.Fatalf("unexpected: %+v", mi)
	}
	mi2 := parseModelInput("bare-model")
	if mi2.ProviderID != "" || mi2.ID != "bare-model" {
		t.Fatalf("unexpected: %+v", mi2)
	}
}

// waitForStatus polls the manager until the run reaches the wanted status or
// the attempt budget is exhausted.
func waitForStatus(t *testing.T, mgr *ConsolidationManager, id, want string, attempts int) {
	t.Helper()
	for i := 0; i < attempts; i++ {
		run, ok := mgr.Get(id)
		if ok && run.Status == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("run %s never reached %s", id, want)
}
```

> The `waitForStatus` helper uses `time.Sleep`, so `"time"` must be in the import block (already included above).

- [ ] **Step 2: Run tests**

Run: `cd ywai && go test ./internal/missions/web/... -run 'Consolidation|ExtractPlan|ParseModelInput' -v`
Expected: all PASS.

- [ ] **Step 3: Commit**

```bash
git add ywai/internal/missions/web/consolidation_test.go
git commit -m "test(memories): add ConsolidationManager tests with fake SessionAPI"
```

---

## Task 10: Wire engram + consolidation into Handlers + register routes

**Files:**
- Modify: `ywai/internal/missions/web/handlers.go` (struct fields + handler methods)
- Modify: `ywai/internal/missions/web/server.go` (New() wiring + registerRoutes)

- [ ] **Step 1: Add fields to the Handlers struct**

In `ywai/internal/missions/web/handlers.go`, add these fields to the `Handlers` struct (right after `opencodeClient opencode.Client`):

```go
	engramClient    engram.Client
	consolidations  *ConsolidationManager
```

And add the import `"github.com/Yoizen/dev-ai-workflow/ywai/internal/engram"` to the import block.

- [ ] **Step 2: Wire in New() and register routes (server.go)**

In `ywai/internal/missions/web/server.go`:

a) In `New()`, after building `h` and before `s.handlers = h`, construct the engram client + consolidation manager and attach them:

```go
	engramClient := engram.DefaultClient()
	h.consolidations = NewConsolidationManager(
		engramClient,
		func() opencode.SessionAPI { return h.opencodeClient.Sessions() },
		func(et string, payload any) { s.hub.BroadcastEvent(et, payload) },
	)
	h.engramClient = engramClient
```

b) In `registerRoutes`, after the OpenCode config block and before `// AI refinement`, add:

```go
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
	mux.HandleFunc("GET /api/engram/timeline", h.EngramTimeline)
	mux.HandleFunc("GET /api/engram/context", h.EngramContext)

	// Consolidations
	mux.HandleFunc("POST /api/engram/consolidations", h.StartConsolidation)
	mux.HandleFunc("GET /api/engram/consolidations/{id}", h.GetConsolidation)
	mux.HandleFunc("POST /api/engram/consolidations/{id}/apply", h.ApplyConsolidation)
	mux.HandleFunc("POST /api/engram/consolidations/{id}/discard", h.DiscardConsolidation)

	// Engram WebSocket (separate path so it isn't caught by /ws mission handlers)
	mux.HandleFunc("GET /engram/ws", h.HandleEngramWebSocket)
```

- [ ] **Step 3: Verify build**

Run: `cd ywai && go build ./internal/missions/web/`
Expected: no output.

- [ ] **Step 4: Commit**

```bash
git add ywai/internal/missions/web/handlers.go ywai/internal/missions/web/server.go
git commit -m "feat(memories): wire engram client + consolidation manager into Handlers"
```

---

## Task 11: engram handler methods

**Files:**
- Create: `ywai/internal/missions/web/engram_handlers.go`

All handlers reuse `writeJSON`/`writeError` from `server.go`. Each handler that
needs engram checks `h.engramClient == nil` and returns 503.

- [ ] **Step 1: Write engram_handlers.go**

```go
package web

import (
	"net/http"
	"strconv"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/engram"
)

func (h *Handlers) requireEngram(w http.ResponseWriter) (engram.Client, bool) {
	if h.engramClient == nil {
		writeError(w, http.StatusServiceUnavailable, "engram client not configured")
		return nil, false
	}
	return h.engramClient, true
}

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
	id := r.PathValue("id")
	obs, err := c.GetObservation(r.Context(), id)
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

// EngramTimeline → GET /api/engram/timeline?limit=
func (h *Handlers) EngramTimeline(w http.ResponseWriter, r *http.Request) {
	c, ok := h.requireEngram(w)
	if !ok {
		return
	}
	events, err := c.Timeline(r.Context(), engram.TimelineRequest{Limit: queryLimit(r, 50, 500)})
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

// decodeJSONBody is a tiny helper used by the write handlers.
func decodeJSONBody(r *http.Request, target any) error {
	return json.NewDecoder(r.Body).Decode(target)
}
```

> `decodeJSONBody` uses `json` which is already imported in `handlers.go` but this file is separate, so add `"encoding/json"` to this file's imports.

- [ ] **Step 2: Add encoding/json import**

The `engram_handlers.go` import block should be:

```go
import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/engram"
)
```

- [ ] **Step 3: Verify build**

Run: `cd ywai && go build ./internal/missions/web/`
Expected: no output.

- [ ] **Step 4: Commit**

```bash
git add ywai/internal/missions/web/engram_handlers.go
git commit -m "feat(memories): add engram CRUD/search/stats/sessions/timeline handlers"
```

---

## Task 12: consolidation + WebSocket handlers

**Files:**
- Modify: `ywai/internal/missions/web/engram_handlers.go` (append consolidation + WS handlers)

- [ ] **Step 1: Append consolidation handlers**

Append to `engram_handlers.go`:

```go
// ─── Consolidation handlers ─────────────────────────────────────────────────

// StartConsolidation → POST /api/engram/consolidations {model, agent}
func (h *Handlers) StartConsolidation(w http.ResponseWriter, r *http.Request) {
	if h.consolidations == nil {
		writeError(w, http.StatusServiceUnavailable, "consolidation manager not configured")
		return
	}
	var req struct {
		Model string `json:"model"`
		Agent string `json:"agent"`
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
	id, err := h.consolidations.Start(r.Context(), req.Model, req.Agent)
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
// All consolidation events are broadcast via hub.BroadcastEvent, so this handler
// only needs to keep the client subscribed.
func (h *Handlers) HandleEngramWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := &Client{conn: conn, send: make(chan []byte, 256)}
	h.hub.register <- client
	defer func() {
		h.hub.unregister <- client
		conn.Close()
	}()
	// Read loop: discard incoming (clients don't send), exit on close.
	go func() {
		defer func() { h.hub.unregister <- client }()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
	// Write loop: pump queued broadcasts to the client.
	for msg := range client.send {
		if err := conn.WriteMessage(1, msg); err != nil {
			return
		}
	}
}
```

- [ ] **Step 2: Verify the Hub client type exists**

Run: `cd ywai && grep -n "type Client struct" internal/missions/web/hub.go && grep -n "register\|unregister" internal/missions/web/hub.go | head`
Expected: shows `Client` struct + `register`/`unregister` channels. If the hub's API differs (e.g. method names), adjust `HandleEngramWebSocket` to match — the existing `HandleWebSocket` in `handlers.go` is the reference: open it and mirror its connection lifecycle exactly.

If `HandleWebSocket` already implements the lifecycle with a helper, reuse that helper instead of the inline version above. Replace the body of `HandleEngramWebSocket` with a call to the same underlying registration logic that `HandleWebSocket` uses.

- [ ] **Step 3: Verify build**

Run: `cd ywai && go build ./internal/missions/web/`
Expected: no output. If the Hub API differs, fix until it compiles.

- [ ] **Step 4: Commit**

```bash
git add ywai/internal/missions/web/engram_handlers.go
git commit -m "feat(memories): add consolidation + WebSocket handlers"
```

---

## Task 13: engram handler tests (httptest + fake client)

**Files:**
- Create: `ywai/internal/missions/web/engram_handlers_test.go`

These build a `Handlers` with the fake engram client (reuse the fake from Task 9's test by copying it, since tests in the same package share code but the fake is defined in `_test.go` so it's available). Note: the fake in `consolidation_test.go` is in the same package `web`, so it's visible here.

- [ ] **Step 1: Write the handler tests**

```go
package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/engram"
)

func newEngramTestHandlers() *Handlers {
	h := &Handlers{}
	h.engramClient = &fakeEngramClient{}
	registerEngramTestRoutes(h)
	return h
}

// registerEngramTestRoutes registers only the engram routes on a fresh mux.
func registerEngramTestRoutes(h *Handlers) http.Handler {
	mux := http.NewServeMux()
	registerRoutes(mux, h) // reuse the real route table
	return recoveryMiddleware(json405Middleware(mux))
}

func TestHandler_EngramStatus(t *testing.T) {
	h := newEngramTestHandlers()
	srv := httptest.NewServer(h.engramTestHandler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/engram/status")
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()
	if body["connected"] != true {
		t.Fatalf("expected connected=true, got %v", body["connected"])
	}
}

func TestHandler_SaveObservation(t *testing.T) {
	fe := &fakeEngramClient{}
	h := &Handlers{engramClient: fe}
	handler := registerEngramTestRoutes(h)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/engram/save", "application/json",
		strings.NewReader(`{"type":"save","content":"hello","importance":5}`))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()
	if len(fe.saved) != 1 || fe.saved[0].Content != "hello" {
		t.Fatalf("expected 1 save 'hello', got %+v", fe.saved)
	}
}

func TestHandler_SaveObservation_Validation(t *testing.T) {
	h := &Handlers{engramClient: &fakeEngramClient{}}
	handler := registerEngramTestRoutes(h)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/engram/save", "application/json",
		strings.NewReader(`{"type":"save","content":""}`))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestHandler_DeleteObservation(t *testing.T) {
	fe := &fakeEngramClient{}
	h := &Handlers{engramClient: fe}
	handler := registerEngramTestRoutes(h)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/engram/observations/obs_3", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(fe.deletedIDs) != 1 || fe.deletedIDs[0] != "obs_3" {
		t.Fatalf("expected delete obs_3, got %v", fe.deletedIDs)
	}
}

func TestHandler_Search(t *testing.T) {
	h := &Handlers{engramClient: &fakeEngramClient{}}
	handler := registerEngramTestRoutes(h)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/engram/search?q=react")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestHandler_Search_MissingQ(t *testing.T) {
	h := &Handlers{engramClient: &fakeEngramClient{}}
	handler := registerEngramTestRoutes(h)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/engram/search")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestHandler_EngramNotConfigured(t *testing.T) {
	h := &Handlers{} // no engramClient
	handler := registerEngramTestRoutes(h)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/engram/observations")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// engramTestHandler returns the handler used by the status test (kept for the
// simple status-only case). It is just registerEngramTestRoutes.
func (h *Handlers) engramTestHandler() http.Handler {
	return registerEngramTestRoutes(h)
}

// Ensure the unused context import doesn't break the build if the compiler
// trims it — reference it here. (Trivial compile guard.)
var _ = context.Background
```

> If `registerRoutes` requires fields the test `Handlers` doesn't set (e.g. `store`), the call may panic only when those routes are *invoked*. Since these tests only hit `/api/engram/*`, that's fine. If `registerRoutes` itself dereferences nil at registration time, instead copy only the engram route lines into a small `registerEngramTestRoutes(mux, h)` helper in this test file.

- [ ] **Step 2: Run tests**

Run: `cd ywai && go test ./internal/missions/web/... -run 'Handler_' -v`
Expected: all PASS.

- [ ] **Step 3: Commit**

```bash
git add ywai/internal/missions/web/engram_handlers_test.go
git commit -m "test(memories): add engram handler tests"
```

---

## Task 14: SPA route whitelist for /memories

**Files:**
- Modify: `ywai/internal/control/server.go`

Without this, a refresh on `/memories` returns 404 instead of the SPA index.

- [ ] **Step 1: Add /memories to the SPA route check**

In `ywai/internal/control/server.go`, find the line in `serveSPA`:
```go
	if path == "/" || path == "/missions" || path == "/settings" {
```
Change it to:
```go
	if path == "/" || path == "/missions" || path == "/settings" || path == "/memories" {
```

- [ ] **Step 2: Verify build**

Run: `cd ywai && go build ./internal/control/`
Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add ywai/internal/control/server.go
git commit -m "feat(memories): add /memories to SPA route whitelist"
```

---

## Task 15: Full backend test + build

- [ ] **Step 1: Run all Go tests**

Run: `cd ywai && go test ./...`
Expected: all PASS. If a pre-existing test fails unrelated to this work, note it but ensure no NEW failures from engram/consolidation/web packages.

- [ ] **Step 2: Full build**

Run: `cd ywai && go build ./...`
Expected: no output.

- [ ] **Step 3: (No commit unless something changed)**

---

## Task 16: Frontend types

**Files:**
- Modify: `ywai/internal/control/web/src/api/types.ts`

- [ ] **Step 1: Append the Memories types section**

At the end of `types.ts`, add:

```ts
// ─── Memories Types ──────────────────────────────────────────────────────────

export interface EngramObservation {
  id: string
  type?: string
  content?: string
  importance?: number
  topic?: string
  sessionID?: string
  tags?: string[]
  metadata?: Record<string, unknown>
  createdAt?: string
  lastAccessed?: string
}

export interface EngramSession {
  id: string
  title?: string
  summary?: string
  start?: string
  end?: string
  topics?: string[]
}

export interface EngramStats {
  total: number
  byType?: Record<string, number>
  byImportance?: Record<string, number>
  avgImportance?: number
}

export interface EngramTimelineEvent {
  id: string
  type?: string
  content?: string
  createdAt?: string
}

export interface EngramContextResult {
  summary?: string
  observations?: EngramObservation[]
  raw?: unknown
}

export interface EngramStatus {
  connected: boolean
  source?: string
  version?: string
  error?: string
}

// Consolidation
export type ConsolidationStatus =
  | 'running'
  | 'awaiting_review'
  | 'applying'
  | 'applied'
  | 'discarded'
  | 'failed'

export interface PlanUpdate {
  observation_id: string
  reason: string
  new_content?: string
  new_importance?: number
}
export interface PlanDelete {
  observation_id: string
  reason: string
}
export interface PlanSummary {
  type: string
  content: string
  importance: number
  metadata?: Record<string, unknown>
}
export interface ConsolidationPlan {
  updates?: PlanUpdate[]
  deletes?: PlanDelete[]
  new_summaries?: PlanSummary[]
  digest?: string
}

export interface ConsolidationRun {
  id: string
  model: string
  agent: string
  status: ConsolidationStatus
  plan?: ConsolidationPlan
  digest?: string
  sessionID?: string
  error?: string
  startedAt?: string
  updatedAt?: string
}

export interface ApplySelection {
  updates: PlanUpdate[]
  deletes: PlanDelete[]
  new_summaries: PlanSummary[]
}
```

- [ ] **Step 2: Typecheck**

Run: `cd ywai/internal/control/web && npx tsc --noEmit`
Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add ywai/internal/control/web/src/api/types.ts
git commit -m "feat(memories): add TypeScript types for engram + consolidation"
```

---

## Task 17: memoriesApi client object

**Files:**
- Modify: `ywai/internal/control/web/src/api/client.ts`

- [ ] **Step 1: Add imports**

In the `import type { ... } from './types'` block at the top, add the new types:

```ts
  EngramObservation,
  EngramSession,
  EngramStats,
  EngramTimelineEvent,
  EngramContextResult,
  EngramStatus,
  ConsolidationRun,
  ConsolidationPlan,
  ApplySelection,
```

- [ ] **Step 2: Append memoriesApi**

At the end of `client.ts`, add:

```ts
// ─── Memories API ───────────────────────────────────────────────────────────

export const memoriesApi = {
  // Engram status
  status: () => request<EngramStatus>('/missions/api/engram/status'),

  // Observations
  listObservations: (limit = 50) =>
    request<{ observations: EngramObservation[] }>(
      `/missions/api/engram/observations?limit=${limit}`,
    ).then((r) => r.observations ?? []),
  getObservation: (id: string) =>
    request<EngramObservation>(`/missions/api/engram/observations/${id}`),
  updateObservation: (id: string, data: {
    content?: string
    importance?: number
    topic?: string
    metadata?: Record<string, unknown>
  }) =>
    request<EngramObservation>(`/missions/api/engram/observations/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),
  deleteObservation: (id: string) =>
    fetch(`/missions/api/engram/observations/${id}`, { method: 'DELETE' }).then(
      (r) => {
        if (!r.ok) throw new Error(`${r.status}`)
      },
    ),
  save: (data: {
    type: string
    content: string
    importance?: number
    topic?: string
    metadata?: Record<string, unknown>
  }) =>
    request<EngramObservation>('/missions/api/engram/save', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  // Search / stats / sessions / timeline / context
  search: (q: string, limit = 50, type?: string) =>
    request<{ observations: EngramObservation[] }>(
      `/missions/api/engram/search?q=${encodeURIComponent(q)}&limit=${limit}${
        type ? `&type=${encodeURIComponent(type)}` : ''
      }`,
    ).then((r) => r.observations ?? []),
  stats: () => request<EngramStats>('/missions/api/engram/stats'),
  listSessions: (limit = 20) =>
    request<{ sessions: EngramSession[] }>(
      `/missions/api/engram/sessions?limit=${limit}`,
    ).then((r) => r.sessions ?? []),
  timeline: (limit = 50) =>
    request<{ events: EngramTimelineEvent[] }>(
      `/missions/api/engram/timeline?limit=${limit}`,
    ).then((r) => r.events ?? []),
  context: (q?: string, limit = 100) =>
    request<EngramContextResult>(
      `/missions/api/engram/context?limit=${limit}${
        q ? `&q=${encodeURIComponent(q)}` : ''
      }`,
    ),

  // Consolidation
  startConsolidation: (data: { model: string; agent: string }) =>
    request<{ run_id: string; status: string }>(
      '/missions/api/engram/consolidations',
      { method: 'POST', body: JSON.stringify(data) },
    ),
  getConsolidation: (id: string) =>
    request<ConsolidationRun>(`/missions/api/engram/consolidations/${id}`),
  applyConsolidation: (id: string, sel: ApplySelection) =>
    request<{ status: string }>(
      `/missions/api/engram/consolidations/${id}/apply`,
      { method: 'POST', body: JSON.stringify(sel) },
    ),
  discardConsolidation: (id: string) =>
    request<{ status: string }>(
      `/missions/api/engram/consolidations/${id}/discard`,
      { method: 'POST' },
    ),
}
```

- [ ] **Step 3: Typecheck**

Run: `cd ywai/internal/control/web && npx tsc --noEmit`
Expected: no output.

- [ ] **Step 4: Commit**

```bash
git add ywai/internal/control/web/src/api/client.ts
git commit -m "feat(memories): add memoriesApi client object"
```

---

## Task 18: memoriesStore (Zustand)

**Files:**
- Create: `ywai/internal/control/web/src/stores/memoriesStore.ts`

- [ ] **Step 1: Write the store**

```ts
import { create } from 'zustand'
import type {
  EngramObservation,
  EngramSession,
  EngramStats,
  EngramTimelineEvent,
  EngramContextResult,
  EngramStatus,
  ConsolidationRun,
  ApplySelection,
  WSMessage,
} from '../api/types'
import { memoriesApi } from '../api/client'

interface MemoriesState {
  // data
  observations: EngramObservation[]
  selectedObservation: EngramObservation | null
  sessions: EngramSession[]
  timeline: EngramTimelineEvent[]
  context: EngramContextResult | null
  stats: EngramStats | null
  engramStatus: EngramStatus | null
  consolidation: ConsolidationRun | null

  // ui
  loading: boolean
  error: string | null

  // actions
  fetchStatus: () => Promise<void>
  fetchObservations: (limit?: number) => Promise<void>
  searchMemories: (q: string) => Promise<void>
  saveMemory: (data: {
    type: string
    content: string
    importance?: number
    topic?: string
  }) => Promise<void>
  updateObservation: (
    id: string,
    data: { content?: string; importance?: number },
  ) => Promise<void>
  deleteObservation: (id: string) => Promise<void>
  fetchStats: () => Promise<void>
  fetchSessions: () => Promise<void>
  fetchTimeline: () => Promise<void>
  fetchContext: (q?: string) => Promise<void>
  selectObservation: (obs: EngramObservation | null) => void

  startConsolidation: (model: string, agent: string) => Promise<string | null>
  applyConsolidation: (id: string, sel: ApplySelection) => Promise<void>
  discardConsolidation: (id: string) => Promise<void>

  handleWSMessage: (msg: WSMessage) => void
}

export const useMemoriesStore = create<MemoriesState>((set, get) => ({
  observations: [],
  selectedObservation: null,
  sessions: [],
  timeline: [],
  context: null,
  stats: null,
  engramStatus: null,
  consolidation: null,
  loading: false,
  error: null,

  fetchStatus: async () => {
    try {
      const status = await memoriesApi.status()
      set({ engramStatus: status })
    } catch {
      set({ engramStatus: { connected: false } })
    }
  },

  fetchObservations: async (limit = 50) => {
    set({ loading: true, error: null })
    try {
      const observations = await memoriesApi.listObservations(limit)
      set({ observations, loading: false })
    } catch (err) {
      set({ error: String(err), loading: false })
    }
  },

  searchMemories: async (q) => {
    set({ loading: true, error: null })
    try {
      const observations = q ? await memoriesApi.search(q) : await memoriesApi.listObservations()
      set({ observations, loading: false })
    } catch (err) {
      set({ error: String(err), loading: false })
    }
  },

  saveMemory: async (data) => {
    await memoriesApi.save(data)
    await get().fetchObservations()
    await get().fetchStats()
  },

  updateObservation: async (id, data) => {
    const updated = await memoriesApi.updateObservation(id, data)
    set((s) => ({
      observations: s.observations.map((o) => (o.id === id ? updated : o)),
      selectedObservation: s.selectedObservation?.id === id ? updated : s.selectedObservation,
    }))
  },

  deleteObservation: async (id) => {
    await memoriesApi.deleteObservation(id)
    set((s) => ({
      observations: s.observations.filter((o) => o.id !== id),
      selectedObservation: s.selectedObservation?.id === id ? null : s.selectedObservation,
    }))
    await get().fetchStats()
  },

  fetchStats: async () => {
    try {
      set({ stats: await memoriesApi.stats() })
    } catch (err) {
      set({ error: String(err) })
    }
  },

  fetchSessions: async () => {
    try {
      set({ sessions: await memoriesApi.listSessions() })
    } catch (err) {
      set({ error: String(err) })
    }
  },

  fetchTimeline: async () => {
    try {
      set({ timeline: await memoriesApi.timeline() })
    } catch (err) {
      set({ error: String(err) })
    }
  },

  fetchContext: async (q) => {
    try {
      set({ context: await memoriesApi.context(q) })
    } catch (err) {
      set({ error: String(err) })
    }
  },

  selectObservation: (obs) => set({ selectedObservation: obs }),

  startConsolidation: async (model, agent) => {
    try {
      const { run_id } = await memoriesApi.startConsolidation({ model, agent })
      set({ consolidation: { id: run_id, model, agent, status: 'running' } })
      return run_id
    } catch (err) {
      set({ error: String(err) })
      return null
    }
  },

  applyConsolidation: async (id, sel) => {
    await memoriesApi.applyConsolidation(id, sel)
    set({ consolidation: null })
    await get().fetchObservations()
    await get().fetchStats()
  },

  discardConsolidation: async (id) => {
    await memoriesApi.discardConsolidation(id)
    set({ consolidation: null })
  },

  handleWSMessage: (msg) => {
    if (!msg.type.startsWith('consolidation.')) return
    const c = get().consolidation
    const payload = (msg.payload ?? {}) as Record<string, unknown>
    const runId = payload.run_id as string
    if (c && c.id === runId) {
      const status = (payload.status as ConsolidationRun['status']) ?? c.status
      if (status === 'completed' || msg.type === 'consolidation.completed') {
        // Refresh the run to get the plan.
        memoriesApi.getConsolidation(runId).then((run) => set({ consolidation: run }))
      } else if (status === 'applied' || msg.type === 'consolidation.applied') {
        set({ consolidation: { ...c, status: 'applied' } })
        get().fetchObservations()
        get().fetchStats()
      } else if (status === 'failed' || msg.type === 'consolidation.failed') {
        set({ consolidation: { ...c, status: 'failed', error: String(payload.error ?? '') } })
      } else {
        set({ consolidation: { ...c, status: status === 'progress' ? 'running' : status } })
      }
    }
  },
}))
```

- [ ] **Step 2: Typecheck**

Run: `cd ywai/internal/control/web && npx tsc --noEmit`
Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add ywai/internal/control/web/src/stores/memoriesStore.ts
git commit -m "feat(memories): add memoriesStore (zustand)"
```

---

## Task 19: App.tsx route + Sidebar nav entry

**Files:**
- Modify: `ywai/internal/control/web/src/App.tsx`
- Modify: `ywai/internal/control/web/src/components/layout/Sidebar.tsx`

- [ ] **Step 1: Add the route in App.tsx**

Replace the `App` body:

```tsx
import { BrowserRouter as Router, Routes, Route } from 'react-router-dom'
import Layout from './components/layout/Layout'
import Kanban from './components/kanban/Kanban'
import Missions from './components/missions/Missions'
import Memories from './components/memories/Memories'
import Settings from './components/settings/Settings'

function App() {
  return (
    <Router>
      <Layout>
        <Routes>
          <Route path="/" element={<Kanban />} />
          <Route path="/missions" element={<Missions />} />
          <Route path="/memories" element={<Memories />} />
          <Route path="/settings" element={<Settings />} />
        </Routes>
      </Layout>
    </Router>
  )
}

export default App
```

- [ ] **Step 2: Add the Memories nav entry in Sidebar.tsx**

In the `NAV_ITEMS` array, after the Missions entry and before Settings, add:

```tsx
	{
		path: "/memories",
		label: "Memories",
		icon: (
			<svg
				width="19"
				height="19"
				viewBox="0 0 24 24"
				fill="none"
				stroke="currentColor"
				strokeWidth="2"
				strokeLinecap="round"
				strokeLinejoin="round"
			>
				<path d="M12 5a3 3 0 1 0-5.997.142 4 4 0 0 0-2.526 5.77 4 4 0 0 0 .556 6.588A4 4 0 1 0 12 18Z" />
				<path d="M12 5a3 3 0 1 1 5.997.142 4 4 0 0 1 2.526 5.77 4 4 0 0 1-.556 6.588A4 4 0 1 1 12 18Z" />
				<path d="M15 13a4.5 4.5 0 0 1-3-4 4.5 4.5 0 0 1-3 4" />
			</svg>
		),
	},
```

(The badge ternary at lines ~101–106 doesn't need a Memories branch — it defaults to 0.)

- [ ] **Step 3: Commit**

```bash
git add ywai/internal/control/web/src/App.tsx ywai/internal/control/web/src/components/layout/Sidebar.tsx
git commit -m "feat(memories): add Memories route + sidebar nav entry"
```

---

## Task 20: Memories page + sub-components + CSS

**Files:**
- Create: `ywai/internal/control/web/src/components/memories/Memories.tsx`
- Create: `ywai/internal/control/web/src/components/memories/Memories.css`
- Create: `ywai/internal/control/web/src/components/memories/MemoryCard.tsx`
- Create: `ywai/internal/control/web/src/components/memories/MemoryDetail.tsx`
- Create: `ywai/internal/control/web/src/components/memories/CaptureMemoryModal.tsx`
- Create: `ywai/internal/control/web/src/components/memories/ConsolidationModal.tsx`
- Create: `ywai/internal/control/web/src/components/memories/ConsolidationPlanReview.tsx`

These follow the existing component conventions (page-header, card, pill, btn, field, tabs, empty-state). Reuse the shared `Modal` and the existing `ModelCombobox` + `SearchSelect`.

- [ ] **Step 1: Memories.tsx (page + 4 internal sub-tabs)**

```tsx
import { useEffect, useState, useCallback } from 'react'
import { useMemoriesStore } from '../../stores/memoriesStore'
import { useMissionsStore } from '../../stores/missionsStore'
import { useWebSocket } from '../../hooks/useWebSocket'
import type { WSMessage } from '../../api/types'
import MemoryCard from './MemoryCard'
import MemoryDetail from './MemoryDetail'
import CaptureMemoryModal from './CaptureMemoryModal'
import ConsolidationModal from './ConsolidationModal'
import './Memories.css'

type SubTab = 'observations' | 'sessions' | 'timeline' | 'context'

const SUB_TABS: { id: SubTab; label: string }[] = [
	{ id: 'observations', label: 'Observaciones' },
	{ id: 'sessions', label: 'Sesiones' },
	{ id: 'timeline', label: 'Timeline' },
	{ id: 'context', label: 'Contexto' },
]

export default function Memories() {
	const [tab, setTab] = useState<SubTab>('observations')
	const [showCapture, setShowCapture] = useState(false)
	const [showConsolidate, setShowConsolidate] = useState(false)
	const [query, setQuery] = useState('')

	const {
		observations,
		selectedObservation,
		sessions,
		timeline,
		context,
		stats,
		engramStatus,
		loading,
		fetchStatus,
		fetchObservations,
		searchMemories,
		fetchStats,
		fetchSessions,
		fetchTimeline,
		fetchContext,
		selectObservation,
	} = useMemoriesStore()

	// Models come from the missions store (already fetched on the Missions page;
	// fetch here too so the consolidation modal works even on a fresh load).
	const models = useMissionsStore((s) => s.models ?? [])
	const agents = useMissionsStore((s) => s.agents ?? [])
	const fetchModels = useMissionsStore((s) => s.fetchModels)
	const fetchAgents = useMissionsStore((s) => s.fetchAgents)

	const handleWSMessage = useCallback((msg: WSMessage) => {
		useMemoriesStore.getState().handleWSMessage(msg)
	}, [])
	useWebSocket('/missions/engram/ws', handleWSMessage)

	useEffect(() => {
		fetchStatus()
		fetchObservations()
		fetchStats()
		fetchModels?.()
		fetchAgents?.()
	}, [fetchStatus, fetchObservations, fetchStats, fetchModels, fetchAgents])

	useEffect(() => {
		if (tab === 'sessions') fetchSessions()
		if (tab === 'timeline') fetchTimeline()
		if (tab === 'context') fetchContext()
	}, [tab, fetchSessions, fetchTimeline, fetchContext])

	const connected = engramStatus?.connected ?? false

	return (
		<div className="memories">
			<header className="page-header">
				<div className="page-heading">
					<span className="page-eyebrow">Memories</span>
					<h1 className="page-title">Memory Management</h1>
					<p className="page-subtitle">
						Explorá, capturá y consolidá las memorias de engram.
					</p>
				</div>
				<div className="page-actions">
					<button className="btn btn-outline" onClick={() => setShowCapture(true)}>
						+ Capturar
					</button>
					<button
						className="btn btn-accent"
						onClick={() => setShowConsolidate(true)}
						disabled={!connected}
						title={!connected ? 'Engram no está disponible' : 'Consolidar memorias'}
					>
						⚡ Consolidar
					</button>
				</div>
			</header>

			{!connected && (
				<div className="alert alert-warning">
					Engram no está disponible. Inicialo con <code>engram serve</code>.
				</div>
			)}

			<div className="kpi-grid">
				<div className="kpi">
					<div className="kpi-top">
						<div className="kpi-value tnum">{stats?.total ?? '—'}</div>
					</div>
					<div className="kpi-label">Total observaciones</div>
				</div>
				<div className="kpi">
					<div className="kpi-top">
						<div className="kpi-value tnum">{sessions.length ?? '—'}</div>
					</div>
					<div className="kpi-label">Sesiones recientes</div>
				</div>
				<div className="kpi">
					<div className="kpi-top">
						<div className="kpi-value tnum">
							{stats?.byType ? Object.keys(stats.byType).length : '—'}
						</div>
					</div>
					<div className="kpi-label">Tipos</div>
				</div>
				<div className="kpi">
					<div className="kpi-top">
						<div className="kpi-value tnum">
							{stats?.avgImportance ? stats.avgImportance.toFixed(1) : '—'}
						</div>
					</div>
					<div className="kpi-label">Importancia prom.</div>
				</div>
			</div>

			<div className="tabs">
				{SUB_TABS.map((t) => (
					<button
						key={t.id}
						className={`tab${tab === t.id ? ' active' : ''}`}
						onClick={() => setTab(t.id)}
					>
						{t.label}
					</button>
				))}
			</div>

			{tab === 'observations' && (
				<section className="memories-section">
					<div className="memories-toolbar">
						<input
							className="input"
							placeholder="🔍 buscar memorias..."
							value={query}
							onChange={(e) => setQuery(e.target.value)}
							onKeyDown={(e) => {
								if (e.key === 'Enter') searchMemories(query)
							}}
						/>
						<button className="btn btn-ghost btn-sm" onClick={() => { setQuery(''); fetchObservations() }}>
							Limpiar
						</button>
					</div>
					<div className="memories-split">
						<div className="memories-list">
							{loading && observations.length === 0 ? (
								<div className="loading-inline">
									<div className="spinner" />
									<span>Cargando memorias…</span>
								</div>
							) : observations.length === 0 ? (
								<div className="empty-state">
									<div className="empty-title">Sin memorias</div>
									<div className="empty-desc">Capturá una nueva o consolidá las existentes.</div>
								</div>
							) : (
								observations.map((o) => (
									<MemoryCard
										key={o.id}
										observation={o}
										active={selectedObservation?.id === o.id}
										onClick={() => selectObservation(o)}
									/>
								))
							)}
						</div>
						<div className="memories-detail">
							{selectedObservation ? <MemoryDetail observation={selectedObservation} /> : (
								<div className="empty-state">
									<div className="empty-title">Seleccioná una memoria</div>
								</div>
							)}
						</div>
					</div>
				</section>
			)}

			{tab === 'sessions' && (
				<section className="memories-section">
					{sessions.length === 0 ? (
						<div className="empty-state"><div className="empty-title">Sin sesiones</div></div>
					) : (
						sessions.map((s) => (
							<div className="card card-pad" key={s.id}>
								<div className="row" style={{ gap: 'var(--space-2)', alignItems: 'baseline' }}>
									<span className="pill pill-primary">{s.id}</span>
									{s.title && <strong>{s.title}</strong>}
								</div>
								{s.summary && <p className="muted">{s.summary}</p>}
							</div>
						))
					)}
				</section>
			)}

			{tab === 'timeline' && (
				<section className="memories-section">
					{timeline.length === 0 ? (
						<div className="empty-state"><div className="empty-title">Timeline vacío</div></div>
					) : (
						timeline.map((e) => (
							<div className="memory-card" key={e.id}>
								<span className="pill pill-muted">{e.type ?? 'event'}</span>
								<span className="muted">{e.content}</span>
							</div>
						))
					)}
				</section>
			)}

			{tab === 'context' && (
				<section className="memories-section">
					{!context ? (
						<div className="loading-inline"><div className="spinner" /><span>Cargando contexto…</span></div>
					) : (
						<div className="card card-pad">
							{context.summary && <p>{context.summary}</p>}
							{context.observations && context.observations.length > 0 && (
								<ul className="memory-context-list">
									{context.observations.map((o) => (
										<li key={o.id}><strong>{o.type}:</strong> {o.content}</li>
									))}
								</ul>
							)}
						</div>
					)}
				</section>
			)}

			<CaptureMemoryModal open={showCapture} onClose={() => setShowCapture(false)} />
			<ConsolidationModal
				open={showConsolidate}
				onClose={() => setShowConsolidate(false)}
				models={models}
				agents={agents}
			/>
		</div>
	)
}
```

> **Note on `useMissionsStore` fields:** The Missions store likely exposes `models`/`agents` via its own fetch actions (it already calls `/missions/api/opencode/models` and `/agents` for the CreateMissionModal). If those fields/methods aren't named `models`/`agents`/`fetchModels`/`fetchAgents`, open `missionsStore.ts` and use the actual names (e.g. `modelsByProvider`, `listModels`, etc.). The consolidation modal consumes whatever shape you wire here.

- [ ] **Step 2: MemoryCard.tsx**

```tsx
import type { EngramObservation } from '../../api/types'

interface Props {
	observation: EngramObservation
	active: boolean
	onClick: () => void
}

export default function MemoryCard({ observation, active, onClick }: Props) {
	const o = observation
	return (
		<div className={`memory-card${active ? ' active' : ''}`} onClick={onClick}>
			<div className="memory-card-head">
				<span className={`pill ${pillClass(o.type)}`}>{o.type ?? 'memory'}</span>
				{o.importance !== undefined && (
					<span className="pill pill-muted">★ {o.importance}</span>
				)}
			</div>
			<div className="memory-card-body">{o.content}</div>
			<div className="memory-card-foot muted">
				{o.sessionID && <span>{o.sessionID}</span>}
				{o.createdAt && <span>{new Date(o.createdAt).toLocaleString()}</span>}
			</div>
		</div>
	)
}

function pillClass(type?: string): string {
	switch (type) {
		case 'save': return 'pill-success'
		case 'summary': return 'pill-accent'
		case 'topic': return 'pill-info'
		case 'observation': return 'pill-primary'
		default: return 'pill-muted'
	}
}
```

- [ ] **Step 3: MemoryDetail.tsx**

```tsx
import { useState } from 'react'
import type { EngramObservation } from '../../api/types'
import { useMemoriesStore } from '../../stores/memoriesStore'

interface Props {
	observation: EngramObservation
}

export default function MemoryDetail({ observation }: Props) {
	const [editing, setEditing] = useState(false)
	const [content, setContent] = useState(observation.content ?? '')
	const updateObservation = useMemoriesStore((s) => s.updateObservation)
	const deleteObservation = useMemoriesStore((s) => s.deleteObservation)

	return (
		<div className="card card-pad memory-detail-panel">
			<div className="row" style={{ justifyContent: 'space-between', alignItems: 'center' }}>
				<span className="muted">{observation.id}</span>
				<div className="row" style={{ gap: 'var(--space-2)' }}>
					<button className="btn btn-ghost btn-sm" onClick={() => setEditing((e) => !e)}>
						{editing ? 'Cancelar' : 'Editar'}
					</button>
					<button
						className="btn btn-danger btn-sm"
						onClick={() => { if (confirm('¿Eliminar esta memoria?')) deleteObservation(observation.id) }}
					>
						Eliminar
					</button>
				</div>
			</div>

			{editing ? (
				<>
					<textarea
						className="textarea"
						value={content}
						onChange={(e) => setContent(e.target.value)}
						rows={8}
					/>
					<button
						className="btn btn-primary btn-sm"
						onClick={() => { updateObservation(observation.id, { content }); setEditing(false) }}
					>
						Guardar
					</button>
				</>
			) : (
				<p className="memory-detail-content">{observation.content}</p>
			)}

			{observation.metadata && Object.keys(observation.metadata).length > 0 && (
				<details>
					<summary className="muted">Metadatos</summary>
					<pre className="mono">{JSON.stringify(observation.metadata, null, 2)}</pre>
				</details>
			)}
		</div>
	)
}
```

- [ ] **Step 4: CaptureMemoryModal.tsx**

```tsx
import { useState } from 'react'
import Modal from '../shared/Modal'
import SearchSelect from '../shared/SearchSelect'
import { useMemoriesStore } from '../../stores/memoriesStore'

interface Props {
	open: boolean
	onClose: () => void
}

const TYPES = ['save', 'observation', 'summary', 'topic']

export default function CaptureMemoryModal({ open, onClose }: Props) {
	const [type, setType] = useState('save')
	const [content, setContent] = useState('')
	const [importance, setImportance] = useState(5)
	const saveMemory = useMemoriesStore((s) => s.saveMemory)

	const submit = async () => {
		if (!content.trim()) return
		await saveMemory({ type, content, importance })
		setContent('')
		setImportance(5)
		onClose()
	}

	return (
		<Modal open={open} onClose={onClose} title="Capturar memoria" width={560}
			footer={
				<>
					<button className="btn btn-ghost" onClick={onClose}>Cancelar</button>
					<button className="btn btn-primary" onClick={submit} disabled={!content.trim()}>
						Guardar
					</button>
				</>
			}
		>
			<div className="field">
				<label className="field-label">Tipo</label>
				<SearchSelect value={type} options={TYPES} onChange={setType} />
			</div>
			<div className="field">
				<label className="field-label">Contenido</label>
				<textarea className="textarea" rows={5} value={content}
					onChange={(e) => setContent(e.target.value)} />
			</div>
			<div className="field">
				<label className="field-label">Importancia: {importance}</label>
				<input type="range" min={1} max={10} value={importance}
					onChange={(e) => setImportance(Number(e.target.value))} style={{ width: '100%' }} />
			</div>
		</Modal>
	)
}
```

- [ ] **Step 5: ConsolidationModal.tsx**

```tsx
import { useEffect, useState } from 'react'
import Modal from '../shared/Modal'
import ModelCombobox from '../missions/ModelCombobox'
import SearchSelect from '../shared/SearchSelect'
import { useMemoriesStore } from '../../stores/memoriesStore'
import ConsolidationPlanReview from './ConsolidationPlanReview'
import type { ModelInfo, AgentInfo } from '../../api/types'

interface Props {
	open: boolean
	onClose: () => void
	models: ModelInfo[]
	agents: string[] | AgentInfo[]
}

export default function ConsolidationModal({ open, onClose, models, agents }: Props) {
	const [model, setModel] = useState('')
	const [agent, setAgent] = useState('memory')
	const consolidation = useMemoriesStore((s) => s.consolidation)
	const startConsolidation = useMemissionsStoreStart()

	useEffect(() => {
		if (open && !model && models.length > 0) setModel(models[0].id)
	}, [open, models, model])

	const agentStrings = agents.map((a) => (typeof a === 'string' ? a : a.id))
	const running = consolidation?.status === 'running'
	const reviewing = consolidation?.status === 'awaiting_review'

	return (
		<Modal open={open} onClose={onClose} title="Consolidar memorias" width={640}>
			{!reviewing ? (
				<>
					<div className="field">
						<label className="field-label">Modelo</label>
						<ModelCombobox id="cons-model" value={model} models={models} onChange={setModel} />
					</div>
					<div className="field">
						<label className="field-label">Agent</label>
						<SearchSelect value={agent} options={agentStrings} onChange={setAgent} />
					</div>

					{running && (
						<div className="alert alert-info">
							<div className="spinner" style={{ marginRight: 8 }} />
							Consolidando… el agent está analizando las memorias.
						</div>
					)}
					{consolidation?.status === 'failed' && (
						<div className="alert alert-danger">Error: {consolidation.error}</div>
					)}

					<div className="row" style={{ justifyContent: 'flex-end', gap: 'var(--space-2)', marginTop: 12 }}>
						<button className="btn btn-ghost" onClick={onClose}>Cerrar</button>
						<button
							className="btn btn-accent"
							disabled={running || !model}
							onClick={() => startConsolidation(model, agent)}
						>
							{running ? 'Consolidando…' : 'Consolidar'}
						</button>
					</div>
				</>
			) : (
				consolidation?.plan && (
					<ConsolidationPlanReview
						plan={consolidation.plan}
						onDone={onClose}
					/>
				)
			)}
		</Modal>
	)
}

// tiny helper hook to avoid importing the store twice
function useMemissionsStoreStart() {
	return useMemoriesStore((s) => s.startConsolidation)
}
```

- [ ] **Step 6: ConsolidationPlanReview.tsx**

```tsx
import { useState } from 'react'
import { useMemoriesStore } from '../../stores/memoriesStore'
import type { ConsolidationPlan } from '../../api/types'

interface Props {
	plan: ConsolidationPlan
	onDone: () => void
}

export default function ConsolidationPlanReview({ plan, onDone }: Props) {
	const apply = useMemoriesStore((s) => s.applyConsolidation)
	const discard = useMemoriesStore((s) => s.discardConsolidation)
	const run = useMemoriesStore((s) => s.consolidation)

	// Default: all checked.
	const [uSel, setUSel] = useState<Record<number, boolean>>(
		Object.fromEntries((plan.updates ?? []).map((_, i) => [i, true])),
	)
	const [dSel, setDSel] = useState<Record<number, boolean>>(
		Object.fromEntries((plan.deletes ?? []).map((_, i) => [i, true])),
	)
	const [sSel, setSSel] = useState<Record<number, boolean>>(
		Object.fromEntries((plan.new_summaries ?? []).map((_, i) => [i, true])),
	)

	const count =
		Object.values(uSel).filter(Boolean).length +
		Object.values(dSel).filter(Boolean).length +
		Object.values(sSel).filter(Boolean).length

	const doApply = async () => {
		if (!run) return
		await apply(run.id, {
			updates: (plan.updates ?? []).filter((_, i) => uSel[i]),
			deletes: (plan.deletes ?? []).filter((_, i) => dSel[i]),
			new_summaries: (plan.new_summaries ?? []).filter((_, i) => sSel[i]),
		})
		onDone()
	}

	const doDiscard = async () => {
		if (!run) return
		await discard(run.id)
		onDone()
	}

	return (
		<div className="consolidation-review">
			{plan.digest && (
				<div className="alert alert-info"><strong>Digest:</strong> {plan.digest}</div>
			)}

			{(plan.updates?.length ?? 0) > 0 && (
				<Section title={`Actualizar (${plan.updates!.length})`}>
					{plan.updates!.map((u, i) => (
						<label key={i} className="review-item">
							<input type="checkbox" checked={!!uSel[i]} onChange={(e) => setUSel({ ...uSel, [i]: e.target.checked })} />
							<div>
								<div className="muted">{u.observation_id} — {u.reason}</div>
								{u.new_content && <div>{u.new_content}</div>}
							</div>
						</label>
					))}
				</Section>
			)}

			{(plan.deletes?.length ?? 0) > 0 && (
				<Section title={`Eliminar (${plan.deletes!.length})`}>
					{plan.deletes!.map((d, i) => (
						<label key={i} className="review-item">
							<input type="checkbox" checked={!!dSel[i]} onChange={(e) => setDSel({ ...dSel, [i]: e.target.checked })} />
							<div><div className="muted">{d.observation_id}</div><div>{d.reason}</div></div>
						</label>
					))}
				</Section>
			)}

			{(plan.new_summaries?.length ?? 0) > 0 && (
				<Section title={`Nuevos resúmenes (${plan.new_summaries!.length})`}>
					{plan.new_summaries!.map((s, i) => (
						<label key={i} className="review-item">
							<input type="checkbox" checked={!!sSel[i]} onChange={(e) => setSSel({ ...sSel, [i]: e.target.checked })} />
							<div><span className="pill pill-accent">{s.type}</span> {s.content}</div>
						</label>
					))}
				</Section>
			)}

			<div className="row" style={{ justifyContent: 'flex-end', gap: 'var(--space-2)', marginTop: 12 }}>
				<button className="btn btn-danger" onClick={doDiscard}>Descartar</button>
				<button className="btn btn-primary" onClick={doApply}>Aplicar {count} cambios</button>
			</div>
		</div>
	)
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
	return (
		<div className="review-section">
			<div className="section-title">{title}</div>
			{children}
		</div>
	)
}
```

- [ ] **Step 7: Memories.css**

```css
.memories {
	display: flex;
	flex-direction: column;
	gap: var(--space-5);
}

.memories-section {
	display: flex;
	flex-direction: column;
	gap: var(--space-3);
}

.memories-toolbar {
	display: flex;
	gap: var(--space-2);
}

.memories-split {
	display: grid;
	grid-template-columns: 1fr 1fr;
	gap: var(--space-4);
}

.memories-list {
	display: flex;
	flex-direction: column;
	gap: var(--space-2);
	max-height: 70vh;
	overflow-y: auto;
}

.memories-detail {
	min-height: 200px;
}

.memory-card {
	padding: var(--space-3);
	border: 1px solid var(--panel-border);
	border-radius: var(--radius-md);
	background: var(--surface);
	cursor: pointer;
	transition: border-color 0.15s, background 0.15s;
	display: flex;
	flex-direction: column;
	gap: var(--space-2);
}

.memory-card:hover {
	border-color: var(--panel-border-strong);
	background: var(--surface-hover);
}

.memory-card.active {
	border-color: var(--info);
}

.memory-card-head {
	display: flex;
	justify-content: space-between;
	align-items: center;
}

.memory-card-body {
	color: var(--text);
}

.memory-card-foot {
	display: flex;
	justify-content: space-between;
	font-size: 0.8rem;
}

.memory-detail-panel {
	display: flex;
	flex-direction: column;
	gap: var(--space-3);
}

.memory-detail-content {
	white-space: pre-wrap;
}

.memory-context-list {
	display: flex;
	flex-direction: column;
	gap: var(--space-1);
	margin-top: var(--space-3);
}

.consolidation-review {
	display: flex;
	flex-direction: column;
	gap: var(--space-3);
}

.review-section {
	display: flex;
	flex-direction: column;
	gap: var(--space-2);
	border-top: 1px solid var(--panel-border);
	padding-top: var(--space-2);
}

.review-item {
	display: flex;
	gap: var(--space-2);
	align-items: flex-start;
	cursor: pointer;
}
```

- [ ] **Step 8: Typecheck + build the UI**

Run: `cd ywai/internal/control/web && npx tsc --noEmit && npm run build`
Expected: build succeeds. If `useMissionsStore` field names differ from `models`/`agents`/`fetchModels`/`fetchAgents`, adjust the imports in `Memories.tsx`/`ConsolidationModal.tsx` to the actual names and re-run until it compiles.

- [ ] **Step 9: Commit**

```bash
git add ywai/internal/control/web/src/components/memories/
git commit -m "feat(memories): add Memories tab UI (observations, sessions, timeline, context, capture, consolidation)"
```

---

## Task 21: Visual smoke test

- [ ] **Step 1: Build + run the control server**

Run: `cd ywai && bash scripts/dev.sh build && ./ywai serve` (in background) — or use the existing dev workflow. Then open `http://localhost:5768/memories`.
Expected: the Memories tab renders with the page header, KPIs, sub-tabs, and (if engram is running) observations. If engram is not running, the warning banner shows.

- [ ] **Step 2: Manual checklist**

- [ ] Sidebar shows "Memories" with the brain icon.
- [ ] Clicking it navigates to `/memories` (no 404 on refresh).
- [ ] Sub-tabs switch between Observaciones / Sesiones / Timeline / Contexto.
- [ ] "+ Capturar" opens the modal; saving a memory (with engram running) adds it to the list.
- [ ] "⚡ Consolidar" opens the modal; model + agent selectors populate.
- [ ] (With opencode + engram running) starting a consolidation shows the running state, then the plan review with checkboxes; "Aplicar N cambios" applies them.

- [ ] **Step 3: Final commit (if any tweaks were made during smoke test)**

```bash
git add -A
git commit -m "fix(memories): smoke-test adjustments"
```

---

## Self-Review (completed during plan writing)

**Spec coverage:**
- ✅ View/manage memories → Tasks 8–11 (handlers) + 20 (UI list/detail/edit/delete).
- ✅ Capture manually → Task 11 (`SaveObservation`) + Task 20 (`CaptureMemoryModal`).
- ✅ Consolidate with opencode (model + agent) → Tasks 7 (agent), 8–9 (manager), 10–12 (handlers), 17 (client), 20 (`ConsolidationModal` + review).
- ✅ Sessions/timeline/context → Tasks 11, 17, 20.
- ✅ Dedicated agent → Task 7.
- ✅ Backend architecture (client in `internal/engram/` + handlers in missions/web) → Tasks 1–13.
- ✅ In-memory state → Task 8 (`ConsolidationManager`).
- ✅ WebSocket feedback → Tasks 8 (`emit`), 12 (`HandleEngramWebSocket`), 18 (`handleWSMessage`).
- ✅ SPA route whitelist → Task 14.

**Placeholder scan:** None — every step has concrete code.

**Type consistency:** `engram.ConsolidationPlan` / `PlanUpdate|PlanDelete|PlanSummary` match across plan.go (Task 4), consolidation.go (Task 8), TS types (Task 16), store (Task 18), and review UI (Task 20). `ApplySelection` shape matches Go (Task 8) and TS (Task 16/18/20). `memoriesApi` method names match store actions.

**Open risks flagged for the executor:**
1. `useMissionsStore` model/agent field names — verified at Task 20 Step 8.
2. `HandleEngramWebSocket` must match the real Hub API — verified at Task 12 Step 2.
3. `registerRoutes` used in tests must not dereference nil at registration — verified at Task 13 Step 1 note.
