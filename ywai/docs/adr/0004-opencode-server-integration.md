# ADR-004: OpenCode Server API Integration for ywai Mission Control

**Status:** Proposed
**Date:** 2026-06-08

---

## Context

ywai Mission Control orchestrates AI-driven development using opencode as the execution engine. The current codebase (`ywai/internal/missions/`) has two distinct opencode integration mechanisms:

### Current integration methods

1. **Config file parsing (discovery)**: `ListModels()` at `web/handlers.go:60` reads `~/.config/opencode/opencode.json` and extracts model names from provider configurations. `ListAgents()` at `web/handlers.go:114` and `kanban/handlers.go:836` read directory listings from `~/.config/opencode/agents/` with optional frontmatter metadata parsing.

2. **Subprocess spawning (execution)**: `GeneratePlanWithOpencode()` at `planning.go:103` spawns `opencode run "generate a plan..."`, captures stdout, and parses the JSON plan response — falling back to local rule-based planning if opencode is unavailable. `SpawnWorker()` at `worker.go:207` spawns `opencode run "Implement feature X..."` with streaming stdout/stderr piped through WebSocket (`web/server.go`, DefaultPort 5769) to broadcast `feature_status`, `mission_status`, and `log_update` events to the Mission Control web UI.

### opencode server API discovery

Running `opencode serve` starts an HTTP server. The OpenAPI specification in `/docopencode.json` reveals these relevant endpoints:

| Endpoint | Method | Description |
|---|---|---|
| `/agent` | GET | List available agents with metadata (name, mode, permissions) |
| `/provider` | GET | List providers grouped as `all`, `default`, `connected` |
| `/api/provider` | GET | Provider listing V2 with detailed metadata |
| `/api/provider/{providerID}` | GET | Specific provider details |
| `/session/{sessionID}/command` | POST | Send a command to an existing session |
| `/session/{sessionID}/message` | GET | Retrieve session messages and responses |
| `/api/question/request` | GET | List pending agent questions (interactive prompts) |
| `/.../question/request/{id}/reply` | POST | Reply to an agent question |
| `/.../question/request/{id}/reject` | POST | Reject an agent question |
| `/tui/submit-prompt` | POST | Submit a prompt via TUI interface |
| `/experimental/tool` | GET | List available tools |
| `/experimental/tool/ids` | GET | List tool IDs |

**Critical gap**: The server API does NOT expose a non-interactive execution endpoint equivalent to `opencode run "task"`. Session-based execution via `POST /session/{sessionID}/command` requires full session lifecycle management (create session → send command → poll messages → close session), which is more complex and stateful than the fire-and-forget subprocess model.

### Key constraints

- The opencode server (`opencode serve`) may or may not be running when ywai interacts with it
- ywai's Go server and the opencode server may be in different network namespaces (Docker containers, WSL, remote dev environments), making `localhost:4096` potentially unreachable
- Subprocess `opencode run` loses the ability to handle interactive questions from opencode during execution — the CLI simply blocks or times out
- The opencode server API is still evolving: multiple endpoints are tagged experimental and there are V1/V2 parallel provider endpoints
- ywai is a Go application with WebSocket streaming to a browser UI — any integration must support real-time progress broadcasting to the web frontend

---

## Decision

Adopt a **hybrid progressive integration strategy** with three phases, governed by a unified `opencode.Client` interface that abstracts the transport mechanism from callers.

### Architecture: The opencode.Client interface

Create `ywai/internal/opencode/client.go` defining:

```go
type Client interface {
    ListModels(ctx context.Context) ([]Model, error)
    ListAgents(ctx context.Context) ([]Agent, error)
    Execute(ctx context.Context, task string, opts ExecuteOptions) (<-chan OutputChunk, error)
    Health(ctx context.Context) error
}
```

Two implementations back this interface:
- **ServerClient** (`ywai/internal/opencode/server_client.go`) — communicates via HTTP to the opencode server API
- **LocalClient** (`ywai/internal/opencode/local_client.go`) — reads config files and spawns subprocesses (extracted from existing handler code)

A **FallbackClient** wraps ServerClient and falls back to LocalClient on connection failure, implementing the cascade: *Try server API → On failure → Fall back to local (config parsing or subprocess)*.

### Phase 1 (Discovery via server API, with config fallback) — Now

Migrate model and agent listing from config file parsing to the opencode server API:

- `ListModels()` → calls `GET /api/provider` on the opencode server, extracts model configurations, derives model keys from `{providerID}/{modelName}`.
- `ListAgents()` → calls `GET /agent` on the opencode server, returns structured agent metadata (name, mode, permissions) instead of raw directory listings.

**Fallback**: If the opencode server is unreachable (connection refused, timeout), FallbackClient falls back to the existing config file parsing logic (`~/.config/opencode/opencode.json` and `~/.config/opencode/agents/` directory). This ensures ywai works even when `opencode serve` is not running.

**Why now?** Discovery via server API is a low-risk change that:
- Eliminates fragile config file parsing (which breaks when opencode changes its config schema)
- Provides richer metadata (`GET /agent` returns mode and permissions, not just filenames)
- Does not affect execution — missions still run via subprocess
- Proves the server connectivity model before investing in session-based execution
- Only affects two handlers (`ListModels`, `ListAgents`) in read-only code paths

### Phase 2 (Session-based execution) — Future

Replace subprocess `opencode run` with session-based execution via the server API:

- Create a session via the server API
- Send the task/full prompt via `POST /session/{sessionID}/command`
- Poll `GET /session/{sessionID}/message` for streaming responses and broadcast them through WebSocket
- Handle interactive questions via `GET /api/question/request` + `POST /.../question/request/{id}/reply`, surfacing them in the Mission Control web UI

This enables capabilities impossible with subprocess:
- **Interactive question handling in the Web UI**: When opencode asks the user a question during execution, ywai surfaces it in Mission Control, waits for user input, and replies programmatically
- **Clean cancellation and timeout control**: Sessions can be terminated without killing OS processes
- **Session history**: Full message history preserved server-side for debugging and replay

**Blocking precondition**: The opencode server API must be confirmed to support session creation and task execution in a documented, stable way. Multiple endpoints in `docopencode.json` are currently tagged experimental.

### Phase 3 (Auto-start and health monitoring) — Future

Add optional opencode server lifecycle management:

- On ywai startup, probe the opencode server health endpoint
- If not running and `opencode.auto_start: true` is configured, spawn `opencode serve` as a child process on a configurable port
- Provide a `/api/opencode/status` endpoint in ywai reporting server connectivity, version, available providers, and agent count
- Graceful shutdown: terminate the managed opencode server when ywai exits

**Decision on auto-start**: ywai MUST NOT auto-start opencode by default. Auto-start is opt-in (`opencode.auto_start: true` in ywai config). Rationale: users may run opencode server independently with custom configuration; spawning a second instance on a different port would be confusing and potentially conflicting. The default behavior is: probe, warn if unreachable via the `/api/opencode/status` endpoint, and fall back to config parsing.

---

## Alternatives Considered

### A. Pure subprocess (status quo)

Keep config file parsing for discovery and subprocess spawning for execution. No server API integration.

| Pros | Cons |
|------|------|
| Already implemented and working | Cannot handle interactive questions |
| No network dependency — works fully offline | Config file parsing breaks on opencode schema changes |
| Simple — one process model, no state management | No access to rich metadata (agent modes, permissions, provider details) |
| No server lifecycle management overhead | No clean cancellation of in-flight execution without SIGKILL |
| Minimal code changes (zero) | Cannot surface agent questions or tool usage in the Web UI |

**Rejected**: This works today but is an architectural dead end. The inability to handle interactive questions means opencode features requiring user confirmation (approvals, clarifications, file overwrites) are either silently skipped or cause timeouts. As opencode evolves toward more interactive and tool-mediated workflows, pure subprocess becomes increasingly limiting.

### B. Pure server API

Drop config parsing and subprocess entirely. All opencode interaction goes through the HTTP server API.

| Pros | Cons |
|------|------|
| Single, clean integration path — one code path | Requires opencode server to ALWAYS be running |
| Full access to all server features and metadata | No session-less fire-and-forget execution endpoint exists |
| Interactive question handling from day one | Server may be unreachable from ywai's network namespace |
| Clean cancellation via session management | Adds ~2s overhead for session creation per task |
| | API is unstable (experimental endpoints may break) |
| | No offline/fallback mode — single point of failure |
| | Forces session lifecycle management on every interaction |

**Rejected**: The server API does not yet have a stable, documented equivalent to `opencode run`. Forcing all execution through sessions when the API is experimental risks breaking changes. Requiring the server to always run adds operational complexity unjustified for simple non-interactive plan generation.

### C. Hybrid progressive (chosen)

Server API for discovery now, subprocess for execution now, progressively migrate execution to sessions as the API stabilizes.

| Pros | Cons |
|------|------|
| Gradual, low-risk migration | Two code paths to maintain during transition |
| Works offline via config fallback | More complex than either pure approach initially |
| Richer metadata for agents/providers today | Must design the opencode.Client abstraction upfront |
| Clear path to interactive question handling | Config fallback logic adds maintenance burden (~200 LOC) |
| No forced server dependency | |
| Can prove server connectivity before committing to session execution | |

**Chosen**: This balances immediate value (richer discovery metadata, elimination of fragile config parsing) with a realistic, incremental migration path toward full server integration once the API stabilizes. The opencode.Client abstraction ensures that callers (handlers, workers) never know which transport is in use.

---

## Consequences

### What becomes easier

- **Agent/provider discovery**: Server API returns structured metadata (mode, permissions, provider details) that config parsing cannot reliably extract. The kanban UI can display agent capabilities without fragile frontmatter parsing in `internal/kanban/handlers.go`.
- **Config schema resilience**: When opencode changes its config format or adds new provider types, ywai does not break — it delegates parsing to the server that owns the schema. Only the LocalClient fallback needs updating.
- **Interactive execution (Phase 2+)**: The session API enables ywai to surface opencode's interactive questions in the Mission Control Web UI and relay user responses — a capability entirely out of reach with subprocess execution.
- **Testing**: The opencode.Client interface enables mocking in unit tests. Currently, tests for planning and worker logic cannot mock opencode interactions.
- **Observability**: The `/api/opencode/status` endpoint provides visibility into server connectivity that currently does not exist.

### What becomes harder

- **Operational complexity**: The abstraction layer (opencode.Client + ServerClient + LocalClient + FallbackClient) adds approximately 300–500 lines of Go code to implement and maintain.
- **Network debugging**: Issues like "opencode server unreachable" vs "API returned an error" are harder to diagnose than subprocess failures where stderr is directly available. Requires structured error types and logging.
- **Connection management**: The ywai server must maintain an HTTP client with connection pooling, configurable timeouts, and retry logic for the opencode server API.
- **Ordering dependency (Phase 2+)**: Session-based execution means ywai must manage session lifecycle (create, poll, close), which is inherently more stateful than spawning a process and waiting for it to exit. This state must survive ywai server restarts.

### Neutral

- **Performance**: Discovery via HTTP adds approximately 10–50ms per call compared to local file reads. This is negligible for the Web UI use case (models/agents are listed once on page load, not in hot paths). Execution performance is unchanged in Phase 1 (still subprocess).
- **Offline capability**: Preserved via the config fallback. Users without the server running still get basic model/agent listing from config files.
- **Binary size**: The new `ywai/internal/opencode/` package adds a small dependency (net/http is already imported in the codebase).

---

## Migration Path

### Step 1: Define the abstraction
Create `ywai/internal/opencode/client.go` with the `Client` interface, `Model`, `Agent`, `ExecuteOptions`, and `OutputChunk` types.

### Step 2: Implement ServerClient
Create `ywai/internal/opencode/server_client.go` that calls the opencode server HTTP API for `ListModels` and `ListAgents`. `Execute` returns `ErrNotImplemented` until Phase 2. Configure the base URL from ywai config (default: `http://localhost:4096`).

### Step 3: Extract LocalClient from existing code
Move config file parsing logic from `web/handlers.go:60-111` (ListModels) and `web/handlers.go:114-137` / `kanban/handlers.go:836-875` (ListAgents) into `ywai/internal/opencode/local_client.go`. Move subprocess execution logic from `planning.go:103` and `worker.go:207` into the same package. Export `DetectOpencode` from the worker package.

### Step 4: Implement fallback
Create `ywai/internal/opencode/fallback_client.go` that tries ServerClient first and falls back to LocalClient on connection failure. Log a warning when falling back so operators know the server is unreachable.

### Step 5: Update handlers
Replace direct config reads in `web/handlers.go` and `kanban/handlers.go` to use `opencode.Client` injected via dependency injection. The handlers should not know which transport is in use.

### Step 6: Add /api/opencode/status endpoint
Expose server health, provider count, agent count, and current transport mode (server/local/fallback) via a new endpoint in the web server.

### Step 7 (Phase 2): Implement session-based execution
When the opencode server API stabilizes, implement ServerClient.Execute() using session creation, command submission, and message polling. Integrate question handling via the question API endpoints with WebSocket broadcasting.

### Step 8 (Phase 3): Auto-start support
Add configuration option `opencode.auto_start` (default: false). When enabled, probe server on startup; if unreachable, spawn `opencode serve` as managed child process. Add graceful shutdown.

---

## Relevant Files

| File | Role |
|------|------|
| `ywai/internal/missions/web/handlers.go:60` | ListModels — current config parser, to be refactored |
| `ywai/internal/missions/web/handlers.go:114` | ListAgents — current directory reader, to be refactored |
| `ywai/internal/kanban/handlers.go:836` | ListAgents — kanban variant with frontmatter parsing |
| `ywai/internal/missions/planning.go:103` | GeneratePlanWithOpencode — subprocess spawner |
| `ywai/internal/missions/worker.go:98` | DetectOpencode — binary detection |
| `ywai/internal/missions/worker.go:207` | SpawnWorker — subprocess spawner with streaming |
| `ywai/internal/missions/web/server.go` | WebSocket server (port 5769), streams to web UI |
| `ywai/internal/missions/web/hub.go` | WebSocket hub, broadcasts to connected clients |
| `/docopencode.json` | OpenCode server OpenAPI specification |
| `~/.config/opencode/opencode.json` | OpenCode config (current target, becomes fallback) |
| `~/.config/opencode/agents/` | OpenCode agents directory (current target, becomes fallback) |
| `ywai/internal/opencode/` | New package to contain the abstraction (to be created) |
