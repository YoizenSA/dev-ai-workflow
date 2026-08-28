## Typed Contracts (orchestrator)

Prefer fenced blocks over free-form prose. When both exist, **the fence wins**.
Missing required fence after a write/test/review phase → treat handoff as **incomplete** and re-delegate once.

### Worker handoff — require ` ```handoff `

Subagents end with JSON (or YAML equivalent):

````markdown
```handoff
{
  "status": "done | blocked | needs-decision",
  "did": "<one line>",
  "files": ["<paths>"],
  "next": "dev | qa | reviewer | devops | close | null",
  "verified": [{"command": "<exact>", "outcome": "<real>"}]
  | "not-requested" | "n/a",
  "findings": [{"path": "", "severity": "P0|P1|P2|P3", "message": ""}],
  "blockers": [],
  "report": {"summary": "<one line>", "detail": "<what next agent needs>"}
}
```
````

| `status` | Action |
|---|---|
| `done` | Advance using `next` when sensible |
| `blocked` / `needs-decision` | Resolve via `question` or sharper re-delegation |
| any `findings` severity `P0` | Do not close the goal |

After write/test work, `verified` must list real commands and outcomes — never "should pass".

### Review ship gate — require ` ```review `

````markdown
```review
{
  "verdict": "ship | ship-with-nits | block",
  "summary": "<1-2 sentences>",
  "issues": [{"path": "", "severity": "P0|P1|P2|P3", "message": "", "fix_hint": ""}]
}
```
````

| Condition | Action |
|---|---|
| `verdict: block` or any P0 | Do not close/deploy; re-open fixer or ask user |
| `ship-with-nits` (P2/P3 only) | May continue; track nits |
| `ship` and no P0/P1 | Continue / close |

### Severity

| Level | Meaning |
|---|---|
| P0 | Ship-blocker (security, data loss, critical path, false green) |
| P1 | Must fix before release |
| P2 | Should fix soon |
| P3 | Nit |

### Brief shape (write agents)

Every write delegation brief: **Goal · Context · Acceptance · Files · Verification · Return format** (`handoff` + `verified`). Optional: `mode: mechanical` | `mode: judgment`.

### Retry ladder

Two re-delegations max per subagent per task: (1) specific failure, (2) dictated patch in the brief, (3) escalate to user. You do not silently loop.
