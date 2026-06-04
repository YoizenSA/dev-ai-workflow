# ywai Agents Workflow

Visual guide to how the orchestrator delegates work across the specialist subagents.

## Overview

The `orchestrator` is the technical lead. It owns the goal, delegates to specialists, collects handoffs, and decides the next step.

```mermaid
graph LR
    U[User] -->|goal| O[orchestrator]
    O --> A[architect]
    O --> D[dev]
    O --> Q[qa]
    O --> R[reviewer]
    O --> DO[devops]
    O --> U
```

## 1. Planning Phase

The orchestrator delegates design/planning to the architect, which produces a **Product plan** (what/why) and a **Technical plan** (how).

```mermaid
graph LR
    U[User] -->|goal| O[orchestrator]
    O -->|PLAN| A[architect]
    A -->|handoff| O
    O -->|next: IMPLEMENT| D[dev]
```

**Architect handoff includes:**
- Product plan: problem, goal, scope, user stories, acceptance criteria
- Technical plan: approach, components, work breakdown, test strategy, rollout

## 2. TDD Branch

The orchestrator asks the user whether to use TDD. This branches the flow.

### TDD = yes (tests first)

```mermaid
graph LR
    O[orchestrator] -->|¿TDD? yes| Q1[qa: write failing tests]
    Q1 -->|handoff| O
    O -->|IMPLEMENT| D[dev: make tests pass]
    D -->|handoff| O
    O -->|VALIDATE| Q2[qa: run + extend coverage]
    Q2 -->|handoff| O
    O -->|next: REVIEW| R[reviewer]
```

### TDD = no (tests after)

```mermaid
graph LR
    O[orchestrator] -->|¿TDD? no| D[dev: implement feature]
    D -->|handoff| O
    O -->|TEST| Q[qa: add tests after]
    Q -->|handoff| O
    O -->|next: REVIEW| R[reviewer]
```

## 3. Fan-out: Parallel Delegation

When work splits into independent streams (e.g. API + frontend + DB migration), the orchestrator can spawn multiple `@dev` in parallel.

```mermaid
graph LR
    O[orchestrator] -->|delegate| D1[dev: API endpoint]
    O -->|delegate| D2[dev: frontend form]
    O -->|delegate| D3[dev: DB migration]
    D1 -->|handoff| O
    D2 -->|handoff| O
    D3 -->|handoff| O
    O -->|integrate| D4[dev: wire together]
    D4 -->|handoff| O
    O -->|next: REVIEW| R[reviewer]
```

**Guardrails:**
- Never run parallel delegations that write the same files.
- Assign disjoint scopes in each brief's `Constraints`.
- Join handoffs, resolve conflicts, then integrate.

## 4. Review Cycle

The reviewer approves or requests changes. If changes are requested, the cycle repeats.

```mermaid
graph LR
    O[orchestrator] -->|REVIEW| R[reviewer]
    R -->|approve| O
    R -->|request changes| D[dev: fix]
    D -->|handoff| R
    R -->|approve| O
    O -->|next: DEPLOY?| DO[devops]
```

## 5. Deploy (optional)

For features that need CI/CD, containers, or deployment, the orchestrator delegates to devops.

```mermaid
graph LR
    O[orchestrator] -->|DEPLOY?| DO[devops]
    DO -->|handoff| O
    O -->|CLOSE| U[summary]
```

## 6. Close

The orchestrator summarizes delivered work, artifacts, and follow-ups.

```mermaid
graph LR
    O[orchestrator] -->|CLOSE| U[User]
```

## Handoff Format

Every subagent ends with a standard handoff:

```
**Status**: done | blocked | needs-decision
**Did**: <summary>
**Artifacts**: <files / tests / ADR / etc>
**Next suggested**: @dev | @qa | @reviewer | @devops | close
**Notes/risks**: <...>
```

## Sub-agent-statusline Plugin

The `sub-agent-statusline` plugin (installed automatically with `ywai install`) gives real-time visibility into running/completed/failed subagents, elapsed time, and token/context usage.

```mermaid
graph LR
    O[orchestrator] -->|delegate| D[dev]
    SL[sub-agent-statusline] -.->|visibility| O
    SL -.->|visibility| D
```

**Key features:**
- Sidebar shows running/completed/failed subagents
- Footer summary when there's activity
- Keyboard navigation: Alt+B to focus sidebar
