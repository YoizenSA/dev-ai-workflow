# ADR-002: Session CLI for Terminal Management

## Status

Proposed

## Context

ywai's CLI (`ywai`) currently handles installation, updates, skills, agents, and doctor checks. It does **not** provide any session management — starting, stopping, listing, or inspecting OpenCode sessions from the terminal.

opencode-manager ships `ocm-cli`, a dedicated CLI that lets users manage sessions without the web UI. This is critical for:
- Headless/CI environments where no browser is available
- Power users who prefer the terminal
- Automation scripts that trigger sessions programmatically

ywai already has:
- Cobra-based CLI (`cmd/ywai/root.go`, `cmd/ywai/commands.go`)
- A sessions model in the kanban system (`internal/kanban/`)
- Background daemon (control server) for persistent operations
- `internal/control/` API that sessions run through

What ywai lacks is a **session as a first-class concept** in the CLI — right now sessions are implicit (you run `ywai kanban` which starts the UI).

## Decision

Add new session subcommands to the ywai CLI under a `ywai session` command group:

```
ywai session list          # List active and recent sessions
ywai session start         # Start a new session (optionally with a goal)
ywai session stop          # Stop the current session
ywai session status        # Show current session details
ywai session log           # View session output/logs
ywai session export        # Export session summary to markdown
```

Key design decisions:

1. **Session stored in SQLite**: Sessions are persisted in the hub database (see ADR-001). Each session has an ID, repo ID (for multi-repo), start/end time, goal, status, and output log reference.

2. **CLI queries control server daemon**: The session subcommands communicate with the running control server via HTTP (`localhost:5768`). If no server is running, `ywai session start` starts it automatically (matching `ywai kanban` behavior).

3. **Session lifecycle mirrors kanban sessions**: A CLI session is the same entity as a kanban delegation — `ywai session start` creates a delegation card, `ywai session stop` closes it. This avoids data duplication.

4. **JSON output flag**: `--json` / `-j` flag on all session commands for script consumption (`ywai session list -j | jq`).

## Consequences

**Positive:**
- Terminal-native session management for headless/CI workflows
- Session data becomes queryable and scriptable
- Reuses existing kanban session model — no new data schema
- Works with or without the web UI running
- Enables session scheduling (see ADR-003) via cron-like CLI automation

**Negative:**
- Adds new CLI surface area to maintain
- Control server must be running for most operations (or auto-started)
- JSON output format needs to be stable for script consumers
- Session logs can grow large — need rotation or truncation

**Neutral:**
- The `session` command group follows Cobra patterns already in use
- We can reuse the existing WebSocket hub for live session output streaming
- Sessions are always associated with a repo (even if "default" or "current directory")

## Alternatives Considered

- **Independent CLI binary (`ywai-session`)**: Too much overhead for a small extension. Keep it under the ywai binary.
- **REST-only management (no CLI)**: Loses headless/CI use case. The CLI is critical for automation.
- **gRPC for CLI↔daemon comms**: Over-engineered for localhost. HTTP is simpler and already exists.
- **YAML config files for sessions**: Harder to query and automate than SQLite + JSON output.

## References

- opencode-manager `ocm-cli`: `ocm session list | start | stop | status`
- ywai's CLI entry: `cmd/ywai/root.go` — Cobra command patterns
- ywai's kanban sessions: `internal/kanban/models.go` — Session model
- ywai's control server: `internal/control/server.go` — Daemon process
