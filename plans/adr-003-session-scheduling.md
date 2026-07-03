# ADR-003: Session Scheduling and Calendar View

## Status

Proposed

## Context

Current ywai sessions are ad-hoc — you start one when you begin working and stop when you're done. There's no way to:
- Plan a session ahead of time ("run analysis on project X at 10am tomorrow")
- See upcoming sessions on a calendar
- Recur a session daily/weekly
- Get notified when a scheduled session is due

opencode-manager adds a **calendar view** and **session scheduling** — users can plan their OpenCode sessions by date/time, visually on a calendar, and sessions auto-start at their scheduled time.

ywai has:
- A kanban system with sessions and delegations
- A control server that runs persistently and could host a scheduler
- A web UI with routing that could host a calendar component
- Go's `time` package and `cron` libraries are available

What ywai lacks is a scheduler engine, a cron-like background ticker, and a calendar UI.

## Decision

Build a **Scheduler Engine** inside the control server and a **Calendar View** in the web UI.

**Scheduler Engine:**
- A goroutine running a ticker that checks every 30 seconds for due sessions
- Sessions are stored in a `scheduled_sessions` SQLite table with: id, repo_id, goal, scheduled_at, duration, recurrence (none/daily/weekly/weekdays), status (pending/running/completed/cancelled), created_at
- When a session is due: create a kanban delegation (reusing existing model), fire a notification via WebSocket to the UI, and optionally run `ywai session start` internal command
- The scheduler is optional — the user can start it with `ywai serve --scheduler` or enable it from settings
- Recurring sessions: after completion, calculate next occurrence and insert a new row

**Calendar View:**
- A new `/calendar` route in the web UI
- Calendar component (weeks or month grid) showing scheduled sessions as colored blocks
- Click a block to see details (repo, goal, status, duration)
- Click a day to create a new scheduled session (modal with form)
- Drag-and-drop to reschedule
- Filter by repo, status, agent type
- Data sourced from `/api/sessions/scheduled` endpoint

## Consequences

**Positive:**
- Proactive planning — sessions are intentional, not reactive
- Calendar provides visual clarity on work distribution across repos
- Recurring sessions automate routine workflows (daily standup review, weekly maintenance)
- Works with WebSocket notifications to alert user when a session starts

**Negative:**
- Scheduler adds a persistent background goroutine to the control server
- Timezone handling is complex (user might schedule across timezones)
- Recurring session logic needs careful edge-case handling (daylight saving, skipped dates)
- Calendar UI is non-trivial to build well (navigation, event overflow, mobile)
- Scheduler relies on the control server being up — if it's down, sessions are missed

**Neutral:**
- The scheduler can log missed sessions and offer a "catch up" button
- Calendar view can be built as a standalone React component first, then integrated
- We can start with simple scheduling and add recurrence in a later phase

## Alternatives Considered

- **OS cron job to trigger sessions**: More reliable but harder to integrate with the ywai UI and session model. No WebSocket notifications, no calendar.
- **External scheduling service (Sidekiq-like)**: Overkill for a local-first tool. A Go goroutine + ticker is sufficient and simple.
- **No scheduler, only calendar (manual start)**: Simpler but doesn't deliver the "set and forget" value. Scheduling is a key differentiator.
- **Use an existing Go cron library (e.g. `robfig/cron`)**: `robfig/cron/v3` is the standard. It handles recurrence, timezone, and daylight saving correctly. We'll use it in Phase 2 for recurrence; Phase 1 uses a simple ticker.

## References

- opencode-manager scheduling: calendar-based session planning, cron-like execution
- ywai's kanban sessions: `internal/kanban/models.go` — existing session/delegation model
- ywai's control server: `internal/control/server.go` — daemon that hosts the scheduler
- ywai's WebSocket hub: `internal/control/hub.go` — for session-start notifications
- `robfig/cron/v3` — standard Go cron library (MIT license)
