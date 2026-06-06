# Legacy → WebApi + Angular Migration Workflow

## 1. Overview

The migration workflow moves legacy Yoizen surfaces to a modern WebApi +
Angular stack through a structured, evidence-first pipeline. Every phase is
gated: **scope** classifies the surface and detects split opportunities,
**plan** builds a dependency graph with row-level evidence, **build** (@dev)
implements task rows, and **validate** checks all nine parity axes against
legacy source evidence. Remediation loops are narrow and tracked via a focused
validator before re-entering full validation. The orchestrator coordinates all
phases through Kanban-backed delegation, never writing application code
itself. Every handoff is evidence-backed — no phase advances on assumptions.

## 2. Agent Map

```
                        ┌─────────────────────────────────┐
                        │    migration-orchestrator        │
                        │  (coordinates everything)        │
                        │  - reads handoffs, updates       │
                        │    migration-progress-tracker    │
                        └──────┬──────┬──────┬────────────┘
                               │      │      │
               ┌───────────────┘      │      └───────────────┐
               ▼                      ▼                      ▼
 ┌──────────────────────┐  ┌──────────────────┐  ┌──────────────────────┐
 │   migration-scope    │  │ migration-       │  │  migration-          │
 │   (classify surface) │  │ planner          │  │  validator           │
 │                      │  │ (create plan     │  │  (validate parity    │
 │  Output:             │  │  with evidence)  │  │   across 9 axes)    │
 │  SCOPE_SINGLE_PLAN   │  │                  │  │                      │
 │  SCOPE_SPLIT_        │  │  Output:         │  │  Output:             │
 │    RECOMMENDED       │  │  PLAN_READY      │  │  APPROVED | REJECTED │
 │  AMBIGUOUS           │  │  PLAN_BLOCKED    │  │  BLOCKED             │
 └──────────┬───────────┘  └────────┬─────────┘  └───────────┬──────────┘
            │                       │                        │
            │                       │     ┌──────────────────┘
            │                       │     │
            └───────────────────────┼─────┘
                                    │
                                    ▼
                          ┌──────────────────┐
                          │    @dev          │
                          │  (build tasks    │
                          │   from plan      │
                          │   rows)          │
                          │                  │
                          │  Output:         │
                          │  BUILD_COMPLETE  │
                          └────────┬─────────┘
                                   │
                                   │  (remediation loop)
                                   ▼
                         ┌──────────────────────────┐
                         │  migration-validator-    │
                         │  focused                 │
                         │  (narrow check on        │
                         │   specific remediation   │
                         │   tasks)                 │
                         │                          │
                         │  Output:                 │
                         │  FOCUSED_APPROVED        │
                         │  FOCUSED_REJECTED        │
                         │  ESCALATE                │
                         └──────────────────────────┘
```

## 3. Full State Machine

```
                           MIGRATION_START
                                 │
                                 ▼
                    ┌──────────────────────┐
                    │       SCOPE          │
                    │  (migration-scope    │
                    │   classifies         │
                    │   surface)           │
                    └──────────┬───────────┘
                               │
               ┌───────────────┴───────────────┐
               │                               │
               ▼                               ▼
    ┌──────────────────────┐      ┌──────────────────────────┐
    │  SCOPE_SINGLE_       │      │  SCOPE_SPLIT_            │
    │  PLAN                │      │  RECOMMENDED             │
    │  (one linear plan)   │      │  (parent + child         │
    └──────────┬───────────┘      │   work graph)            │
               │                  └───────────┬──────────────┘
               │                              │
               ▼                              ▼
    ┌──────────────────────┐      ┌──────────────────────────┐
    │       PLAN           │      │    PARENT PLAN           │
    │  (migration-planner  │      │    + CHILD PLANS         │
    │   creates full plan) │      │    (one planner call     │
    │                      │      │     per plan)            │
    └──────────┬───────────┘      └───────────┬──────────────┘
               │                              │
               ▼                              ▼
    ┌──────────────────────┐      ┌──────────────────────────┐
    │       BUILD          │      │    BUILD                 │
    │  (@dev implements    │      │    (parallel child       │
    │   plan task rows)    │      │     builds; parent       │
    │                      │      │     serial gate)         │
    └──────────┬───────────┘      └───────────┬──────────────┘
               │                              │
               ▼                              ▼
    ┌──────────────────────┐      ┌──────────────────────────┐
    │     VALIDATE         │      │    VALIDATE              │
    │  (migration-         │      │    (children first,      │
    │   validator checks   │      │     then parent          │
    │   all 9 axes)        │      │     final gate)          │
    └──────────┬───────────┘      └───────────┬──────────────┘
               │                              │
       ┌───────┴───────┐              ┌───────┴───────┐
       ▼               ▼              ▼               ▼
   APPROVED        REJECTED      APPROVED        REJECTED
       │               │              │               │
       ▼               ▼              ▼               ▼
   COMPLETE     ┌──────────────┐  COMPLETE    ┌──────────────┐
                │ REMEDIATION  │              │ REMEDIATION  │
                │ (@dev fixes  │              │ (@dev fixes  │
                │  findings)   │              │  findings)   │
                └──────┬───────┘              └──────┬───────┘
                       │                             │
                       ▼                             ▼
                ┌──────────────┐              ┌──────────────┐
                │  VALIDATE    │              │  VALIDATE    │
                │  FOCUSED     │              │  FOCUSED     │
                │  (narrow     │              │  (narrow     │
                │   check)     │              │   check)     │
                └──────┬───────┘              └──────┬───────┘
                       │                             │
               ┌───────┴───────┐             ┌───────┴───────┐
               ▼               ▼             ▼               ▼
        FOCUSED_         FOCUSED_       FOCUSED_        FOCUSED_
        APPROVED         REJECTED       APPROVED        REJECTED
            │                │              │               │
            ▼                ▼              ▼               ▼
       FULL VALIDATE    back to      FULL VALIDATE    back to
       (validator)      BUILD        (validator)      BUILD
            │                │              │               │
            ▼                │              ▼               │
        APPROVED             │          APPROVED            │
            │                │              │               │
            ▼                │              ▼               │
        COMPLETE             │          COMPLETE            │
                             │                              │
                      ┌──────┘                      ┌──────┘
                      │ (after LOOP_GUARD or        │
                      │  MAX_ROUNDS_REACHED)        │
                      ▼                             ▼
                 ORCHESTRATOR                   ORCHESTRATOR
                 ESCALATES                      ESCALATES
```

### Guard clauses (orchestrator)

| Guard | Trigger | Action |
|-------|---------|--------|
| `LOOP_GUARD` | Same finding fingerprint reappears after remediation pass | Stop, escalate to user |
| `MAX_ROUNDS_REACHED` | Build/validate rounds exhausted (configurable) | Stop, report exhaustion |
| `BUDGET_GUARD` | Token / cost budget exceeded | Stop, report budget status |

### Interruption recovery

- **Before plan creation**: re-delegate `migration-scope` — never guess the scope
- **Mid-build**: check latest plan `status` and file tree before re-delegating
- **Child plan half-built**: delegate `build-child-plans` again (idempotent)
- **Remediation in progress**: resume from open findings/tasks; verify partial fixes before adding more changes
- Store latest fingerprint in plan frontmatter `workflow` metadata when feasible

## 4. Delegation Chain

```
orchestrator ──task──▶ migration-scope
                         │
                         └──▶ returns SCOPE_SINGLE_PLAN
                         │    or SCOPE_SPLIT_RECOMMENDED
                         │    or AMBIGUOUS (needs user)

orchestrator ──task──▶ migration-planner
                         │
                         └──▶ returns PLAN_READY (with evidence graph)
                         │    or PLAN_BLOCKED (irreconcilable gap)
                         │    or EVIDENCE_GAP / GRAPH_CONFLICT
                         │    or AWAITING_INPUT (needs user)

orchestrator ──task──▶ @dev (build)
                         │
                         └──▶ returns BUILD_COMPLETE

orchestrator ──task──▶ migration-validator
                         │
                         └──▶ returns APPROVED (all 9 axes pass)
                         │    or REJECTED (axes fail, fixes listed)
                         │    or BLOCKED (missing evidence)

orchestrator ──task──▶ @dev (remediate)
                         │
                         └──▶ returns BUILD_COMPLETE

orchestrator ──task──▶ migration-validator-focused
                         │
                         └──▶ returns FOCUSED_APPROVED (remediation resolved)
                         │    or FOCUSED_REJECTED (remediation incomplete)
                         │    or ESCALATE (scope broader than expected)
```

### Sequential vs parallel

- **Sequential spine** (scope → plan → build → validate): use `task` (synchronous).
  The output is available inline and the session stays dirty with file writes.
- **Parallel fan-out** (independent child plans, research spikes): use
  `delegate` (asynchronous). Runs in an isolated session — file writes are NOT
  available in the orchestrator context. Collect handoffs and advance.

**Never poll `delegation_list`** to check completion — it wastes tokens. Wait
for `<task-notification>`.

## 5. State Transitions Table

| From | To | Trigger | Who |
|------|----|---------|-----|
| `missing` | `planned` | `PLAN_READY` | migration-planner |
| `planned` | `implemented` | `BUILD_COMPLETE` | @dev |
| `implemented` | `validated` | `APPROVED` (all axes pass) | migration-validator |
| `implemented` | `remediation-needed` | `REJECTED` (axes fail) | migration-validator |
| `remediation-needed` | `implemented` | `BUILD_COMPLETE` | @dev (remediate) |
| `remediation-needed` | `validated` | `FOCUSED_APPROVED` → full validator `APPROVED` | val-focused → validator |
| `validated` | `complete` | orchestrator marks tracker | migration-orchestrator |
| any | `blocked` | `BLOCKED` / `PLAN_BLOCKED` | planner or validator |
| any | `awaiting-input` | `AWAITING_INPUT` | planner or scope |
| `remediation-needed` | `remediation-needed` | `LOOP_GUARD` / `MAX_ROUNDS_REACHED` | orchestrator |

### Child plan transitions (parent plan flow)

| From | To | Trigger | Who |
|------|----|---------|-----|
| child `missing` | child `validated` | same flow as above, per child | scope → plan → @dev → validator |
| all children `validated` | parent `planned` | orchestrator gates parent plan creation | orchestrator |
| parent `implemented` | parent `validated` | `APPROVED` (parent axes pass) | migration-validator |

## 6. Terminal Markers Reference Table

| Marker | Agent | Meaning |
|--------|-------|---------|
| `SCOPE_SINGLE_PLAN` | migration-scope | Single migration plan, linear flow |
| `SCOPE_SPLIT_RECOMMENDED` | migration-scope | Parent + children work graph; fan-out recommended |
| `AMBIGUOUS` | migration-scope | Scope boundaries unclear; needs user clarification |
| `EVIDENCE_GAP` | scope, planner, validator | Missing row-level legacy source evidence |
| `GRAPH_CONFLICT` | scope, planner | Cyclic or contradictory dependency edges |
| `PLAN_READY` | migration-planner | Plan complete with evidence-backed dependency graph and task rows |
| `PLAN_BLOCKED` | migration-planner | Irreconcilable evidence gap — cannot produce a valid plan |
| `AWAITING_INPUT` | planner, scope | Targeted clarification question before continuing |
| `APPROVED` | migration-validator | All 9 parity axes pass with row-level evidence |
| `REJECTED` | migration-validator | One or more axes fail; mandatory fixes listed |
| `BLOCKED` | migration-validator | Missing evidence prevents validation |
| `FOCUSED_APPROVED` | val-focused | Remediation tasks resolved — ready for full re-validation |
| `FOCUSED_REJECTED` | val-focused | Remediation incomplete — back to @dev |
| `ESCALATE` | val-focused | Scope broader than expected — requires full validator |
| `LOOP_GUARD` | orchestrator | Same finding fingerprint after remediation — no progress |
| `MAX_ROUNDS_REACHED` | orchestrator | Build/validate rounds exhausted |
| `BUDGET_GUARD` | orchestrator | Token/cost budget exceeded |
| `COMPLETED` | orchestrator | Migration workflow finished — plan validated and tracker updated |

### Dependency Evidence States (scope, planner)

| State | Meaning |
|-------|---------|
| `VERIFIED` | Row-level source/test/render evidence confirmed |
| `PARTIAL` | Some evidence exists but gaps remain |
| `MISSING` | No evidence found — requires investigation or user input |
| `CONFLICT` | Contradictory evidence across sources |

## 7. Remediation Loop

```
                    ┌─────────────────────────────────────────┐
                    │                                         │
                    ▼                                         │
            ┌──────────────┐                                  │
            │   REJECTED   │  (validator returns REJECTED     │
            │              │   with mandatory fixes list)     │
            └──────┬───────┘                                  │
                   │                                          │
                   ▼                                          │
            ┌──────────────┐                                  │
            │  @dev fixes  │  (implements remediation tasks   │
            │              │   from validation findings)      │
            └──────┬───────┘                                  │
                   │                                          │
                   ▼                                          │
            ┌──────────────┐                                  │
            │ val-focused  │  (narrow check on specific       │
            │              │   remediation tasks only)        │
            └──────┬───────┘                                  │
                   │                                          │
         ┌─────────┴─────────┐                                │
         ▼                   ▼                                │
  FOCUSED_APPROVED    FOCUSED_REJECTED                        │
         │                   │                                │
         ▼                   └──────── back to @dev ──────────┘
  ┌──────────────┐
  │   FULL       │  (migration-validator checks all 9 axes)
  │  VALIDATE    │
  └──────┬───────┘
         │
   ┌─────┴─────┐
   ▼           ▼
APPROVED    REJECTED
   │           │
   ▼           └──────── back to @dev ────────────────────────┘
COMPLETE

    ESCALATE path:
    ┌──────────┐
    │ ESCALATE │  (val-focused detected scope broader
    └────┬─────┘   than expected remediation)
         │
         ▼
    ┌──────────────┐
    │ FULL VALIDATE │  (back to migration-validator)
    └──────────────┘
```

### Remediation guard

The orchestrator fingerprints each `REJECTED` finding set. If the same
fingerprint appears after a remediation pass (no observable progress),
the orchestrator stops with `LOOP_GUARD` and escalates to the user.

## 8. Kanban Flow

### Column progression

```
 ┌─────────┐    ┌─────────┐    ┌─────────────┐    ┌────────┐    ┌──────┐
 │ backlog │───▶│  ready  │───▶│ in_progress │───▶│ review │───▶│ done │
 └─────────┘    └─────────┘    └─────────────┘    └────────┘    └──────┘
      ▲               ▲                                  │
      │               │                                  │
      └─── changes ───┘                                  │
      (remediation                                      │
       requested)                                        │
                                                         ▼
                                              ┌──────────────┐
                                              │   blocked    │
                                              │ (BLOCKED /   │
                                              │  PLAN_       │
                                              │  BLOCKED)    │
                                              └──────────────┘
```

### Phase → Kanban mapping

| Phase | Agent Delegated | Column | Status |
|-------|----------------|--------|--------|
| Start | orchestrator creates card | `backlog` | `pending` |
| Scope | migration-scope | `in_progress` | `running` |
| Scope done | handoff returned | `review` | `review` |
| Scope accepted | orchestrator advances | `ready` → `in_progress` | `pending` → `running` |
| Plan | migration-planner | `in_progress` | `running` |
| Plan done | handoff returned | `review` | `review` |
| Plan accepted | orchestrator advances | `ready` → `in_progress` | `pending` → `running` |
| Build | @dev | `in_progress` | `running` |
| Build done | `BUILD_COMPLETE` | `review` | `review` |
| Validate | migration-validator | `in_progress` | `running` |
| `APPROVED` | validator returns | `done` | `done` |
| `REJECTED` | validator returns | `backlog` | `changes` |
| Remediate | @dev (fix findings) | `in_progress` | `running` |
| Val-focused | val-focused | `in_progress` | `running` |
| `FOCUSED_APPROVED` | val-focused returns | `review` | `review` |
| Full re-validate | migration-validator | `in_progress` | `running` |
| `BLOCKED` | any phase | `blocked` | `blocked` |

### Orchestrator Kanban commands

| Event | Command |
|-------|--------|
| New delegation created | `kanban_create_delegation(agent, task_summary)` |
| Phase started | `kanban_update_delegation(id, column="in_progress", status="running")` |
| Progress / activity logged | `kanban_add_activity(delegation_id=<id>, type="progress", content="<update>")` |
| Handoff received (pass) | `kanban_add_activity(delegation_id=<id>, type="progress", content="<handoff>")` then `kanban_update_delegation(id, handoff_preview="<summary>", column="review", status="review")` |
| Handoff rejected (changes) | `kanban_update_delegation(id, column="backlog", status="changes")` |
| Phase blocked / needs decision | `kanban_add_activity(delegation_id=<id>, type="blocked", content="<reason>")` then `kanban_update_delegation(id, status="blocked", blocker="<reason>")` |
| Decision resolved | `kanban_resolve_activity(delegation_id=<id>, activity_id=<actId>, resolution="<outcome>")` then `kanban_update_delegation(id, status="pending", column="ready")` |
| Work complete | `kanban_update_delegation(id, column="done", status="done")` |

### Status mapping reference

| Status | Column | Meaning |
|--------|--------|---------|
| `pending` | `backlog` / `ready` | Not started |
| `running` | `in_progress` | Active delegation |
| `review` | `review` | Waiting for orchestrator to consume handoff |
| `changes` | `backlog` | Remediation requested |
| `blocked` | `blocked` | Needs user decision |
| `done` | `done` | Completed |

## 9. Evidence-First Gates (Rules from the Skill)

### What counts as evidence

- **Row-level**: specific legacy source file:line references for each API
  route, component state, data row, and event handler
- **Source**: actual legacy code (not documentation, not assumptions based on names)
- **Test**: existing test coverage or new tests that exercise the parity point
- **Render**: visual or DOM output comparison (legacy vs Angular)

### What does NOT count as evidence

- Matching entity/component/enum names between legacy and new code
- File existence alone (e.g. "the service file exists")
- Tracker status alone
- Generic "build passes" evidence
- Examples from skill documentation taken as proof

### Evidence lifecycle per dependency

1. **Scope**: classify each dependency as `VERIFIED`, `PARTIAL`, `MISSING`, or `CONFLICT`
2. **Plan**: only mark a dependency `ready` when row-level source/test/render evidence is confirmed
3. **Validate**: every finding must cite legacy source file:line — no blanket evidence
4. **Remediate**: fix the specific evidence gap; do not widen the scope

## 10. Plan File Conventions

- **Location**: `Yoizen.Legacy/migrations/plans/<legacy-page-slug>.md`
- **Tracker**: `Yoizen.Legacy/migration-progress-tracker.md`
- **Plan is the contract**: all validation findings, remediation tasks,
  resolution logs, and evidence logs live inside the plan file. Do NOT create
  standalone files (`evidence-v001.md`, `validation-round-1.md`,
  `handoff-remediation.md`).
- **Plan retention**: validated plans are retained as the parity contract,
  validation record, and audit trail. Ask the user before adopting an archive
  strategy.

### Required plan frontmatter

| Field | Purpose |
|-------|---------|
| `status` | Current state: `missing`, `planned`, `implemented`, `validated`, `remediation-needed` |
| `planType` | `single` or `parent` or `child` |
| `dependencyGraph` | Evidence state per dependency: `VERIFIED` / `PARTIAL` / `MISSING` / `CONFLICT` |
| `estimatedTaskCount` | Number of task rows |
| `totalRowCount` | Total row-level evidence points |
| `validationRunId` | e.g. `VR-YYYYMMDD-XX` (set by validator) |
| `updatedAt` | ISO timestamp of last state change |

## 11. Validation Axes (Full Validator)

The migration-validator checks all nine axes with legacy source evidence:

| # | Axis | What it checks |
|---|------|----------------|
| 1 | Behavior | Functional parity — same inputs produce same outputs |
| 2 | Data | Row-level data parity across API responses and state |
| 3 | Routing | URI, query parameters, route guards match legacy |
| 4 | Rendering | Visual output parity — DOM structure and content |
| 5 | State | Component and application state transitions match |
| 6 | Styling | Visual appearance parity (layout, colors, typography) |
| 7 | Security | Auth, authorization, CSRF, XSS parity |
| 8 | Accessibility | ARIA, keyboard nav, screen-reader parity |
| 9 | Performance | Load time, render time, bundle size within acceptable delta |

### Validation output format

```
Gate: APPROVED | REJECTED | BLOCKED
Validation run: VR-YYYYMMDD-XX
Axes checked: 9
Findings:
  - [axis]: PASS / FAIL — <evidence citation: legacy file:line>
  - ...
Mandatory fixes: <list if REJECTED, each with specific remediation task>
Next suggested: migration-orchestrator (continue flow or remediation loop)
```

## 12. Agent Guardrails Summary

| Agent | ✅ Does | ❌ Does NOT |
|-------|--------|-------------|
| orchestrator | Read handoffs, advance phases, update tracker and Kanban | Write application code, run build/deploy, classify scope, create plans, validate parity |
| migration-scope | Classify surface, audit dependency evidence, recommend splits | Create plans, implement code, validate parity, infer from names |
| migration-planner | Create evidence-backed plans with task rows, flag gaps/conflicts | Implement code, classify scope, validate parity, create plans without evidence |
| migration-validator | Check all 9 axes with evidence, produce gate decisions, list fixes | Modify source code, remediate findings, approve without row-level evidence, skip axes |
| val-focused | Narrow check on specific remediation tasks, escalate if scope expands | Validate entire page, set parent to validated, expand scope, modify code, remediate |
| @dev | Implement task rows, remediate findings | Classify scope, create plans, validate parity, self-approve |
