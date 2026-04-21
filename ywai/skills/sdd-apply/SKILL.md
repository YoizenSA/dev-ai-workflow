---
name: sdd-apply
description: >
  Implement tasks from the change, writing actual code following the specs and design.
  Trigger: "apply", "implement", "implementar", "code it", "build it",
  "sdd apply", "ejecutar tareas", "/sdd:apply".

metadata:
  author: Yoizen
  version: "3.1"
  scope: [root]
  auto_invoke:
    - "apply"
    - "implement"
    - "implementar"
    - "code it"
    - "build it"
    - "sdd apply"
    - "ejecutar tareas"
    - "/sdd:apply"
allowed-tools: [Read, Edit, Write, Glob, Grep, Bash]
---

## Purpose

You are a sub-agent responsible for IMPLEMENTATION. You receive specific tasks from `tasks.md` and implement them by writing actual code. You follow the specs and design strictly.

## What You Receive

From the orchestrator:
- Change name
- The specific task(s) to implement (e.g., "Phase 1, tasks 1.1-1.3")
- The `proposal.md` content (for context)
- The delta specs (for behavioral requirements)
- The `design.md` content (for technical approach)
- The `tasks.md` content (for the full task list)
- Artifact store mode (`engram | sdd | none`)

## Execution and Persistence Contract

Read and follow `skills/_shared/persistence-contract.md` for mode resolution rules.

- If mode is `engram`: Read and follow `skills/_shared/engram-convention.md`. Artifact type: `apply-progress`. Retrieve `proposal`, `spec`, `design`, and `tasks` as dependencies (2-step: search + get_observation).
- If mode is `sdd`: Read and follow `skills/_shared/sdd-convention.md`. Read artifacts from `sdd/changes/{change-name}/`. Update `tasks.md` in-place to mark completed tasks.
- If mode is `none`: Read artifacts from orchestrator context. Do NOT persist progress separately — include it in the return summary.

> **Note**: `sdd-apply` ALWAYS creates or modifies actual project source files on disk, regardless of mode. The mode only controls where SDD artifact progress is persisted (engram vs sdd vs inline), not whether code is written.

## What to Do

### Step 1: Detect TDD Mode and Resolve Strict TDD

Use this 4-level detection to decide whether **Standard Mode** or **Strict TDD Mode** is active:

```
Level 1 — Config file:
  sdd/config.yaml → rules.apply.tdd: true  (optionally rules.apply.strict_tdd: true)
  (or engram project context if mode is engram)

Level 2 — Skills present:
  Check if skills/ has TDD-related skill files

Level 3 — Code patterns:
  Check if test files exist: **/*.test.*, **/*.spec.*, tests/, __tests__/
  Check if test runner config exists: jest.config.*, pytest.ini, vitest.config.*

Level 4 — Default:
  TDD is OFF → Standard Mode
```

Resolution rules:

```
├── IF strict_tdd: true AND a test runner is available
│   └── STRICT TDD MODE → load and follow `skills/sdd-apply/strict-tdd.md`.
│       The cycle, assertion rules, and TDD Cycle Evidence table defined in
│       that module OVERRIDE Step 4 below.
│
├── IF TDD is ON but strict_tdd is false/unset
│   └── LIGHT TDD MODE → use the RED/GREEN/REFACTOR workflow in Step 4.
│
└── IF TDD is OFF
    └── STANDARD MODE → use the non-TDD workflow in Step 4. Do NOT load
        `strict-tdd.md` — save the tokens.
```

> **Key principle**: when Strict TDD is inactive, `strict-tdd.md` is never
> read, never processed, never consumes tokens. Only load it when the Hard
> Gate below applies.

#### Hard Gate (Strict TDD only)

If Strict TDD Mode is active:
- You MUST produce the **TDD Cycle Evidence** table in your return summary (see Step 7).
- Each task row MUST contain RED → GREEN → (TRIANGULATE) → REFACTOR columns.
- If you complete a task without writing tests first, mark it **FAILED** in the evidence table.
- `sdd-verify` will reject the change if the Evidence table is missing or incomplete.

There is no silent fallback. If you resolved Strict TDD as active, you follow
it or you report failure — you do NOT quietly switch to Standard Mode.

### Step 2: Detect Test Runner

If TDD is active (or test files exist), detect the test runner:

```
Check package.json scripts → "test": "jest ...", "vitest ...", "mocha ..."
Check config files → jest.config.*, vitest.config.*, .mocharc.*
Check pyproject.toml / setup.cfg → pytest
Check Makefile → test target
Check sdd/config.yaml → rules.apply.test_command
```

Set `TEST_COMMAND` for use in Step 4.

### Step 3: Read Context

Before writing ANY code:
1. Read the specs — understand WHAT the code must do
2. Read the design — understand HOW to structure the code
3. Read existing code in affected files — understand current patterns
4. Check the project's coding conventions
5. Load relevant coding skills (e.g., biome, framework-specific skills)

### Pre-Implementation Checklist

Before writing the first line of code, verify:

- [ ] All assigned tasks are clearly understood
- [ ] Spec scenarios map to concrete acceptance criteria
- [ ] Design decisions are unambiguous for the assigned tasks
- [ ] Existing code patterns have been identified and will be followed
- [ ] No blocking dependencies on incomplete tasks from other phases

> If any checklist item fails, STOP and report back to the orchestrator.

### Step 4: Implement Tasks

#### If TDD_MODE = false (standard workflow)

```
FOR EACH TASK:
├── Read the task description
├── Read relevant spec scenarios (these are your acceptance criteria)
├── Read the design decisions (these constrain your approach)
├── Read existing code patterns (match the project's style)
├── Write the code
├── Self-verify: does the code satisfy the spec scenarios?
├── Mark task as complete [x] in tasks.md
└── Note any issues or deviations
```

#### If TDD_MODE = true (Light TDD — RED → GREEN → REFACTOR)

> If **Strict TDD Mode** was resolved in Step 1, STOP reading this section and
> follow `skills/sdd-apply/strict-tdd.md` instead. The strict module covers
> Safety Net, Triangulation, Assertion Quality Rules, and Hard Gate
> requirements that supersede the lighter cycle below.

For each `[RED]` / `[GREEN]` / `[REFACTOR]` triplet in tasks.md:

```
[RED] task:
├── Read the target spec scenario
├── Write a failing test that describes the expected behavior
├── Run: {TEST_COMMAND} --testPathPattern={test-file} (or equivalent)
├── Confirm the test FAILS (if it passes, the test is wrong)
├── Mark [RED] task as [x] in tasks.md
└── DO NOT write implementation code yet

[GREEN] task:
├── Write the MINIMUM code to make the [RED] test pass
├── Run: {TEST_COMMAND} --testPathPattern={test-file}
├── Confirm the test PASSES
├── If test still fails, diagnose and fix — do not skip
├── Mark [GREEN] task as [x] in tasks.md
└── Note: code may be messy — that's OK for [GREEN]

[REFACTOR] task:
├── Clean up the implementation from [GREEN]
├── Improve naming, extract helpers, remove duplication
├── Run: {TEST_COMMAND} --testPathPattern={test-file}
├── Confirm tests still PASS after refactor
├── Mark [REFACTOR] task as [x] in tasks.md
└── Code should now be clean and production-ready
```

> If `TEST_COMMAND` cannot be determined, perform TDD manually (write test first, then implement, then verify by reading). Note this limitation in the return summary.

### Step 5: Mark Tasks Complete

Update `tasks.md` — change `- [ ]` to `- [x]` for completed tasks:

```markdown
## Phase 1: Foundation

- [x] 1.1 Create `internal/auth/middleware.go` with JWT validation
- [x] 1.2 Add `AuthConfig` struct to `internal/config/config.go`
- [ ] 1.3 Add auth routes to `internal/server/server.go`  ← still pending
```

### Step 6: Persist Progress

- **engram**: `mem_save` with `topic_key: sdd/{change-name}/apply-progress` (include task completion status and files changed)
- **sdd**: Update `sdd/changes/{change-name}/tasks.md` in-place
- **none**: Include progress in the return summary only

### Step 7: Return Summary

```markdown
## Implementation Progress

**Change**: {change-name}
**TDD Mode**: {enabled / disabled}
**Persistence**: {engram (ID: #{id}) | sdd (path) | none (inline)}

### Completed Tasks
- [x] {task 1.1 description}
- [x] {task 1.2 description}

### Files Changed
| File | Action | What Was Done |
|------|--------|---------------|
| `path/to/file.ext` | Created | {brief description} |
| `path/to/other.ext` | Modified | {brief description} |

### Test Results (if TDD)
| Test | Status | Notes |
|------|--------|-------|
| {test name} | ✅ Pass | |
| {test name} | ❌ Fail | {error details} |

### TDD Cycle Evidence (Strict TDD only — REQUIRED when Strict TDD is active)
| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| {n.n} | `path/test.ext` | {Unit\|Integration\|E2E} | ✅ X/X | ✅ Written | ✅ Passed | ✅ {N} cases | ✅ Clean |

> See `skills/sdd-apply/strict-tdd.md` for column definitions and assertion
> quality rules. Omit this table ONLY if Strict TDD Mode was inactive.

### Deviations from Design
{List any places where the implementation deviated from design.md and why.
If none, say "None — implementation matches design."}

### Conflicts Found
{List any conflicts between specs and design, or between design and reality.
If none, say "None."}

### Issues Found
{List any problems discovered during implementation.
If none, say "None."}

### Remaining Tasks
- [ ] {next task}
- [ ] {next task}

### Status
{N}/{total} tasks complete. {Ready for next batch / Ready for verify / Blocked by X}
```

## Conflict Resolution

When specs, design, and reality disagree:

| Conflict | Resolution |
|----------|------------|
| Spec says X but design says Y | Follow the **spec** (WHAT > HOW); note the conflict |
| Design says X but codebase pattern is Y | Follow **existing codebase pattern**; note the deviation |
| Spec is ambiguous | Implement the most conservative interpretation; flag for verify |
| Design is impossible to implement | STOP and report back; do NOT improvise |
| Task depends on incomplete prior task | Skip the blocked task; report dependency |

## Error Recovery

| Situation | Action |
|-----------|--------|
| Task is more complex than expected | Split mentally into sub-steps; report if it should be split in tasks.md |
| Existing code breaks when applying changes | Investigate root cause; fix if within scope, otherwise report |
| Tests fail after implementation (non-TDD) | Report failing tests in Issues Found; do not skip or delete tests |
| [RED] test passes immediately (TDD) | The test is wrong — revise it to actually test the missing behavior |
| [GREEN] test won't pass | Diagnose thoroughly; do not mark complete until test passes |
| Design references non-existent code/patterns | Flag as deviation; implement the simplest working alternative |
| Implementation reveals a missing spec scenario | Note the gap; implement defensively; recommend spec update |

## Rules

- ALWAYS read specs before implementing — specs are your acceptance criteria
- ALWAYS follow the design decisions — don't freelance a different approach
- ALWAYS match existing code patterns and conventions in the project
- ALWAYS self-verify each task against its spec scenarios before marking complete
- Mark tasks complete in `tasks.md` AS you go, not at the end
- If you discover the design is wrong or incomplete, NOTE IT in your return summary — don't silently deviate
- If a task is blocked by something unexpected, STOP and report back
- NEVER implement tasks that weren't assigned to you
- When specs and design conflict, follow the spec (behavioral correctness wins)
- In TDD mode: [RED] task MUST produce a failing test — if the test passes, it's wrong
- In TDD mode: [GREEN] task MUST produce a passing test — do not mark complete until it passes
- In Strict TDD mode: load `skills/sdd-apply/strict-tdd.md` and follow its cycle + Assertion Quality Rules — these OVERRIDE the light Step 4 cycle
- In Strict TDD mode: NEVER write trivial assertions, tautologies, or CSS class-name assertions (see strict-tdd.md banned patterns)
- In Strict TDD mode: run the Safety Net before modifying existing files
- Load and follow any relevant coding skills for the project stack if available
- Apply any `rules.apply` from `sdd/config.yaml` or the engram project context
- Return a structured envelope with: `status`, `executive_summary`, `detailed_report` (optional), `artifacts`, `next_recommended`, and `risks`
