# ADR-005: Health Monitoring for Repos and Agents

## Status

Proposed

## Context

ywai currently has no visibility into the health of its managed repos or agents. If:
- A repo's directory is moved or deleted
- An agent binary (opencode, claude-code) is uninstalled or broken
- A repo's opencode.json is corrupted or invalid
- The control server's SQLite database is running low on disk space

There's no way for the user to know until something breaks.

opencode-manager adds **health monitoring** — a dashboard showing uptime, status, and health checks for all registered repos and installed agents.

ywai already has:
- A `ywai doctor` command (via gentle-ai) that checks the ecosystem
- Agent detection logic (`internal/agent/`)
- Agent profile management (`internal/agents/`)
- A persistent control server on port 5768
- SQLite for storing persistent data
- The push_store.go mechanism for status tracking

What ywai lacks is a continuous health monitoring system — checks that run periodically, surface results in the web UI, and optionally send alerts.

## Decision

Build a **Health Monitor** — a background goroutine in the control server that periodically runs checks and exposes results via API and UI.

**Health Checks:**

| Check | What it tests | Interval | Severity |
|-------|--------------|----------|----------|
| repo_exists | Repo directory exists and is readable | Every boot + on-demand | Critical |
| repo_config | `opencode.json` or `.opencode/` config is valid JSON | Every boot + on-demand | High |
| agent_binary | Agent binary exists in PATH and is executable | Every boot + on-demand | Critical |
| agent_version | Agent version matches expected range | Daily | Low |
| disk_space | Free disk space on ~/.ywai/ partition | Hourly | Medium |
| server_uptime | Control server has been running without restart | Continuous | Low |
| db_integrity | SQLite `PRAGMA integrity_check` | Weekly | High |
| git_status | Last commit date, branch, unpushed commits | On-demand | Low |

**Data model:**
- `health_checks` table in SQLite: id, repo_id, check_name, status (pass/warn/fail/error), message, last_run_at, duration_ms, next_run_at
- `health_check_config` table: check_name, enabled, interval_seconds, severity

**API endpoints:**
- `GET /api/health` — aggregate health status (pass/warn/fail)
- `GET /api/health/checks` — all check results, optionally filtered
- `POST /api/health/run` — trigger a specific check or all checks
- `GET /api/health/history?check=<name>` — historical results for a check

**UI:**
- Health badge in the sidebar/topbar (green/yellow/red)
- Health page at `/health` showing all checks with last result and next run time
- Each check row shows: name, status (colored dot), last run time, duration, detail message
- "Run now" button per check
- Health timeline chart showing pass/fail over time

## Consequences

**Positive:**
- Proactive issue detection (broken agent, deleted repo, full disk)
- Single dashboard for all repo and agent health
- Health data feeds into scheduling (don't schedule on a broken repo)
- Historical data helps diagnose intermittent issues
- Low overhead — checks are cheap and run on configurable intervals

**Negative:**
- Adds background goroutines to the control server
- Health check data consumes SQLite space over time (mitigated by retention policy)
- Disk space check differs by OS — needs platform-specific handling
- Agent binary check may be noisy if user switches between agents
- Some checks (git_status) require running external commands — potential security concern in multi-user setups

**Neutral:**
- The health monitor is independent — it doesn't affect any other feature if it fails
- Retention: keep 30 days of check history, auto-purge older records
- Checks are on-demand by default; user enables periodic checks in settings

## Alternatives Considered

- **Use gentle-ai doctor**: Already exists but is CLI-only, not continuous, and doesn't cover repo health.
- **External monitoring (Prometheus + Grafana)**: Too heavy for a local dev tool. Overkill.
- **File-based health state (no DB)**: Loses history. DB is already there and cheap.
- **No health monitoring**: Simpler, but the feature pays for itself the first time it catches a broken agent or deleted repo.
- **Push-based health from agents**: Requires modifying agent code. Pull-based (ywai checks) is simpler and doesn't require agent cooperation.

## References

- opencode-manager health: repo uptime, agent status, health dashboard
- ywai's `ywai doctor`: `internal/gentlai/` — existing ecosystem health check
- ywai's agent detection: `internal/agent/` — binary detection logic
- ywai's push store: `internal/control/push_store.go` — status tracking pattern
- Control server: `internal/control/server.go` — daemon that hosts the monitor
