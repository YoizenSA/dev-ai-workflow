# Orchestrator Execution Modes — Design Idea

**Status**: Implemented (MVP in agents)  
**Author**: Architect  
**Date**: 2026-08-05  
**Related**: `ywai/agents/core/orchestrator/`, `ywai/internal/config/orchestrator_profiles.json`, omp harness comparison

## Implementation notes (shipped)

Landed in-repo (re-run `ywai install --overwrite` / agent install to refresh OpenCode agents):

| Item | Where |
|------|--------|
| Modes `solo` \| `thin` \| `full` + triage invert (unsure → thin) | `agents/core/orchestrator/AGENT.md` |
| Risk policy separate from mode | same |
| Solo-capable permissions (write/search) | `agents/core/orchestrator/permissions.json` |
| Compact contracts + JSON handoff | `agents/sections/{orchestrator-contracts,handoff,handoff-qa,context-gathering}.md` |
| Single scout `@finder` (QA scout mode in brief) | `agents/core/finder/AGENT.md`; `qa-finder` removed |
| QA group wires to `@finder` | `groups.json`, `delegations.json`, qa-automation agents, workflow |
| Model profiles unchanged (`balanced` kept as model map only) | `orchestrator_profiles.json` |

Still open / not in this MVP: dormant-rules plugin, separate `execution_policy` config schema, hard metrics harness, prompt-cache TUI mode badge, deleting leftover `~/.config/opencode/agents/qa-finder.md` on install.

## 1. Problem

Same provider + model feels faster in [oh-my-pi (omp)](https://github.com/can1357/oh-my-pi) than in OpenCode with our CORE agents.

Root causes we control:

| Layer | Effect |
|-------|--------|
| OpenCode harness (`str_replace`, shell tools, no hashline) | More edit retries and output tokens |
| Global prompt stack (gentle-ai, SDD, large system context) | Higher TTFT and cost per turn |
| Multi-agent protocol (SCOUT → PLAN → DEV → QA → REVIEW) | Latency × N subagents even for small work |
| Orchestrator always-heavy posture | Setup cost paid even when the task is trivial |

Our CORE agent prompts are relatively small. The expensive part is **always coordinating** and **always loading ceremony**.

### Current gap

The orchestrator already has a triage table (`trivial` vs `goal`), but:

1. **Trivial still delegates** to `@dev` / `@ask` — it is still multi-agent (extra hop).
2. Orchestrator **cannot act alone**: `edit`/`write`/`grep`/`codegraph` are denied; prompt says “never write code.”
3. **Unsure → default `goal`**, which forces the full delivery pipeline.
4. **Orchestrator profiles** (`fast` / `balanced` / `deep`) only assign **models**, not **behavior**.

Result: even “simple fix” under the orchestrator pays coordinator overhead.

We cannot clone omp’s Rust harness (hashline, in-process tools, model-tuned edit formats) inside OpenCode. We *can* make the orchestrator **behave like a single agent when multi-agent is unnecessary**, and wire that to profiles.

## 2. Goal

One orchestrator agent that:

- Acts as a **solo coding agent** when the task is small.
- Uses a **thin path** (0–1 hops, no SCOUT/PLAN/REVIEW) when the task is clear but slightly larger.
- Runs the **full delivery pipeline** only when multi-phase / multi-agent / ship-grade work justifies it.

Profiles control not only *which model* each role uses, but *how aggressively* the orchestrator orchestrates.

## 3. Core idea: execution modes

Three modes on the **same** agent (not three separate agents):

| Mode | When | Behavior |
|------|------|----------|
| **solo** | One question, one file, small fix, no design→impl→test→review chain | Act as a single agent: read, edit, verify. **Zero** `task` / `delegate`. |
| **thin** | Clear “do X”, limited scope, no design/ship ceremony | Prefer doing it yourself; **at most one** short hop to `@dev`/`@qa` if needed. No SCOUT / PLAN / REVIEW. |
| **full** | Multi-phase, multi-file with ordering deps, UI design, ship | Current coordinator: scout → plan → (design) → impl/test → review → close. **No direct edits.** |

### Escape hatch

```
solo  → (scope blows up: many files, new APIs, design needed) → escalate to thin or full
full  → does not silently drop to solo mid-goal without saying so
```

If in `solo` the work is about to touch many files or needs architecture, **stop and escalate** — do not invent a hidden mini-pipeline.

### Visible status (every turn start)

```
mode: solo · reason: single-file fix
```

User can override in natural language: “full pipeline”, “solo”, “just fix it”, “orchestrate”.

## 4. Profiles: models + orchestration policy

Today (`orchestrator_profiles.json`):

- `fast` / `balanced` / `deep` = per-agent model map only.

Proposed extension: profile = **models + orchestration policy**.

| Profile | Bias | Default mode | Solo write | Review |
|---------|------|--------------|------------|--------|
| **fast** | Speed / cost | `solo` | allowed | never (unless user asks ship/review) |
| **balanced** | Default sane | `thin` | allowed | on ship / when user asks |
| **deep** | Quality | `full` | denied (pure coordinator) | always (or on ship, stricter than balanced) |

Optional later: a dedicated **`omp` / `lean`** profile = fast + solo-first + minimal handoffs + write-capable orchestrator. Not required for MVP if `fast` absorbs that bias.

### Suggested schema fields

```yaml
# under each orchestrator profile (seed + user override)
orchestration:
  default_mode: solo | thin | full
  allow_solo_write: true | false
  max_hops_before_escalate: 0 | 1 | 2
  require_review: never | on_ship | always
  escalate_on:
    - multi_file_deps
    - ui_design
    - ship
    - user_says_orchestrate
```

Shipped defaults (sketch):

| Profile | `default_mode` | `allow_solo_write` | `max_hops` | `require_review` |
|---------|----------------|--------------------|------------|------------------|
| fast | solo | true | 0–1 | never |
| balanced | thin | true | 1 | on_ship |
| deep | full | false | 2 (existing retry budget) | always |

Activation remains the existing `active_orchestrator_profile` mechanism: changing profile rewrites agent markdown models **and** (once implemented) installs the matching orchestration policy into the orchestrator prompt/permissions.

## 5. Permissions strategy

Prompt-only solo mode fails if tools stay denied: the model tries to edit, host rejects, retry loop — **worse** than today.

| Option | Description | Tradeoff |
|--------|-------------|----------|
| **A — Profile-driven perms (recommended)** | `fast`/`balanced`: orchestrator install gets read/edit/grep/bash (and related) allow. `deep`: keep current deny + pure coordinator. | One agent; profile owns persona. Clear install story. |
| **B — Always write-capable** | Orchestrator always has write tools; mode is prompt discipline only. | Faster to ship; easier for deep to “cheat” and edit. |
| **C — Two agents** | `orchestrator` (flexible) + `orchestrator-strict` (coordinator only). | User must know which to open; more surface. |

**Recommendation: A.** Aligns with “we already have profiles.”

### Permissions sketch by mode/profile

| Capability | solo / thin (fast, balanced) | full (deep, or after escalate) |
|------------|------------------------------|--------------------------------|
| read, grep, glob, codegraph | allow | deny or unused (delegate) |
| edit, write | allow | deny |
| bash | allow (impl + verify) | verify-only allowlist (as today) |
| task, delegate | available but **must not use** in solo; thin at most one hop | primary tools |
| question, todowrite, skill | allow | allow |

Note: OpenCode may not hot-swap permissions mid-session. Practical approach: **install-time permissions follow profile**; mode `full` under a write-capable install relies on prompt “do not edit in full mode.” Deep profile keeps hard deny for purity.

## 6. Triage rewrite

Replace binary `trivial` / `goal` with mode selection.

| Request shape | Mode |
|---------------|------|
| Explain / compare / research, no code change | solo (or thin → single `@ask` only if preferred) |
| One file, small fix, single test, typo | **solo** |
| Clear implement X, few files, no design/ship | **thin** |
| Multi-phase, multi-agent, multi-file deps, UI design, ship | **full** |
| Unsure | **thin** (or solo under `fast`) — **not** full |

**Invert the current default:** unsure must not default to full pipeline.

### Keyword overrides (omp-style, prose only)

| User signal | Force |
|-------------|--------|
| `solo`, `rápido`, `just fix`, `quick` | solo |
| `orchestrate`, `ship`, `full pipeline`, `coordina` | full |
| none | profile `default_mode` + table above |

## 7. Protocol changes by mode

| Concern | solo | thin | full |
|---------|------|------|------|
| `todowrite` | optional / skip | light checklist optional | mandatory phase checklist |
| SCOUT `@finder` | no | no | yes, one bounded |
| PLAN `@architect` | no | no | yes |
| DESIGN `@designer` | no | no | when UI |
| TDD question gate | no | no | yes (as today) |
| Handoff fences / contracts | no | no (or minimal if one hop) | yes |
| `@reviewer` | no | no unless user asks | per `require_review` |
| Close summary | short done note | short | full delivery summary |

## 8. Non-goals

- Reimplementing omp’s hashline, in-process ripgrep/shell, or Rust natives.
- Making multi-agent as fast as single-agent for the same work (physics: more hops cost).
- Three user-facing agents (`orchestrator-fast`, …) — **one agent, profile decides**.
- Silent full pipelines inside solo mode.

## 9. MVP path

1. **Design freeze** this document (modes + profile fields + triage invert).
2. Extend profile seed schema with `orchestration.*` (defaults for fast/balanced/deep).
3. Rewrite orchestrator `AGENT.md` triage for `solo | thin | full` and unsure → non-full.
4. Profile-driven install: `allow_solo_write` flips orchestrator `permissions.json` (or generated OpenCode frontmatter).
5. Skip handoff/contracts injection when mode is solo/thin (or when profile is fast).
6. Docs: one table “when the orchestrator stays solo vs escalates.”
7. Optional later: Settings UI knob “Orchestration bias” + `omp`/`lean` named profile.

### Success criteria

Same small task (one-file fix), same model, active profile `fast`:

- **One session**, **0 subagents**, system work limited to orchestrator tools.
- Status line shows `mode: solo`.
- Perceived latency closer to a direct `@dev` / omp-style session.

Multi-phase goals under `deep` or forced full: existing pipeline quality preserved, with short handoffs preferred.

## 10. Risks

| Risk | Mitigation |
|------|------------|
| Solo mode under-scopes a large task | Explicit escalate rule; max files / “new API” triggers |
| Write-capable orchestrator ignores full-mode discipline | `deep` keeps deny; full mode restates no-edit; evals later |
| Profile schema drift vs installed agent markdown | Same install path that already writes models |
| Users stay on `balanced`/`deep` and see no speedup | Document that **fast + solo** is the speed path; consider default profile discussion separately |

## 11. Open questions

1. Should `balanced` default to `thin` or `solo`?
2. Under `fast` solo, is git commit allowed to the orchestrator, or still review-then-commit / user-owned?
3. Do we inject mode policy as a **generated section** at install time (from profile JSON) or keep a single static AGENT.md with “read active profile” language?
4. Should thin’s single hop prefer sync `task` only (never async `delegate`) to preserve undo?

## 12. Summary

**Do not clone omp.** Make the orchestrator **mode-aware** and make **profiles own orchestration policy**, not only models.

- **solo** = single agent when multi-agent is waste.
- **thin** = default sane middle for balanced.
- **full** = current technical-lead pipeline when the goal earns it.
- **fast / balanced / deep** gain a real behavioral meaning, not just cheaper or stronger weights.
