## Handoff

End with a fenced `handoff` block. Prose above is fine; the fence is mandatory for routing.

````markdown
```handoff
{
  "status": "done | blocked | needs-decision",
  "did": "<one line>",
  "files": ["<paths changed or inspected>"],
  "next": "dev | qa | reviewer | devops | close | null",
  "verified": [{"command": "<exact>", "outcome": "<real>"}]
  | "not-requested" | "n/a",
  "findings": [],
  "blockers": [],
  "report": {"summary": "<one line>", "detail": "<full handoff for next agent — do not truncate>"}
}
```
````

- **status**: `done` only when acceptance criteria are met.
- **verified**: after write/test work, real commands + outcomes. Use `not-requested` / `n/a` when appropriate.
- **findings**: optional; severity `P0`|`P1`|`P2`|`P3` (`P0` = ship-blocker).
