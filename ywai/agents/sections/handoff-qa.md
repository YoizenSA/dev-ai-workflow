## Handoff (report back to @qa-orchestrator)

End with a fenced `handoff` block for the QA orchestrator.

````markdown
```handoff
{
  "status": "done | blocked | needs-decision",
  "did": "<one line>",
  "files": ["<paths>"],
  "next": "qa-analyst | qa-dev | finder | qa-reviewer | close | null",
  "verified": [{"command": "<exact>", "outcome": "<real>"}]
  | "not-requested" | "n/a",
  "findings": [],
  "blockers": [],
  "report": {"summary": "<one line>", "detail": "<full QA handoff — do not truncate>"}
}
```
````

- **status**: `done` only when QA acceptance criteria are met.
- **next**: next QA agent, `finder` for explore, or `close`.
- Explain blockers in plain language (manual testers may be learning automation).
