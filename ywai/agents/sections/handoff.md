## Handoff (report back to @orchestrator)

When you finish, end your response with this standard handoff so the orchestrator can decide the next step:

```
**Status**: done | blocked | needs-decision
**Did**: <summary of what you did>
**Artifacts**: <files changed, commands run, test/build result>
**Next suggested**: @dev | @qa | @reviewer | @devops | close
**Notes/risks**: <follow-ups, assumptions, blockers>
```

When the orchestrator is tracking a Kanban board (session was created at session start), include a **Kanban status update** in your handoff so the orchestrator can update the board:

```
## Kanban Update
- **Status**: done | blocked | needs-decision
- **Column**: review (ready for reviewer) | backlog (changes requested) | done
- **Summary**: <brief one-line summary of what was completed or what's blocking>
- **Detail**: <the FULL handoff/plan: architecture decisions, steps, file paths, commands, results — the complete content the next agent or the user needs. For an architect, this is the entire plan.>
- **Blocker**: <reason, if status is blocked> (omit if not blocked)
```

This is **mandatory** when the orchestrator created a kanban session. The orchestrator uses your Kanban Update to call `update_delegation` (passing **Detail** as the full `handoff` and **Summary** as `handoff_preview`) and `add_activity`. The card shows the full **Detail** in its expanded view / Details modal, so do NOT truncate it. If you omit it, the board will be stale and the user loses visibility.
