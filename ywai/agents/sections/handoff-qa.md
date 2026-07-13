## Handoff (report back to @qa-orchestrator)

When you finish, end your response with a **fenced** `handoff` block so the QA orchestrator can parse it. Prose above the fence is fine for humans; the fence is mandatory for routing.

````markdown
```handoff
status: done | blocked | needs-decision
did: <summary of what you did>
artifacts:
  - path: <file path, command, or test id>
    kind: file | command | test
next: qa-analyst | qa-dev | qa-finder | qa-reviewer | close | null
risks:
  - <follow-up, assumption, or blocker>
findings: []   # optional; severity P0|P1|P2|P3
kanban:
  column: review | backlog | done
  summary: <one-line summary>
  detail: <FULL handoff: findings, steps, paths, commands, test results — do not truncate>
```
````

### Field rules

- **status**: `done` only when the QA acceptance criteria are met.
- **next**: next QA agent, or `close`.
- **kanban.detail**: full content for the next agent or learner — never truncate.
- Explain blockers in plain language (manual testers may be learning automation).

### Severity (when using findings)

| Level | Meaning |
|---|---|
| P0 | Ship-blocker / test completely wrong or unsafe |
| P1 | Must fix before trusting the suite |
| P2 | Should improve soon |
| P3 | Nit / teaching note |

### Legacy Kanban prose (optional extra)

If present and it conflicts with the fence, **the fence wins**.

```
## Kanban Update
- **Status**: done | blocked | needs-decision
- **Column**: review | backlog | done
- **Summary**: <one line>
- **Detail**: <same as kanban.detail>
- **Blocker**: <reason if blocked>
```

This is **mandatory** when the orchestrator tracks a Kanban board.
