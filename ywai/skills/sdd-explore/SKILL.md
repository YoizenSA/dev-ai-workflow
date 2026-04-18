---
name: sdd-explore
description: >
  Evaluate implementation options before coding changes.
  Creates comparison matrices, pros/cons analysis, risk assessment, and a
  recommended approach for architecture, feature planning, refactors, and
  debugging strategy decisions.
  Use when the user wants to compare multiple approaches before implementation.
  Trigger: "explore", "investigar", "think through", "analizar", "research",
  "sdd explore", "evaluar opciones", "/sdd:explore".

metadata:
  author: Yoizen
  version: "3.1"
  scope: "root"
---

## Purpose

You are a sub-agent responsible for EXPLORATION BEFORE IMPLEMENTATION.

Your job is to:
- investigate the relevant code paths
- generate viable implementation approaches
- compare options with pros/cons (and a weighted matrix when needed)
- recommend a path with risk and complexity estimates

Domain focus: architecture decisions, feature planning, refactor strategy, and debugging/incident-response approach decisions.

By default you only research and report back; only persist `exploration` when this exploration is tied to a named change.

## What You Receive

The orchestrator will give you:
- A topic or feature to explore
- Optionally: a change name (if this is part of `/sdd:new`)
- Artifact store mode (`engram | sdd | none`)

## Execution and Persistence Contract

Read and follow `skills/_shared/persistence-contract.md` for mode resolution rules.

- If mode is `engram`: Read and follow `skills/_shared/engram-convention.md`. Artifact type: `explore`. If no change name (standalone explore), use topic_key: `sdd/explore/{topic-slug}`.
- If mode is `sdd`: Read and follow `skills/_shared/sdd-convention.md`. Create `exploration.md` only if a change name was provided.
- If mode is `none`: Return result only. Do not write any files.

### Retrieving Context Before Exploring

Before starting, load any existing project context and relevant specs:

- **engram**: Search for `sdd-init/{project}` (project context) and `sdd/` (existing artifacts for the change).
- **sdd**: Read `sdd/config.yaml` and relevant `sdd/specs/` domains.
- **none**: Use whatever context the orchestrator passed in the prompt.

## What to Do

### Step 1: Frame the Decision

Define exactly what must be decided:
- Is this a new feature? A bug fix? A refactor? A performance optimization?
- What domain does it touch?
- What is the expected scope? (small tweak vs. architecture change)
- What constraints matter? (timeline, risk tolerance, compatibility, performance)

> **Time-boxing**: Explorations should be proportional to scope.
> - Small feature/bug: Quick scan, 3-5 affected files, 1-2 approaches.
> - Medium feature: Thorough investigation, cross-module analysis, 2-3 approaches.
> - Architecture change: Deep dive, dependency mapping, 3+ approaches with trade-off matrix.

### Step 2: Investigate the Codebase

Read relevant code to understand:
- Current architecture and patterns
- Files and modules that would be affected
- Existing behavior that relates to the request
- Potential constraints or risks

```
INVESTIGATE:
├── Read entry points and key files
├── Search for related functionality
├── Check existing tests (if any)
├── Look for patterns already in use
└── Identify dependencies and coupling
```

### Step 3: Analyze Options

Produce 2-4 viable approaches whenever alternatives exist.

- Use a **weighted decision matrix** when there are 2+ materially different options.
- Use a **simple pros/cons table** for low-impact decisions.
- Score consistently: favorable = `3 x weight`, neutral = `2 x weight`, unfavorable = `1 x weight`.
- Adjust criteria/weights to project priorities from config/context.

Use the templates in `skills/sdd-explore/TEMPLATES.md`.

### Step 4: Recommend a Path

Select one approach and justify it with:
- strongest trade-off outcome
- key risks and mitigations
- expected complexity and blast radius
- suggested SDD depth (fast-track vs full pipeline)

### Step 5: Optionally Persist Exploration

If the orchestrator provided a change name, persist the analysis:

- **engram**: `mem_save` with `topic_key: sdd/{change-name}/explore`
- **sdd**: Create `sdd/changes/{change-name}/exploration.md`
- **none** or no change name: skip persistence — return analysis only

### Step 6: Return Structured Analysis

Return the structure from `skills/sdd-explore/TEMPLATES.md` (and write the same content if persisting).

## Error Recovery

| Situation | Action |
|-----------|--------|
| Codebase too large to fully explore | Focus on entry points + direct dependencies; flag unexplored areas |
| Request too vague | Return clarifying questions as `next_recommended` items |
| Multiple valid approaches, no clear winner | Present top 2 options with decision criteria to resolve |
| Cannot find related code or specs | Report what was searched; mark as likely net-new and list assumptions |

## Rules

- The ONLY file you MAY create is `exploration.md` inside the change folder (sdd mode, change name provided)
- DO NOT modify any existing code or files
- ALWAYS read real code, never guess about the codebase
- Keep your analysis CONCISE — the orchestrator needs a summary, not a novel
- If you can't find enough information, say so clearly and list what you searched
- For architecture-level explorations, include a dependency graph of affected modules
- Always include a complexity estimate to help the orchestrator decide SDD depth
- Return a structured envelope with: `status`, `executive_summary`, `detailed_report` (optional), `artifacts`, `next_recommended`, and `risks`
