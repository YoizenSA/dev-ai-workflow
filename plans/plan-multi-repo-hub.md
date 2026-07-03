# Implementation Plan: Multi-Repo Management Hub

## Overview

Add a repo registry to ywai so that the control server and web UI can manage multiple OpenCode projects from one dashboard. Users register project paths, the hub detects agent type and config state, and all features scope their data per-repo.

## Goals

- Register, list, and remove projects from a central hub
- Detect agent type (opencode, claude-code, etc.) and config state per repo
- Scope all existing features (kanban, missions, workflows) by repo ID
- Aggregate dashboard showing all repos at a glance
- CLI subcommands for hub management (`ywai hub add|list|remove|sync`)
- Backward compatibility: existing single-project workflow still works

## Technical Design

### Architecture

```
~/.ywai/hub/
├── hub.db              # SQLite registry database
├── repos/
│   ├── <repo-id>/
│   │   ├── kanban.db   # Per-repo kanban data
│   │   ├── missions.db # Per-repo missions
│   │   └── workflows/  # Per-repo workflow runs
│   └── ...
└── settings.json       # Hub-level settings
```

The hub is a new package: `internal/hub/` with sub-packages:
- `internal/hub/registry.go` — RepoRegistry (CRUD, scan, detect agent)
- `internal/hub/store.go` — SQLite-backed storage for registry entries
- `internal/hub/detector.go` — Agent/config detection logic

### Data Model

```sql
CREATE TABLE repos (
    id          TEXT PRIMARY KEY,           -- uuid or slug
    name        TEXT NOT NULL,              -- display name (dir name)
    path        TEXT NOT NULL UNIQUE,       -- absolute filesystem path
    agent_type  TEXT NOT NULL DEFAULT '',   -- opencode, claude-code, etc.
    config_hash TEXT,                       -- sha256 of opencode.json for change detection
    added_at    TEXT NOT NULL,              -- ISO 8601
    synced_at   TEXT,                       -- last sync timestamp
    status      TEXT NOT NULL DEFAULT 'active'  -- active, missing, error
);

CREATE TABLE repo_tags (
    repo_id TEXT NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    tag     TEXT NOT NULL,
    PRIMARY KEY (repo_id, tag)
);
```

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/repos` | List all registered repos with status |
| POST | `/api/repos` | Register a new repo (body: `{path, name?}`) |
| GET | `/api/repos/{id}` | Get repo details |
| DELETE | `/api/repos/{id}` | Remove repo from registry |
| POST | `/api/repos/{id}/sync` | Trigger config re-scan |
| GET | `/api/repos/hub/status` | Aggregate hub status (count by status) |
| PUT | `/api/repos/{id}/settings` | Per-repo settings override |

### UI Components

- **RepoSelector** — dropdown in the top bar to switch active repo
- **HubDashboard** — new page (`/hub`) with cards for each repo showing: name, path, agent type, status, last sync, config validity
- **RepoSetupWizard** — modal for adding a new repo (path input + optional name)
- **RepoDetailPage** — existing pages (kanban, missions, etc.) scoped to a repo via URL param `?repo=<id>`

### CLI Commands

```
ywai hub add <path> [--name NAME]   # Register a repo
ywai hub list [--json]              # List registered repos
ywai hub remove <id>                # Remove a repo
ywai hub sync [<id>]                # Sync config for one or all repos
ywai hub status [<id>]              # Show health/status
```

## Implementation Phases

### Phase 1: Registry Core (Sprint 1 — 3 days)

- [ ] Create `internal/hub/` package with `Repo`, `RepoRegistry` types
- [ ] Implement SQLite store: create table, CRUD operations
- [ ] Implement agent detection (`detectAgent(path)`) scanning for binaries and config files
- [ ] Implement config change detection (hash comparison)
- [ ] Add hub scaffolding: create `~/.ywai/hub/` on first boot
- [ ] Add `GET /api/repos` and `POST /api/repos` endpoints
- [ ] Add `ywai hub add`, `ywai hub list` CLI subcommands
- [ ] Write unit tests for registry and store

### Phase 2: UI Integration (Sprint 2 — 3 days)

- [ ] Build `RepoSelector` component (dropdown in sidebar top bar)
- [ ] Wire Zustand store for current repo state
- [ ] Build `HubDashboard` page with repo cards
- [ ] Build `RepoSetupWizard` modal
- [ ] Add `DELETE /api/repos/{id}` + `POST /api/repos/{id}/sync` endpoints
- [ ] Add `ywai hub remove`, `ywai hub sync` CLI subcommands
- [ ] Scope existing pages by repo ID (kanban, missions, workflows)
- [ ] Write integration tests for API endpoints

### Phase 3: Data Migration & Polish (Sprint 3 — 2 days)

- [ ] One-time migration: import legacy single-project data into hub
- [ ] Per-repo kanban/missions database isolation (or shared with `repo_id` column)
- [ ] Error handling for missing/deleted repos (graceful degradation)
- [ ] UI polish: empty state, loading skeletons, error toasts
- [ ] Documentation: README update, CLI help text

## Dependencies

- `internal/config/` must expose the hub directory path
- The control server must start from hub context (not current directory)
- Existing dashboard pages must accept a `repo_id` query parameter

## Risks

- **Risk**: Existing users have per-project data outside `~/.ywai/hub/`.  
  **Mitigation**: Phase 3 adds an import command; existing workflows continue working in "legacy" mode.
- **Risk**: Paths change (repo moved after registration).  
  **Mitigation**: Sync checks path existence; marks as "missing" if not found, suggests update.
- **Risk**: Large number of repos slows down aggregate queries.  
  **Mitigation**: Lazy loading — only load active repo's data; aggregate dashboard does count-only queries.

## Success Criteria

- [ ] User can add 3+ repos and see them all on a dashboard
- [ ] Switching repo in the UI scopes all data correctly (kanban, missions, workflows)
- [ ] Adding a deleted repo path shows "missing" status
- [ ] CLI commands return correct machine-parseable JSON with `--json`
- [ ] All existing tests pass (backward compatible)
- [ ] Legacy single-project mode still works (adds as implicit "default" repo)

## Estimated Effort

- Phase 1: 3 days
- Phase 2: 3 days
- Phase 3: 2 days
- Total: 8 days
