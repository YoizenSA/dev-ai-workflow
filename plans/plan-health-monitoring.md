# Implementation Plan: Health Monitoring for Repos and Agents

## Overview

Add a health monitoring system to the ywai control server that periodically checks installed agents, registered repos, database integrity, and system resources. Results are exposed via API and displayed in a web UI dashboard with a status badge.

## Goals

- Periodic health checks for repo existence, agent binary, config validity, disk space
- Web UI health dashboard showing all checks with pass/warn/fail status
- Health badge in sidebar showing aggregate status (green/yellow/red)
- On-demand check triggering ("Run now")
- Historical check results with pass/fail timeline
- Auto-purge old history (30-day retention)
- CLI command: `ywai health [check]` for terminal access

## Technical Design

### Architecture

```
Ticker (configurable interval) ──► HealthMonitor
        │                              │
        │                    ┌─────────┼──────────┐
        │                    ▼         ▼          ▼
        │              repo_exists  agent_binary  disk_space
        │                    │         │          │
        ▼                    ▼         ▼          ▼
   Health Store            SQLite (results + config)
   (batch insert)
```

The monitor is a package `internal/health/` with individual check functions, a runner that orchestrates them, and a SQLite store for results.

### Data Model

```sql
CREATE TABLE health_checks (
    id          INTEGER PRIMARY KEY,
    repo_id     TEXT,                          -- NULL for global checks (disk, server)
    check_name  TEXT NOT NULL,                 -- repo_exists, agent_binary, etc.
    status      TEXT NOT NULL,                 -- pass, warn, fail, error
    message     TEXT,                          -- human-readable detail
    duration_ms INTEGER NOT NULL,
    details     TEXT,                          -- optional JSON blob with extra data
    run_at      TEXT NOT NULL                  -- ISO 8601
);

CREATE INDEX idx_health_repo_check ON health_checks(repo_id, check_name);
CREATE INDEX idx_health_run_at ON health_checks(run_at);

CREATE TABLE health_check_config (
    check_name      TEXT PRIMARY KEY,
    enabled         INTEGER NOT NULL DEFAULT 1,
    interval_secs   INTEGER NOT NULL DEFAULT 3600,
    severity        TEXT NOT NULL DEFAULT 'low', -- critical, high, medium, low
    timeout_secs    INTEGER NOT NULL DEFAULT 10
);

-- Seed default config
INSERT INTO health_check_config VALUES
    ('repo_exists',   1, 0,    'critical', 5),    -- on-demand only (0 = manual)
    ('repo_config',   1, 0,    'high',     5),
    ('agent_binary',  1, 3600, 'critical', 10),
    ('agent_version', 1, 86400,'low',      10),
    ('disk_space',    1, 3600, 'medium',   5),
    ('server_uptime', 0, 300,  'low',      2),    -- informational, disabled by default
    ('db_integrity',  1, 604800,'high',     30);   -- weekly
```

### Health Checks

Each check is a function implementing:

```go
type CheckFunc func(ctx context.Context, repo *hub.Repo) health.CheckResult

type CheckResult struct {
    Status     health.Status  // pass, warn, fail, error
    Message    string
    DurationMs int64
    Details    interface{}    // optional extra data (JSON)
}
```

| Check | Implementation | Status logic |
|-------|---------------|--------------|
| `repo_exists` | `os.Stat(repo.Path)` | pass: exists and readable; fail: not found |
| `repo_config` | Read + JSON parse `opencode.json` or scan `.opencode/` | pass: valid; warn: missing fields; fail: invalid JSON / parse error |
| `agent_binary` | `exec.LookPath(binaryName)` | pass: found; fail: not in PATH; error: permission denied |
| `agent_version` | Run `binaryName --version`, parse semver | pass: matches; warn: outdated but compatible; fail: incompatible |
| `disk_space` | `syscall.Statfs` on `~/.ywai/` | pass: >20% free; warn: 10-20%; fail: <10% |
| `server_uptime` | Record start time, compute uptime | pass: <7d; warn: 7-30d; fail: >30d (should restart) |
| `db_integrity` | `PRAGMA integrity_check` on hub.db | pass: "ok"; fail: any other output |

### Package Structure

```
internal/health/
├── monitor.go       # HealthMonitor: runner, ticker, orchestrator
├── checks.go        # All check functions (repo_exists, agent_binary, etc.)
├── store.go         # SQLite store for checks + config
├── models.go        # CheckResult, CheckStatus, CheckConfig types
├── handler.go       # HTTP handlers for health endpoints
├── monitor_test.go
├── checks_test.go
└── store_test.go
```

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/health` | Aggregate health (pass/warn/fail) with count per status |
| GET | `/api/health/checks` | All check results (latest per check) |
| GET | `/api/health/checks?repo=<id>` | Filter by repo |
| POST | `/api/health/run` | Trigger all checks (body: `{checks: ["repo_exists"]}` to filter) |
| POST | `/api/health/run/{check}` | Trigger a specific check |
| GET | `/api/health/history/{check}` | Historical results for a check (query: from, to, limit) |
| GET | `/api/health/config` | Get check configurations |
| PATCH | `/api/health/config` | Update check configuration |

### UI Components

- **HealthBadge** — small dot/badge in sidebar header: green (all pass), yellow (any warn), red (any fail)
- **HealthPage** (`/health`) — full page with:
  - Summary cards: "X passing, Y warnings, Z failures"
  - Check list table: name, status (colored dot), last run (relative time), duration, message
  - "Run now" action per check and "Run all" button
  - Historical sparkline for each check (pass/fail over time)
- **HealthTimeline** — optional chart component showing pass rate over 7/14/30 day windows

### CLI Commands

```
ywai health              # Show aggregate health status
ywai health list         # List all checks with latest result
ywai health run [check]  # Run a specific check or all checks
ywai health config       # View check configurations
```

## Implementation Phases

### Phase 1: Health Engine + Checks (Sprint 1 — 2 days)

- [ ] Create `internal/health/` package with types and SQLite store
- [ ] Implement all 7 check functions in `checks.go`
- [ ] Implement `HealthMonitor` with ticker and runner
- [ ] Add health config seeding on first boot
- [ ] Add `GET /api/health` aggregate endpoint
- [ ] Add `POST /api/health/run` trigger endpoint
- [ ] Write unit tests for each check function

### Phase 2: Web UI Health Dashboard (Sprint 2 — 1 day)

- [ ] Build `HealthBadge` component in sidebar
- [ ] Build `HealthPage` with check list table
- [ ] Wire health Zustand store with polling (30s interval)
- [ ] Add "Run now" buttons with loading state
- [ ] Add history endpoint `GET /api/health/history/{check}`
- [ ] Style status colors (green/yellow/red) per yz-ui design system
- [ ] Write frontend tests

### Phase 3: History, CLI, Polish (Sprint 3 — 1 day)

- [ ] Add auto-purge goroutine (deletes records older than 30 days)
- [ ] Add `ywai health` CLI commands
- [ ] Add historical sparkline/chart to health page
- [ ] Add `PATCH /api/health/config` endpoint + settings UI
- [ ] Add WebSocket push for real-time check results
- [ ] Error handling: what happens if a check hangs or panics (timeout + recovery)
- [ ] Documentation: health monitoring in README

## Dependencies

- Multi-repo hub (ADR-001) — health checks run per repo; without hub, checks run against current directory only
- Control server must be running — checks run inside the daemon process

## Risks

- **Risk**: A check hangs (e.g., disk check on NFS mount).  
  **Mitigation**: Context-based timeout per check (configurable per check in `health_check_config.timeout_secs`).
- **Risk**: `db_integrity` check locks the database for too long.  
  **Mitigation**: Run `PRAGMA quick_check` instead of `integrity_check` (faster, less locking). Run only during low-activity periods.
- **Risk**: Frequent checks generate too many rows in SQLite.  
  **Mitigation**: Default intervals are conservative (hourly for most, weekly for DB check). 30-day auto-purge keeps the table bounded.
- **Risk**: Agent binary check produces false positives (user has multiple agent versions).  
  **Mitigation**: Check is per-registered-binary. If a repo uses `opencode`, only check for opencode. Allow disabling the check.

## Success Criteria

- [ ] All 7 health checks run and return correct pass/warn/fail status
- [ ] Aggregate health endpoint returns correct count per status
- [ ] Health badge in sidebar reflects current aggregate status
- [ ] Health page lists all checks with last run time and detail message
- [ ] "Run now" triggers a check and updates the UI in real-time
- [ ] History endpoint returns paginated check results
- [ ] Auto-purge removes records older than 30 days
- [ ] `ywai health` CLI shows same data as web UI
- [ ] All checks timeout gracefully if they hang

## Estimated Effort

- Phase 1: 2 days
- Phase 2: 1 day
- Phase 3: 1 day
- Total: 4 days
