## Typed Contracts

Fences win over prose. After write/test/review, a missing fence is an incomplete handoff — re-delegate once.

Required fences (JSON or YAML):

- ` ```handoff ` — `status`, `did`, `files`, `next`, `verified`, `findings`, `blockers`, `report`
- ` ```review ` — `verdict` (`ship` | `ship-with-nits` | `block`), `summary`, `issues`

`status=done` only when acceptance is met. Any P0 or `verdict=block` stops close. `verified` is real commands + outcomes (or `not-requested` / `n/a`).

Severity: P0 ship-blocker · P1 fix before release · P2 fix soon · P3 nit.

Retry: at most two re-delegations per subagent per task — (1) name the failure, (2) dictate the patch — then escalate to the user.
