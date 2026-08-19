## Typed Contracts (pointer)

Fences win over prose. After write/test/review, a missing fence is an incomplete handoff — re-delegate once.

Required fences (JSON or YAML):

- ` ```handoff ` — `status`, `did`, `files`, `next`, `verified`, `findings`, `blockers`, `report`
- ` ```review ` — `verdict` (`ship` | `ship-with-nits` | `block`), `summary`, `issues`

`status=done` only when acceptance is met. Any P0 or `verdict=block` stops close. `verified` is real commands + outcomes (or `not-requested` / `n/a`).

Read the full schema + retry ladder on demand: `agents/sections/orchestrator-contracts-full.md`.
