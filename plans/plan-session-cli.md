# Implementation Plan: Session CLI

## Overview

Add session management commands to the `ywai` CLI so users can start, stop, list, and inspect sessions from the terminal — without the web UI. Sessions are the same entities as kanban delegations, avoiding data duplication.

## Goals

- `ywai session list` — show active/recent sessions with status, repo, goal, duration
- `ywai session start [--goal GOAL] [--repo ID]` — start a new session, creating a kanban delegation
- `ywai session stop [<id>]` — stop a session (latest or by ID)
- `ywai session status [<id>]` — show detailed session info, elapsed time
- `ywai session log [<id>] [--tail N] [--json]` — view session output/logs
- `ywai session export [<id>]` — export session summary as markdown
- All commands support `--json` for script consumption
- Works with running control server; auto-starts it if needed
- Reuses the kanban Session/Delegation model

## Technical Design

### Architecture

```
Terminal → CLI (cobra) → HTTP client → Control Server Daemon → Kanban Store
                                         (port 5768)           (SQLite)
```

The CLI never talks to the store directly. All session operations go through the control server's REST API. This keeps the CLI thin and the daemon as the single source of truth.

### Data Model

Sessions are kanban `Session` and `Delegation` objects from `internal/kanban/models.go`. No new data model needed. The CLI adds a thin query layer:

```go
// Proposed additions to internal/kanban/models.go (or new file sessions.go)
type SessionSummary struct {
    ID        string `json:"id"`
    RepoID    string `json:"repo_id"`
    Goal      string `json:"goal"`
    Status    string `json:"status"`     // running, completed, cancelled
    StartedAt string `json:"started_at"`
    EndedAt   string `json:"ended_at,omitempty"`
    Duration  string `json:"duration"`   // human-readable
    Agent     string `json:"agent"`
}
```

### CLI Subcommand Structure

```
session
  list      -- List sessions
    --status=all|active|completed|cancelled  (default: all)
    --repo=<id>                               (filter by repo)
    --limit=N                                 (default: 20)
    --json                                    (machine output)
    --since=24h                               (time range)
  
  start     -- Start a new session
    --goal="..."                              (session goal)
    --repo=<id>                               (repo to attach to)
    --agent=<name>                            (agent profile)
    --json                                    (output session ID)
  
  stop      -- Stop a session
    [id]                                      (session ID, defaults to latest active)
    --json
  
  status    -- Show session status
    [id]                                      (session ID, defaults to latest)
    --json
    --watch / -w                              (poll every 2s)
  
  log       -- View session logs
    [id]                                      (session ID, defaults to latest)
    --tail=N                                  (last N lines)
    --follow / -f                             (tail -f mode)
    --json                                    (each line as JSON)
  
  export    -- Export session summary
    [id]                                      (session ID, defaults to latest)
    --out=<file>                              (write to file, default stdout)
    --format=markdown|json                    (default: markdown)
```

### API Endpoints (new on the control server)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/sessions` | List sessions (query: status, repo, limit, since) |
| POST | `/api/sessions` | Create a new session |
| GET | `/api/sessions/{id}` | Get session details |
| PATCH | `/api/sessions/{id}` | Update session (stop, change goal, etc.) |
| GET | `/api/sessions/{id}/log` | Get session log (query: tail, format) |
| POST | `/api/sessions/{id}/export` | Trigger export, returns markdown |

### Log Storage

Session logs are already stored by the run store (`internal/control/workflows_runstore.go`). The `log` command reads these files. For sessions that span multiple delegations, logs are concatenated in chronological order.

## Implementation Phases

### Phase 1: CLI Framework + List/Start/Stop (Sprint 1 — 3 days)

- [ ] Add `session` command group to `cmd/ywai/root.go` (cobra)
- [ ] Implement session API client in `internal/control/client.go` (HTTP to localhost:5768)
- [ ] Add `GET /api/sessions` and `POST /api/sessions` endpoints to control server
- [ ] Implement `ywai session list` (table output, `--json`)
- [ ] Implement `ywai session start` (creates kanban delegation)
- [ ] Implement `ywai session stop` (updates delegation status)
- [ ] Auto-start control server if not running
- [ ] Write tests for CLI parsing + API handlers

### Phase 2: Status, Log, Export (Sprint 2 — 2 days)

- [ ] Add `GET /api/sessions/{id}` and `PATCH /api/sessions/{id}` endpoints
- [ ] Add `GET /api/sessions/{id}/log` with tail/follow support
- [ ] Implement `ywai session status` with `--watch` mode
- [ ] Implement `ywai session log` with `--tail` and `--follow`
- [ ] Add `POST /api/sessions/{id}/export` endpoint
- [ ] Implement `ywai session export` (markdown generator)
- [ ] Add session duration calculation and formatting
- [ ] Write integration tests for the full session lifecycle

### Phase 3: Polish & Multi-Repo (Sprint 3 — 1 day)

- [ ] `--repo` filter across all commands
- [ ] Tab-completion for session IDs (list recent sessions as suggestions)
- [ ] Color-coded output: green for active, yellow for completed, red for cancelled
- [ ] Error messages for common failures (no server, session not found)
- [ ] Documentation: man-page style help, examples in `--help`
- [ ] Update shell completion scripts

## Dependencies

- Control server must be running (auto-started if not)
- Kanban session model must exist (already does: `internal/kanban/models.go`)
- Multi-repo hub (ADR-001) adds `--repo` filtering — Phase 1 works without it

## Risks

- **Risk**: Control server isn't running and auto-start fails (port conflict, permissions).  
  **Mitigation**: Clear error message with instructions to run `ywai kanban` or `ywai serve`.
- **Risk**: Session log files grow unbounded.  
  **Mitigation**: Use existing log rotation in `workflows_runstore.go`; add `--tail` to avoid reading entire files.
- **Risk**: User expects `session start` to launch an interactive terminal.  
  **Mitigation**: Document that `session start` creates a tracked session context; the user then works as usual. The session is a recording/tracking scope, not a pty.

## Success Criteria

- [ ] `ywai session start --goal "fix auth bug"` returns a session ID within 1s
- [ ] `ywai session list` shows the new session as "active" with correct goal
- [ ] `ywai session stop <id>` marks it completed
- [ ] `ywai session log <id>` shows the session output
- [ ] `ywai session status --watch` updates every 2s
- [ ] `ywai session export <id>` produces a valid markdown summary
- [ ] All commands work with `--json` producing valid JSON to stdout
- [ ] Running without a daemon starts one and retries

## Estimated Effort

- Phase 1: 3 days
- Phase 2: 2 days
- Phase 3: 1 day
- Total: 6 days
