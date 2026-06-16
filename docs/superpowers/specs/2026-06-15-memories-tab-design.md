# Memories Tab — Design Spec

**Date:** 2026-06-15
**Status:** Approved (pending user spec review)
**Objective:** Add a new "Memories" tab to the control UI to view, capture, and consolidate engram memories, connected to opencode for consolidation runs with model + agent selection.

---

## 1. Overview

Add a **Memories** tab to the ywai control UI that lets users:

1. **View & manage** existing engram memories (browse, search, filter, edit, delete).
2. **Capture** new memories manually (type, content, importance, metadata).
3. **Consolidate** memories using opencode — select model + agent, run a consolidation, review the proposed plan, and selectively apply changes.
4. **Explore** sessions, timeline, and the current memory context (what the system "knows").

The tab connects to two external services through the Go backend:
- **engram** (separate binary, port **7437**) — the memory store with REST API.
- **opencode** (port **4096**) — used to run consolidation via its `SessionAPI`.

A **dedicated memory agent** (`ywai/agents/memory/`) specializes in proposing consolidation plans. The agent only proposes; the backend applies changes after user review.

---

## 2. Architecture

```
Browser (control UI :5768)
  └─ Memories tab  →  HTTP /api/engram/* + /engram/ws (same-origin)
                          │
        Go control server (internal/control/server.go :5768)
          └─ missions/web (existing mux, extended)
                ├─ engram handlers (/api/engram/*)
                ├─ consolidation handlers (/api/engram/consolidations/*)
                ├─ /engram/ws (WS broadcast hub)
                │
                ├─ internal/engram/ (NEW client → engram :7437)
                └─ opencode.SessionAPI (→ opencode :4096)

engram :7437  (SQLite + FTS5, REST API)
opencode :4096  (LLM via providers)
```

### New pieces

1. **`internal/engram/`** — HTTP client package to engram (mirrors `internal/opencode/`): `client.go` (interface + impl), `factory.go` (`DefaultClient()` + `ProbeServer()`), `models.go` (DTOs).
2. **Handlers in `missions/web`** — extend `registerRoutes()` and the `Handlers` struct with an `engramClient` field + a consolidation manager. Routes under `/api/engram/*`.
3. **Consolidation manager** — small in-memory coordinator (goroutine per run) that drives opencode session lifecycle and broadcasts progress via the existing WS hub.
4. **`ywai/agents/memory/`** — dedicated agent: `AGENT.md` (consolidation prompt), `tools.json` (engram **read** tools only — the agent must not write; the backend applies changes after review), `skills.txt`.

### Bonus fix

Wire the `opencodeClient` into the missions `Engine` (`NewEngine` → `e.workers.SetClient(client)`) so the `SessionAPI` path works in production. Today the client is nil, so `canUseAPI()` returns false and everything goes through the CLI. This fix benefits missions as well.

---

## 3. engram HTTP API contract

The engram binary exposes a REST API on port **7437** (overridable via `ENGRAM_URL`). Endpoints consumed by ywai (from engram's `internal/server/server.go`):

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/health` | Liveness / status probe |
| GET | `/observations/recent?limit=N` | Most recent observations |
| GET | `/observations/{id}` | Single observation |
| PATCH | `/observations/{id}` | Update content/importance/metadata |
| DELETE | `/observations/{id}` | Delete observation |
| POST | `/save` | Save a new observation (capture) |
| GET | `/search?q=...&limit=N` | Full-text search (FTS5) |
| GET | `/stats` | Counts by type, totals |
| GET | `/sessions/recent?limit=N` | Recent engram sessions |
| GET | `/timeline?limit=N` | Chronological memory events |
| GET | `/context?...` | Current memory context (what the system knows) |

### Client interface (`internal/engram/client.go`)

```go
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

`factory.go`: `DefaultClient(ctx)` probes `http://127.0.0.1:7437/health` (3s timeout); if reachable returns an HTTP client, else returns a client whose every call returns `ErrEngramUnavailable`.

---

## 4. Backend endpoints (exposed to frontend)

All routes are added to the **missions/web mux**. The control server mounts missions/web under `/missions` (`internal/control/server.go:buildRoutes`), so:

- **Handlers register:** `/api/engram/*` (and `/engram/ws`).
- **Browser calls:** `/missions/api/engram/*` (and `/missions/engram/ws`) — the `/missions` prefix is stripped before forwarding.
- **Vite dev proxy:** the `/missions` rule already exists in `vite.config.ts`, so no proxy change is needed. The `memoriesApi` client object uses the full `/missions/api/engram/...` path (mirroring how `missionsApi` uses `/missions/api/...`).

> The existing `missionsApi` object in `client.ts` already follows this convention (it calls `/missions/api/missions`, `/missions/api/opencode/models`, etc.), so `memoriesApi` is consistent with it.

| Method | Route | Handler action |
|--------|-------|----------------|
| GET | `/api/engram/status` | engram connection status |
| GET | `/api/engram/observations?limit=` | recent observations |
| GET | `/api/engram/observations/{id}` | one observation |
| PATCH | `/api/engram/observations/{id}` | edit observation |
| DELETE | `/api/engram/observations/{id}` | delete observation |
| POST | `/api/engram/save` | capture new memory |
| GET | `/api/engram/search?q=&limit=` | FTS5 search |
| GET | `/api/engram/stats` | stats (counts by type) |
| GET | `/api/engram/sessions?limit=` | recent sessions |
| GET | `/api/engram/timeline?limit=` | timeline events |
| GET | `/api/engram/context` | current memory context |
| POST | `/api/engram/consolidations` | start consolidation (async); returns `run_id` (HTTP 202) |
| GET | `/api/engram/consolidations/{id}` | run status + plan (if awaiting review) |
| POST | `/api/engram/consolidations/{id}/apply` | apply selected plan items |
| POST | `/api/engram/consolidations/{id}/discard` | discard the proposal |
| GET | `/engram/ws` | WebSocket: `consolidation.started`, `.progress`, `.completed`, `.applied`, `.failed` |

### Error handling

- Reuse `writeJSON` / `writeError` from `missions/web/server.go`.
- If engram is down, `GET /api/engram/status` returns `{connected: false}`; all other engram endpoints return `503` with `{error: "engram unavailable"}`.
- If opencode is down, `POST /consolidations` returns `503` with `{error: "opencode server not running"}`.

---

## 5. Consolidation flow (with review)

### Lifecycle

```
running → awaiting_review → applying → applied
   │                              │
   └→ failed               discarded
```

### Start

`POST /api/engram/consolidations {model, agent}`:
1. Generate `run_id`; store `ConsolidationRun{status:"running"}` in memory.
2. Return `202 {run_id}` immediately.
3. Spawn a goroutine (the consolidation manager).

### Goroutine steps

1. `engram.GetContext()` — fetch everything the system knows (observations, recent sessions).
2. `opencode.Sessions().Create(agent="memory", model=<chosen>, title="Consolidation run_id")`.
3. Build a prompt embedding the context, instructing the agent to produce a structured JSON plan.
4. `Sessions().Prompt()` → `Sessions().Wait()` (broadcast progress via WS).
5. `Sessions().Messages()` → take the last assistant message → parse the JSON plan.
6. Store `ConsolidationRun.Plan`; set status `awaiting_review`.
7. Broadcast `consolidation.completed {run_id}`.

### Review (frontend)

The UI shows the plan with per-item checkboxes:
- updates[] → each with `observation_id`, `reason`, optional `new_content`, optional `new_importance`.
- deletes[] → each with `observation_id`, `reason`.
- new_summaries[] → each with `type`, `content`, `importance`, optional `metadata`.
- digest (optional) → executive summary string.

### Apply

`POST /api/engram/consolidations/{id}/apply {accepted_updates[], accepted_deletes[], accepted_summaries[]}`:
1. Set status `applying`.
2. For each accepted update → `engram.UpdateObservation(...)`.
3. For each accepted delete → `engram.DeleteObservation(id)`.
4. For each accepted summary → `engram.Save(...)`.
5. Broadcast progress; set status `applied` (or `failed` on error).

### Discard

`POST /api/engram/consolidations/{id}/discard` → set status `discarded`, no engram writes.

### Consolidation plan JSON schema

```typescript
interface ConsolidationPlan {
  updates: Array<{
    observation_id: string;
    reason: string;
    new_content?: string;
    new_importance?: number;
  }>;
  deletes: Array<{
    observation_id: string;
    reason: string;
  }>;
  new_summaries: Array<{
    type: string;       // e.g. "summary", "topic"
    content: string;
    importance: number;
    metadata?: Record<string, unknown>;
  }>;
  digest?: string;
}
```

### In-memory state

```go
type ConsolidationRun struct {
    ID        string
    Model     string
    Agent     string
    Status    string  // running | awaiting_review | applying | applied | discarded | failed
    Plan      *ConsolidationPlan
    SessionID string
    Error     string
    StartedAt time.Time
    UpdatedAt time.Time
}
```

A `map[string]*ConsolidationRun` guarded by `sync.RWMutex` lives in the consolidation manager (held by `Handlers`). On server restart, in-flight runs are lost — accepted because engram is not modified until `apply`, so no data corruption.

---

## 6. Dedicated memory agent (`ywai/agents/memory/`)

### `AGENT.md`

System prompt for a memory-consolidation specialist. Behavior:
- Reads the provided memory context.
- Identifies: (a) duplicate observations, (b) obsolete/contradicted entries, (c) themes worth summarizing.
- Produces a single `ConsolidationPlan` JSON object (schema above).
- **Must not invent content** — only reorganize, summarize, or flag existing observations.
- Includes the `digest` field with a high-level summary of the current memory state.

### `tools.json`

Engram **read** tools only (so the agent can explore if needed, but cannot write):
- `engram_mem_context`
- `engram_mem_search`
- `engram_mem_get_observation`

Write tools (`engram_mem_save`, `engram_mem_update`, `engram_mem_session_*`) are **excluded** — the backend applies changes after user review. This guarantees the user stays in control.

### `skills.txt`

Left empty (or lists relevant skills if any are added later).

---

## 7. Frontend design

### Files (follow Missions pattern)

```
src/components/memories/
├── Memories.tsx                  # page (default export) + 4 internal sub-tabs
├── Memories.css                  # page-scoped styles (tokens only)
├── MemoryCard.tsx                # observation card
├── MemoryDetail.tsx              # split-view detail panel
├── CaptureMemoryModal.tsx        # manual capture form
├── ConsolidationModal.tsx        # start consolidation + live progress
└── ConsolidationPlanReview.tsx   # selective plan review/apply
```

Plus:
- `src/stores/memoriesStore.ts` — Zustand store (mirrors `missionsStore.ts`).
- `src/api/client.ts` — add `memoriesApi` exported object.
- `src/api/types.ts` — add `// ─── Memories Types ───` section.
- `src/App.tsx` — add `<Route path="/memories" element={<Memories />} />`.
- `src/components/layout/Sidebar.tsx` — add `NAV_ITEMS` entry (path, label, brain/database SVG icon).

### Page layout

```
┌─────────────────────────────────────────────────────────────┐
│ page-header                                  [Consolidar]   │
│   eyebrow: Memories                                          │
│   title: Memory Management                                   │
│   subtitle: Explorá, capturá y consolidá las memorias...    │
├─────────────────────────────────────────────────────────────┤
│ kpi-grid: [Total obs] [Sesiones] [Tipos] [Importancia avg]  │
├─────────────────────────────────────────────────────────────┤
│ sub-tabs: [Observaciones] [Sesiones] [Timeline] [Contexto]  │
├─────────────────────────────────────────────────────────────┤
│ search bar + filters      [+ Capturar]                       │
├──────────────────────────────────┬──────────────────────────┤
│  Observation cards (list)        │  Detail panel            │
│                                  │  (selected observation)  │
└──────────────────────────────────┴──────────────────────────┘
```

### Internal sub-tabs (Settings.tsx pattern)

- **Observaciones** — list + detail + search/filters + edit/delete + capture.
- **Sesiones** — recent sessions list with summaries.
- **Timeline** — chronological memory events.
- **Contexto** — readable view of `/context` (what the system knows now).

### Capture modal

Uses shared `Modal` + `SearchSelect`:
- Tipo (save / observation / summary / topic — SearchSelect with `allowCustom`).
- Contenido (textarea).
- Importancia (1–10, number input or slider).
- Metadatos (optional, JSON key-value).

### Consolidation modal

**Step 1 — Configure:** `ModelCombobox` (models from `/api/opencode/models`) + `SearchSelect` for agent (defaulting to `memory`, from `/api/opencode/agents`).

**Step 2 — Live progress:** checklist driven by WebSocket events (`consolidation.started`, `.progress`, `.completed`).

**Step 3 — Review (`ConsolidationPlanReview`):**
- Digest (if present) shown at the top.
- Collapsible sections: updates, deletes, new_summaries — each item has a checkbox (pre-checked by default) + reason + diff preview.
- Footer: `[Descartar]` + `[Aplicar N cambios]` (N = sum of checked items).

### Store (`memoriesStore.ts`)

State: `observations`, `sessions`, `timeline`, `context`, `stats`, `selectedObservation`, `consolidation` (active run), `loading`, `error`, `engramConnected`.

Actions: `fetchObservations`, `searchMemories`, `saveMemory`, `updateObservation`, `deleteObservation`, `fetchStats`, `fetchSessions`, `fetchTimeline`, `fetchContext`, `startConsolidation`, `applyConsolidation`, `discardConsolidation`, `handleWSMessage`.

`handleWSMessage` reacts to `consolidation.*` events to update `consolidation` in real time.

### WebSocket

The page connects via `useWebSocket("/missions/engram/ws", handleWSMessage)` (path adjusted for the `/missions` control-server prefix). Events update the store live.

### Empty states

- **Engram not running** (`/api/engram/status` → `{connected:false}`): an `alert-warning` banner "Engram no está disponible. Inicialo con `engram serve`." Sub-tabs show `empty-state`.
- **No memories**: `empty-state` with CTAs "Capturar" / "Consolidar".

### Styling

CSS tokens only (`var(--*)`). Reuse prebuilt classes: `.page-header`, `.kpi-grid`, `.card`, `.pill` (per type), `.btn*`, `.field`, `.input`, `.tabs`, `.empty-state`, `.alert*`. Per-page CSS holds only layout specifics (`.memory-card`, `.memory-detail`, `.consolidation-review`).

---

## 8. Testing strategy

### Go

- **`internal/engram/`**: unit tests with an `httptest.Server` stubbing engram responses (same approach as `server_client_test.go`). Cover: `ProbeServer`, each client method, error decoding, unavailable case.
- **Consolidation manager**: unit test with a fake `SessionAPI` (interface) that returns canned messages; verify the plan is parsed and the run transitions through statuses.
- **Handlers**: `net/http/httptest` tests for the `/api/engram/*` routes, injecting a fake engram client (interface) — mirrors `missions/web/web_test.go`. Cover: status, list, search, save, update, delete, consolidation start/get/apply/discard.
- Run `bash scripts/dev.sh test` before every commit (per `AGENTS.md`).

### Frontend

- Type-safety via `tsc` (the build step). No JS test framework is configured in the UI today; the spec does **not** introduce one (YAGNI) — the Go backend is where logic correctness is asserted. The UI is verified by the `kanban` dev script equivalent (manual visual check via `dev.sh`).

---

## 9. Out of scope (v1)

- Persisting consolidation runs across server restarts (in-memory is enough for v1).
- Scheduling automatic/periodic consolidations.
- Drag-and-drop reorganization of memories.
- A digest/history of past consolidations (only the active run is tracked).
- Engram **write** tools for the agent (by design — the backend applies changes).
- Permissions/auth on the engram API (engram runs locally; assumed trusted).

---

## 10. Open questions (resolved during design)

| Question | Decision |
|----------|----------|
| Consolidation mode | With review (agent proposes, user approves) |
| Dedicated agent | Yes, `ywai/agents/memory/` |
| Backend architecture | Approach A: engram client + handlers in missions/web |
| Execution | Via opencode `SessionAPI` (async + WS) |
| State persistence | In-memory (process lifetime) |
| Visual scope | Full, polished (like Missions) |
