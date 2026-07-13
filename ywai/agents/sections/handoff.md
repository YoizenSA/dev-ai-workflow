## Handoff (report back to the orchestrator)

When you finish, end your response with a **fenced** `handoff` block so the orchestrator can parse it. Prose above the fence is fine for humans; the fence is mandatory for routing.

````markdown
```handoff
status: done | blocked | needs-decision
did: <summary of what you did>
artifacts:
  - path: <file path, command, or test id>
    kind: file | command | test
next: dev | qa | reviewer | devops | close | null
risks:
  - <follow-up, assumption, or blocker>
findings: []   # optional; use severity P0|P1|P2|P3 when reporting issues
kanban:
  column: review | backlog | done
  summary: <one-line summary>
  detail: <FULL handoff/plan: decisions, steps, paths, commands, results — do not truncate>
```
````

### Field rules

- **status**: `done` only when acceptance criteria are met; `blocked` / `needs-decision` when the orchestrator or user must act.
- **next**: who should run next (`close` when nothing remains).
- **kanban.detail**: full content for the next agent or user — never truncate.
- **findings**: include when you discovered issues; `P0` = ship-blocker.

### Severity (when using findings)

| Level | Meaning |
|---|---|
| P0 | Ship-blocker |
| P1 | Must fix before release |
| P2 | Should fix soon |
| P3 | Nit |

### Legacy Kanban prose (optional extra)

You may also include a short human-readable Kanban Update; if it conflicts with the fence, **the fence wins**.

```
## Kanban Update
- **Status**: done | blocked | needs-decision
- **Column**: review | backlog | done
- **Summary**: <one line>
- **Detail**: <same as kanban.detail>
- **Blocker**: <reason if blocked>
```

This is **mandatory** when the orchestrator tracks a Kanban board: the orchestrator uses `kanban.detail` / Detail as the full `handoff` and `kanban.summary` as `handoff_preview`.
