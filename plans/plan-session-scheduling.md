# Implementation Plan: Session Scheduling and Calendar View

## Overview

Add a scheduler engine to the ywai control server (background goroutine that starts sessions at predetermined times) and a calendar web UI for visually planning sessions. Supports one-off and recurring sessions.

## Goals

- Schedule a session for a specific date/time
- View scheduled sessions on a calendar (month/week/day)
- Recurring sessions: daily, weekly, weekdays
- Auto-start scheduled sessions when the time comes
- WebSocket notification when a session starts
- Drag-and-drop to reschedule on the calendar
- Edit, cancel, and manually trigger scheduled sessions

## Technical Design

### Architecture

```
Calendar UI (web) ──HTTP──► Control Server ──► Scheduler Engine
                                 │                    │
                                 │              ┌─────┴──────┐
                                 │              │  Ticker     │
                                 │              │  (30s)      │
                                 │              └─────┬──────┘
                                 │                    │
                                 ▼                    ▼
                          Kanban Store        Scheduled Sessions
                          (SQLite)            Table (SQLite)
```

The scheduler is a goroutine in the control server with a ticker. Every 30 seconds it queries for sessions where `scheduled_at <= now AND status = 'pending'`, transitions them to `running`, and creates kanban delegations.

### Data Model

```sql
CREATE TABLE scheduled_sessions (
    id            TEXT PRIMARY KEY,
    repo_id       TEXT NOT NULL,              -- FK to repos.id
    goal          TEXT NOT NULL,              -- session goal/description
    scheduled_at  TEXT NOT NULL,              -- ISO 8601
    duration      INTEGER DEFAULT 0,          -- minutes (0 = no limit)
    timezone      TEXT NOT NULL DEFAULT 'UTC',
    recurrence    TEXT NOT NULL DEFAULT 'none', -- none, daily, weekly, weekdays
    recurrence_end TEXT,                      -- optional end date for recurrence
    status        TEXT NOT NULL DEFAULT 'pending', -- pending, running, completed, cancelled, missed
    session_id    TEXT,                       -- kanban session ID when started
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);

CREATE INDEX idx_scheduled_pending ON scheduled_sessions(status, scheduled_at);
```

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/sessions/scheduled` | List scheduled sessions (query: status, repo, from, to) |
| POST | `/api/sessions/scheduled` | Create a scheduled session |
| GET | `/api/sessions/scheduled/{id}` | Get scheduled session details |
| PATCH | `/api/sessions/scheduled/{id}` | Update (reschedule, change goal, etc.) |
| DELETE | `/api/sessions/scheduled/{id}` | Cancel/delete a scheduled session |
| POST | `/api/sessions/scheduled/{id}/trigger` | Manually trigger a scheduled session now |
| GET | `/api/sessions/scheduled/range?from=&to=` | Calendar events between two dates |

### UI Components

- **CalendarPage** (`/calendar`) — main calendar view
  - Month grid (default), week view toggle
  - Color-coded events by repo
  - Click day → open "New Session" modal
  - Click event → open "Edit Session" modal
  - Drag event → reschedule (PATCH to API)
- **NewSessionModal** — form: repo selector, goal, date/time picker, duration, recurrence
- **EditSessionModal** — same form, pre-filled, with delete button
- **SessionIndicator** — subtle indicator in sidebar showing next scheduled session
- **NotificationToast** — WebSocket-driven "Session started!" notification

### Scheduler Engine

```go
// internal/scheduler/scheduler.go
type Scheduler struct {
    store     *ScheduledStore
    kanban    *kanban.Service
    hub       *ws.Hub  // for notifications
    ticker    *time.Ticker
    repoStore *hub.Registry
    mu        sync.Mutex
}

func (s *Scheduler) Start()
func (s *Scheduler) Stop()
func (s *Scheduler) tick()  // called every 30s by ticker
    - Query: SELECT * FROM scheduled_sessions WHERE status='pending' AND scheduled_at <= now()
    - For each:
      1. Create kanban session via session CLI engine
      2. Update scheduled_sessions.status = 'running', session_id = <new id>
      3. Send WebSocket notification
      4. If recurrence: compute next occurrence, insert new row

func computeNextRecurrence(current time.Time, recurrence string) time.Time
```

## Implementation Phases

### Phase 1: Scheduler Engine + Data Model (Sprint 1 — 4 days)

- [ ] Create `internal/scheduler/` package with `Scheduler` struct and data model
- [ ] Implement `ScheduledSession` model + SQLite store (CRUD + ranged queries)
- [ ] Create `scheduled_sessions` table migration
- [ ] Implement ticker goroutine with `tick()` logic (query due sessions, create delegations)
- [ ] Implement recurrence computation (daily, weekly, weekdays)
- [ ] Implement WebSocket notification on session start
- [ ] Add `POST /api/sessions/scheduled` and `GET /api/sessions/scheduled` endpoints
- [ ] Integrate scheduler with control server startup (start/stop lifecycle)
- [ ] Write unit tests for scheduler engine and recurrence

### Phase 2: Calendar UI (Sprint 2 — 4 days)

- [ ] Build CalendarPage component with month grid layout
- [ ] Integrate a calendar library (e.g., `react-calendar` or custom grid)
- [ ] Build NewSessionModal with form (repo selector, goal, datetime, recurrence)
- [ ] Build EditSessionModal with reschedule, edit, delete
- [ ] Add `GET /api/sessions/scheduled/range` endpoint for calendar data
- [ ] Wire Zustand store for scheduled sessions
- [ ] Implement drag-and-drop rescheduling (DnD library or native HTML5)
- [ ] Add SessionIndicator to sidebar
- [ ] Write frontend tests (component + store)

### Phase 3: Notification + Polish (Sprint 3 — 2 days)

- [ ] WebSocket integration for real-time session start notifications
- [ ] NotificationToast component
- [ ] "Missed sessions" handling (control server was down during scheduled time)
- [ ] Retry/triggers: button to manually start missed sessions
- [ ] Timezone picker in scheduling form
- [ ] Recurrence end date support
- [ ] Scheduler settings UI (enable/disable, interval config)
- [ ] Error handling: scheduling on a deleted repo
- [ ] Documentation: calendar workflow in user guide

## Dependencies

- Multi-repo hub (ADR-001 / plan-multi-repo-hub) — the scheduler schedules sessions *for* a repo
- Session CLI (ADR-002 / plan-session-cli) — the scheduler uses `session start` internally
- WebSocket hub already exists in `internal/control/hub.go`
- Calendar library: either a lightweight one (`react-calendar`) or custom

## Risks

- **Risk**: Control server restart causes missed scheduled sessions.  
  **Mitigation**: On boot, check for pending sessions with `scheduled_at < now` and offer to trigger or mark as missed.
- **Risk**: Timezone handling bugs (daylight saving transitions).  
  **Mitigation**: Use Go's `time.LoadLocation` and `robfig/cron` for recurrence in Phase 3. Store timezone per session.
- **Risk**: User schedules a session and forgets — nothing happens visibly.  
  **Mitigation**: WebSocket notification + optional system notification (Phase 3).
- **Risk**: Calendar UI with many events gets slow.  
  **Mitigation**: Virtualized grid, page events per month (only fetch visible range).

## Success Criteria

- [ ] User can schedule a session for tomorrow at 10am and it auto-starts
- [ ] Calendar shows the scheduled session at the correct date/time
- [ ] Dragging an event to a new time reschedules it
- [ ] Daily recurring session creates a new entry after each completion
- [ ] WebSocket notifies the UI when a scheduled session starts
- [ ] Missed sessions (server was down) are detectable and triggerable
- [ ] All CRUD operations work through API and UI

## Estimated Effort

- Phase 1: 4 days
- Phase 2: 4 days
- Phase 3: 2 days
- Total: 10 days
