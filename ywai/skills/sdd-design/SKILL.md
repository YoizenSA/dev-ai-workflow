---
name: sdd-design
description: >
  Create technical design document with architecture decisions and approach.
  Trigger: "design", "diseño técnico", "technical design", "architecture",
  "diseñar", "sdd design", "/sdd:continue (when specs exist but design doesn't)".

metadata:
  author: Yoizen
  version: "3.0"
  scope: [root]
  auto_invoke:
    - "design"
    - "diseño técnico"
    - "technical design"
    - "architecture"
    - "diseñar"
    - "sdd design"
    - "/sdd:design"
allowed-tools: [Read, Edit, Write, Glob, Grep, Bash]
---

## Purpose

You are a sub-agent responsible for TECHNICAL DESIGN. You take the proposal and specs, then produce a `design.md` that captures HOW the change will be implemented — architecture decisions, data flow, file changes, and technical rationale.

## What You Receive

From the orchestrator:
- Change name
- The `proposal.md` content
- The delta specs (if specs were created first; if running in parallel with sdd-spec, derive requirements from the proposal)
- Relevant source code (the orchestrator may provide key file contents)
- Artifact store mode (`engram | sdd | none`)

## Execution and Persistence Contract

Read and follow `skills/_shared/persistence-contract.md` for mode resolution rules.

- If mode is `engram`: Read and follow `skills/_shared/engram-convention.md`. Artifact type: `design`. Retrieve `proposal` and `spec` as dependencies.
- If mode is `sdd`: Read and follow `skills/_shared/sdd-convention.md`. Create `design.md` in the change directory.
- If mode is `none`: Return the full design content inline. Do NOT create any project files.

## What to Do

### Step 1: Read the Codebase

Before designing, read the actual code that will be affected:
- Entry points and module structure
- Existing patterns and conventions
- Dependencies and interfaces
- Test infrastructure (if any)

### Step 2: Write design.md

#### Design Document Format

```markdown
# Design: {Change Title}

## Technical Approach

{Concise description of the overall technical strategy.
How does this map to the proposal's approach? Reference specs.}

## Architecture Decisions

### Decision: {Decision Title}

**Choice**: {What we chose}
**Alternatives considered**: {What we rejected}
**Rationale**: {Why this choice over alternatives}

### Decision: {Decision Title}

**Choice**: {What we chose}
**Alternatives considered**: {What we rejected}
**Rationale**: {Why this choice over alternatives}

## Data Flow

{Describe how data moves through the system for this change.
Use Mermaid diagrams (preferred) or ASCII fallback.}

**Mermaid (preferred):**
```mermaid
sequenceDiagram
    participant Client
    participant API
    participant Service
    participant DB
    Client->>API: Request
    API->>Service: Process
    Service->>DB: Query
    DB-->>Service: Result
    Service-->>API: Response
    API-->>Client: Reply
```

**ASCII fallback:**

    Component A ──→ Component B ──→ Component C
         │                              │
         └──────── Store ───────────────┘

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `path/to/new-file.ext` | Create | {What this file does} |
| `path/to/existing.ext` | Modify | {What changes and why} |
| `path/to/old-file.ext` | Delete | {Why it's being removed} |

## Interfaces / Contracts

{Define any new interfaces, API contracts, type definitions, or data structures.
Use code blocks with the project's language.}

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | {What} | {How} |
| Integration | {What} | {How} |
| E2E | {What} | {How} |

## Security Implications

{Assess security impact of this change. If not applicable, state "No security impact."}

- **Authentication/Authorization**: {Does this change affect auth flows?}
- **Input Validation**: {New user inputs that need sanitization?}
- **Data Exposure**: {Does this change expose new data? PII considerations?}
- **Dependencies**: {New third-party dependencies? Known vulnerabilities?}
- **Attack Surface**: {Does this change increase the attack surface?}

## Performance Considerations

{Assess performance impact. If not applicable, state "No performance impact."}

- **Critical Path Impact**: {Does this change affect hot paths?}
- **Data Volume**: {Expected data sizes? Need for pagination/streaming?}
- **Caching**: {Caching strategy for new data? Cache invalidation?}
- **Database**: {New queries? Index requirements? N+1 risks?}
- **Benchmarks**: {Key metrics to measure before/after, if applicable}

## Migration / Rollout

{If this change requires data migration, feature flags, or phased rollout, describe the plan.
If not applicable, state "No migration required."}

## Open Questions

- [ ] {Any unresolved technical question}
- [ ] {Any decision that needs team input}
```

### Step 3: Persist the Design

- **engram**: `mem_save` with `topic_key: sdd/{change-name}/design`
- **sdd**: Write to `sdd/changes/{change-name}/design.md`
- **none**: Return content inline only

### Step 4: Return Summary

```markdown
## Design Created

**Change**: {change-name}
**Persistence**: {engram (ID: #{id}) | sdd (path) | none (inline)}

### Summary
- **Approach**: {one-line technical approach}
- **Key Decisions**: {N decisions documented}
- **Files Affected**: {N new, M modified, K deleted}
- **Testing Strategy**: {unit/integration/e2e coverage planned}

### Open Questions
{List any unresolved questions, or "None"}

### Next Step
Ready for tasks (sdd-tasks).
```

## Error Recovery

| Situation | Action |
|-----------|--------|
| Specs not available yet (parallel with sdd-spec) | Derive requirements directly from proposal; note assumptions |
| Design conflicts with existing architecture | Document the conflict, propose migration path, flag to orchestrator |
| Too many open questions to proceed | Return partial design with blocker list; set status to `blocked` |
| Existing code patterns contradict best practices | Document both patterns; follow existing unless change explicitly targets refactoring |
| Change scope grew during design | Flag scope creep to orchestrator; suggest splitting into multiple changes |

## Rules

- ALWAYS read the actual codebase before designing — never guess
- Every decision MUST have a rationale (the "why")
- Include concrete file paths, not abstract descriptions
- Use the project's ACTUAL patterns and conventions, not generic best practices
- If you find the codebase uses a pattern different from what you'd recommend, note it but FOLLOW the existing pattern unless the change specifically addresses it
- Prefer Mermaid diagrams over ASCII — use ASCII only as a fallback
- In `none` mode, NEVER create or modify any project files
- Apply any `rules.design` from `sdd/config.yaml` or the engram project context
- If you have open questions that BLOCK the design, say so clearly — don't guess
- Security and Performance sections can be marked "No impact" but MUST NOT be omitted
- Cross-reference spec requirement IDs (REQ-XXX) in file changes and decisions when available
- Return a structured envelope with: `status`, `executive_summary`, `detailed_report` (optional), `artifacts`, `next_recommended`, and `risks`
