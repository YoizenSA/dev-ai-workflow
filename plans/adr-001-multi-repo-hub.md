# ADR-001: Multi-Repo Management Hub

## Status

Proposed

## Context

ywai currently operates inside a single project directory. It manages skills, agents, and workflows for one OpenCode project at a time. The control server (port 5768) is started per-project.

opencode-manager introduces a **multi-repo hub** — a centralized panel that can manage multiple OpenCode repositories simultaneously. This is useful for developers who:

- Maintain several projects (work + personal + OSS)
- Need a single dashboard to see all projects at once
- Want to switch contexts or run commands across repos without leaving the web UI

ywai's architecture is well-positioned for this because it already has:
- A persistent control server daemon
- SQLite storage for kanban, missions, workflows, and settings
- A web UI with routing and Zustand stores
- A profiles store that manages agent profiles

However, ywai's control server is currently started per-project and has no concept of a "repository registry."

## Decision

Build a **Repo Registry** — a SQLite-backed store of registered projects, each with a path, name, detected agent type (opencode, claude-code, etc.), and sync status. The control server becomes a **hub** that manages multiple repos instead of one.

Key design decisions:

1. **Registry-first boot**: The control server always starts from a registry directory (`~/.ywai/hub/`). It scans for registered repos and exposes them via `/api/repos` endpoints.

2. **Lazy reconciliation**: Repo registration stores the path and a cursor (last-sync time). On each view, the hub checks if the repo still exists and if its config has changed. No background sync — the user triggers sync explicitly or the hub does it on access.

3. **Per-repo namespacing**: All per-repo data (kanban, missions, workflows) is stored under the repo's ID namespace. The existing per-project stores are adapted to accept a `repoID` parameter.

4. **Single-port daemon**: One server instance on one port manages all repos. The UI shows a repo selector/dropdown, then scopes all data views to the selected repo. A "Hub" view shows an aggregate dashboard.

5. **New CLI subcommand**: `ywai hub add <path>`, `ywai hub list`, `ywai hub remove <id>` to manage the registry from the terminal.

## Consequences

**Positive:**
- Developer can manage all projects from one dashboard
- Reduces context-switching overhead
- Enables cross-repo features (search, health monitoring, scheduled sessions)
- ywai positions itself as a true management tool, not just a per-project helper

**Negative:**
- Existing single-project workflow still works but may feel more complex
- Data migration: existing per-project SQLite files need to be imported into the hub
- State consistency: the hub needs to handle repos being moved/deleted/unavailable
- First boot setup: user must register repos before they see any data

**Neutral:**
- The hub directory (`~/.ywai/hub/`) becomes the new default. Existing setups need a one-time migration step.
- Per-repo isolation means we can still support the "standalone" mode if needed.

## Alternatives Considered

- **Keep single-project mode, add "switch project" in UI**: Simpler but doesn't scale. No aggregate dashboard, no cross-repo features. Still requires restarting the server to switch.
- **Use symlinks in a projects directory**: Shell-level solution, brittle. No way to track state or sync status.
- **Embed a full git-remote manager**: Over-engineered. ywai doesn't need to clone/push — it just needs to know where repos live and read their state.
- **File-system watcher (fsnotify)**: Auto-discovers repos but adds complexity. Lazy reconciliation is simpler and sufficient.

## References

- opencode-manager repo hub: repo registry, per-repo inspection, aggregate dashboard
- ywai's existing ProfileStore: `internal/control/profiles_store.go` — namespaced file-based profiles
- ywai's kanban store: `internal/kanban/store.go` — SQLite-backed kanban per session
- `~/.ywai/` directory convention in `internal/config/` for storage paths
