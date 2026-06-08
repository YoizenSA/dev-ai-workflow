# Missions UI Research Report

## Overview

This report synthesizes research from Factory.ai Missions, builderz-labs/mission-control,
Ralph TUI, Conduit, Claude Squad, Grove TUI, Hermes Agent Mission Control, and Agents UI
to define the UI components and patterns needed for a multi-agent orchestration "Mission Control" system.

---

## 1. Key UI Components Needed

Based on the research of 7+ mission-control/multi-agent TUI systems, the following components
are essential:

### 1.1 Mission List / Plan View
- **Goal**: Show all active, queued, and completed missions
- **Pattern**: Left sidebar or top-level tab list; each entry shows mission name, status badge
  (planning / in-progress / blocked / completed / failed), and a progress bar
- **Sources**: Factory.ai sidebar, builderz-labs task panel, Ralph TUI task list

### 1.2 Mission Detail / Orchestration View
- **Goal**: Deep-dive into a single mission's structure and execution state
- **Pattern**: Split-panel layout —
  - **Left**: Feature/milestone tree (hierarchical, collapsible)
  - **Right**: Detail panel showing logs, status, worker output, and controls
- **Sources**: Factory.ai Mission Control (screenshot shows tree + detail panel),
  Ralph TUI (task detail on right), builderz-labs (feature tree)

### 1.3 Progress Dashboard / Overview
- **Goal**: At-a-glance status of all active work
- **Pattern**: Overview stats bar at top —
  - Total features / completed / in-progress / blocked
  - Token usage and estimated cost
  - Run count (worker + validator runs)
  - Active time / elapsed duration
- **Sources**: Factory.ai architecture blog post, builderz-labs token dashboard,
  Hermes analytics panel

### 1.4 Live Log Viewer / Streaming Output
- **Goal**: See what a worker agent is doing right now
- **Pattern**: Scrollable log panel with real-time append, color-coded by level
  (info/warn/error/agent-thought), filterable by feature or worker
- **Sources**: Ralph TUI live agent output, Conduit real-time streaming,
  Claude Squad terminal preview pane, Factory.ai streaming logs

### 1.5 Validation Results Panel
- **Goal**: Show validation outcomes per milestone
- **Pattern**: Tabbed or expandable sections showing —
  - Pass/fail per validation run
  - Number of issues found (blocking / non-blocking / suggestions)
  - Issues list with details and linked feature IDs
- **Sources**: Factory.ai reliability loop data, builderz-labs Aegis review system

### 1.6 Intervention / Control Panel
- **Goal**: Pause, resume, redirect, or kill agents
- **Pattern**: Action buttons per feature/milestone —
  - Pause / Resume (orchestrator-level)
  - Retry (single feature or milestone)
  - Skip / Cancel
  - "Talk to orchestrator" chat input
- **Sources**: Factory.ai intervention docs, Ralph TUI pause/resume/kill,
  Hermes control panel

### 1.7 Status Bar / Footer
- **Goal**: Always-visible system state
- **Pattern**: Bottom bar showing —
  - Active agent count
  - Running time
  - Model usage / token spend (running total)
  - Key shortcuts (help)
- **Sources**: Ralph TUI footer, Conduit status bar, Claude Squad bottom panel

### 1.8 Cost / Token Usage Panel
- **Goal**: Monitor spend in real time
- **Pattern**: Bar charts or summary tables showing —
  - Tokens per model (input, output, cache read)
  - Estimated cost per feature/milestone
  - Cumulative spend
- **Sources**: builderz-labs token-dashboard, Hermes usage analytics,
  Factory.ai architecture data (778.5M tokens, 38.8k lines)

### 1.9 Timeline / Gantt View
- **Goal**: Visualize execution ordering and dependencies
- **Pattern**: Horizontal timeline showing milestones and features as bars,
  with dependency arrows and concurrent execution indicators
- **Sources**: Ralph TUI multi-epic support, Factory.ai milestone ordering

---

## 2. Layout Recommendations

### TUI Layout (Recommended for CLI-first tools)

```
┌──────────────────────────────────────────────────────────────┐
│  MISSION CONTROL                             Status Bar     │
│  Mission: "Build Auth System"               v0.1.0          │
├──────────────────────┬───────────────────────────────────────┤
│                      │                                       │
│  FEATURE TREE        │  DETAIL PANEL                         │
│                      │                                       │
│  📋 Plan (3/6 done)  │  ┌─────────────────────────────────┐ │
│  ├── Milestone 1 ✅  │  │ Feature: "Implement JWT tokens" │ │
│  │  ├── Feat A ✅    │  │ Status: ● In Progress (5m 12s)  │ │
│  │  ├── Feat B ✅    │  │ Agent: worker-3 (Claude Sonnet) │ │
│  │  └── Feat C 🔄   │  │ Model: sonnet-4-6               │ │
│  ├── Milestone 2 🔄  │  │ Tokens: ↑ 12.4k / ↓ 1.2k       │ │
│  │  ├── Feat D 🔄   │  ├─────────────────────────────────┤ │
│  │  └── Feat E ⏳   │  │  ╭─ worker-3 ● reasoning...     │ │
│  └── Milestone 3 ⏳  │  │  │ > Generated token.go         │ │
│                      │  │  │ > Running go test ./auth/... │ │
│                      │  │  │  ✓ Tests pass (24/24)        │ │
│                      │  │  ╰─ output ▲ 83 lines           │ │
│                      │  ├─────────────────────────────────┤ │
│                      │  │ [▶ Pause] [↻ Retry] [✕ Cancel]  │ │
│                      │  └─────────────────────────────────┘ │
│                      │                                       │
├──────────────────────┴───────────────────────────────────────┤
│  💰 Tokens: 45.2M / $12.30  │  ⏱ 23m 18s  │  ✦ ? Help      │
└──────────────────────────────────────────────────────────────┘
```

### Web UI Layout (for browser-based dashboards)

```
┌──────────────────────────────────────────────────────────────┐
│  LOGO   Missions Dashboard                    [User] [⚙]    │
├──────────────────────────────────────────────────────────────┤
│  [Overview] [Active Missions] [History] [Settings]           │
├───────────────────────────────────┬──────────────────────────┤
│                                   │                          │
│  STATS ROW                        │  LIVE ACTIVITY FEED      │
│  ┌──────┐┌──────┐┌──────┐┌────┐  │                          │
│  │Total ││Active││Done  ││Cost │  │  [10:32] ✅ Milestone    │
│  │  12  ││   3  ││  8   ││$47  │  │  1 validation passed    │
│  └──────┘└──────┘└──────┘└────┘  │  [10:28] 🔄 worker-4     │
│                                   │  started "Feat D"        │
│  MISSION TABLE                    │  [10:15] ❌ Feat C       │
│  ┌──────────────────────────────┐ │  validation failed (2)   │
│  │ Mission          Status  Prg │ │  [10:00] 🎯 Milestone    │
│  │ Build Auth Sys   ▶ Active 62%│ │  2 checkpoint            │
│  │ API Migration    ⏸ Paused 34%│ │                          │
│  │ Refactor DB      ✅ Done 100%│ │                          │
│  └──────────────────────────────┘ │                          │
│                                   │                          │
├───────────────────────────────────┴──────────────────────────┤
│  Footer: Agent status, docs links, version                    │
└──────────────────────────────────────────────────────────────┘
```

---

## 3. What Information to Show at Each Level

### 3.1 Mission Level (Overview)

| Piece of Info | Why It Matters | Source |
|---|---|---|
| Mission name & description | Identify the work | All systems |
| Overall progress (% complete) | Quick health check | Factory.ai, Ralph TUI |
| Status (planning / active / paused / complete / failed) | Know if it's running | Factory.ai |
| Feature count (total / done / in-progress / blocked) | Work breakdown | Factory.ai, builderz-labs |
| Token usage & estimated cost (cumulative) | Cost awareness | builderz-labs, Hermes |
| Run count (worker + validator) | Understand complexity | Factory.ai architecture blog |
| Elapsed time | Duration tracking | Ralph TUI |
| Milestone summary | Checkpoint status | Factory.ai |
| Validation pass rate per milestone | Quality signal | Factory.ai architecture blog |
| Orchestrator model info | Know which models used | Factory.ai settings |

### 3.2 Milestone Level

| Piece of Info | Why It Matters | Source |
|---|---|---|
| Milestone name & goal | Purpose | Factory.ai |
| Status (pending / active / validating / complete / failed) | Current phase | Factory.ai |
| Features within milestone (tree) | What's included | Factory.ai, Ralph TUI |
| Validation round count | How many fix cycles | Factory.ai architecture blog |
| Issues found (blocking / non-blocking / suggestions) | Quality | Factory.ai architecture blog |
| Whether scrutiny & user-testing were run | Validation depth | Factory.ai docs |
| Dependency status (previous milestone must pass first) | Ordering | Factory.ai architecture blog |
| Estimated vs actual effort | Planning accuracy | N/A (deduced) |

### 3.3 Feature Level (Worker)

| Piece of Info | Why It Matters | Source |
|---|---|---|
| Feature name & description | Identity | Factory.ai, Ralph TUI |
| Status (queued / running / done / blocked) | Progress | All systems |
| Worker agent ID | Traceability | Factory.ai, builderz-labs |
| Model in use | Cost/quality | Factory.ai |
| Live log / streaming output | Real-time visibility | Ralph TUI, Conduit |
| Token usage per feature | Granular cost tracking | builderz-labs |
| Execution duration | Performance | Ralph TUI |
| Exit code / error message | Failure diagnosis | Claude Squad |
| Files modified (diff summary) | Change awareness | Claude Squad |
| Retry count | Problem detection | N/A (deduced) |

### 3.4 Worker Level (Agent Execution)

| Piece of Info | Why It Matters | Source |
|---|---|---|
| Agent thoughts / reasoning chain | What it's thinking | Ralph TUI, Hermes |
| Tool calls (read, edit, bash, etc.) | What it's doing | Ralph TUI, Agents UI |
| File paths being read or written | Change tracking | Agents UI, Claude Squad |
| Command output / test results | Outcomes | Claude Squad, Ralph TUI |
| Token count per turn | Granular cost | Conduit |
| Current step description | Context | Ralph TUI |
| Elapsed time for current step | Stuck detection | Ralph TUI |
| Errors encountered | Debugging | All systems |

---

## 4. TUI vs Web UI Considerations

### When to Choose TUI

| Reason | Examples |
|---|---|
| Developer workflow lives in terminal | Ralph TUI, Conduit, Claude Squad |
| Need to spawn/manage subprocess agents | Claude Squad (tmux-based) |
| Running headless / SSH sessions | Conduit (works over SSH) |
| Keyboard-driven efficiency | Vim-style keybindings (Conduit) |
| Lower resource overhead | Terminal rendering vs browser |
| Integration with existing CLI tools | Ralph TUI (direct pipe to agents) |

**Best for**: Individual developers, CLI-first tools, headless servers, SSH workflows

### When to Choose Web UI

| Reason | Examples |
|---|---|
| Richer visualizations | builderz-labs (32 panels) |
| Team collaboration / sharing | Factory.ai web Missions |
| Mouse-friendly interaction | builderz-labs, Factory.ai |
| Complex data (charts, graphs, cost analytics) | builderz-labs token-dashboard |
| Multi-user access / RBAC | builderz-labs (viewer/operator/admin) |
| Persistent (survives SSH disconnect) | Factory.ai web |
| API-driven (programmatic access) | builderz-labs, Factory.ai |

**Best for**: Teams, complex dashboards, non-technical stakeholders, persistent monitoring

### Hybrid Approach (Best of Both)

Some systems offer both modes:
- **Conduit**: TUI + Web serve (coming soon)
- **Factory.ai**: CLI `/missions` + web dashboard
- **Agents UI**: Native macOS app with MCP tools + terminal
- **builderz-labs/mission-control**: Web UI that spawns/coordinates terminal agents

### Recommendation

For a **ywai missions system**, a **TUI-first approach** is the strongest fit because:

1. **ywai is CLI-native** — users already work in terminals
2. **Integrates naturally** — can spawn agents (Claude Code, OpenCode) as child processes
3. **Lower friction** — no need to open a browser; stay in the dev workflow
4. **Keyboard-driven** — faster iteration for developer users
5. **Progressive enhancement** — Could add a web-mode (`ywai missions serve --port 8080`)
   later for team sharing

The TUI should use a **split-panel layout** (left: feature tree, right: detail/logs)
inspired by Ralph TUI and Claude Squad, with a bottom status bar inspired by Conduit.

---

## 5. Key Design Patterns from Research

### Pattern A: The Feature Tree + Detail Split
Used by: Factory.ai, Ralph TUI, builderz-labs
- Left panel: nested tree of milestones → features
- Right panel: show detail of selected item (logs, status, controls)
- Bottom bar: always-visible system status
- This is the most proven and recommended layout

### Pattern B: Real-time Streaming in Detail Panel
Used by: Ralph TUI, Conduit, Claude Squad
- Streaming agent output updates in-place (no full redraws)
- Color-coded by source (agent thought vs tool call vs output)
- Auto-scroll to newest content, with ability to scroll back
- Pause auto-scroll when user scrolls up

### Pattern C: State Recovery / Persistence
Used by: Ralph TUI, Claude Squad
- Session state saved to disk as JSON (`.ralph-tui/session.json`)
- On restart, resume from last known state
- Key for long-running missions that may be interrupted

### Pattern D: Multi-model Orchestrator/Worker Split
Used by: Factory.ai
- Orchestrator uses strong model (Claude Opus) for planning/validation
- Workers use cheaper/faster models (Claude Sonnet/Haiku) for implementation
- UI should show which model each worker is using

### Pattern E: Human-in-the-Loop Checkpoints
Used by: Factory.ai, Ralph TUI
- At milestone boundaries, allow user to review validation results
- User can approve, request changes, or abort
- Not fire-and-forget; treat user as project manager

---

## 6. Common Pitfalls to Avoid

1. **Fire-and-forget** — Always show what agents are doing; never hide execution
2. **No state persistence** — Missions can run for hours/days; must survive crashes
3. **Log overload** — Filter by feature, level, and worker; don't dump raw logs
4. **No intervention** — User must be able to pause/resume/redirect at any granularity
5. **Missing cost visibility** — Token/cost tracking is essential for trust
6. **No validation visibility** — Show what validators found and what was fixed
7. **Flat lists vs trees** — Hierarchical (mission → milestone → feature) is essential
   for complex projects

---

## 7. Feature Ideas for ywai Missions

### Must-Have (MVP)
- Feature tree with milestone grouping
- Live streaming agent output per feature
- Status badges (queued/running/done/blocked/failed)
- Pause/resume per feature and per mission
- Bottom status bar (elapsed time, token count)
- Session persistence (resume after crash)

### Nice-to-Have (V2)
- Validation results panel (pass/fail, issues list)
- Cost/token tracking per feature and cumulative
- Timeline/Gantt view of execution
- Web mode (`ywai missions serve`)
- Multi-agent parallel execution view
- "Talk to orchestrator" intervention input
- Model selection per worker (orchestrator vs worker split)
- GitHub integration (commit status, PR creation)

### Advanced (Future)
- Mission templates / presets
- Scheduled / recurring missions
- Collaborative mode (share session via URL)
- Custom agent profiles per mission
- Automated retry on failure
- Aegis-style quality gates
