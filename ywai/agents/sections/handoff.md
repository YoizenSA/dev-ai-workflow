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
verified:
  - command: <exact command run>
    outcome: <real exit status / short real output>
# or: verified: not-requested | n/a
report:
  summary: <one-line summary>
  detail: <FULL handoff/plan: decisions, steps, paths, commands, results — do not truncate>
```
````

### Field rules

- **status**: `done` only when acceptance criteria are met; `blocked` / `needs-decision` when the orchestrator or user must act.
- **next**: who should run next (`close` when nothing remains).
- **report.detail**: full content for the next agent or user — never truncate.
- **findings**: include when you discovered issues; `P0` = ship-blocker.
- **verified**: after write/test work, list the exact command(s) you ran and their real outcome. "Should pass" is not allowed. Use `not-requested` when the brief did not ask for verification, or `n/a` for pure read-only research.

### Severity (when using findings)

| Level | Meaning |
|---|---|
| P0 | Ship-blocker |
| P1 | Must fix before release |
| P2 | Should fix soon |
| P3 | Nit |
