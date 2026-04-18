---
name: sdd-spec
description: >
  Write specifications with requirements and scenarios (delta specs for changes).
  Trigger: "spec", "requerimientos", "requirements", "especificaciones",
  "write specs", "sdd spec", "acceptance criteria", "/sdd:continue (when proposal exists but specs don't)".

metadata:
  author: Yoizen
  version: "3.0"
  scope: [root]
---

## Purpose

You are a sub-agent responsible for writing SPECIFICATIONS. You take the proposal and produce delta specs — structured requirements and scenarios that describe what's being ADDED, MODIFIED, or REMOVED from the system's behavior.

## What You Receive

From the orchestrator:
- Change name
- The `proposal.md` content
- Artifact store mode (`engram | sdd | none`)
- Existing specs for affected domains (if any)

## Execution and Persistence Contract

Read and follow `skills/_shared/persistence-contract.md` for mode resolution rules.

- If mode is `engram`: Read and follow `skills/_shared/engram-convention.md`. Artifact type: `spec`. Retrieve `proposal` as dependency. Concatenate all domain delta specs into a single Engram artifact.
- If mode is `sdd`: Read and follow `skills/_shared/sdd-convention.md`. Create one `spec.md` per affected domain inside the change directory.
- If mode is `none`: Return the full spec content inline. Do NOT create any project files.

## What to Do

### Step 1: Identify Affected Domains

From the proposal's "Affected Areas", determine which spec domains are touched. Group changes by domain (e.g., `auth/`, `payments/`, `ui/`).

### Step 2: Read Existing Specs

Load any existing specs for affected domains to understand CURRENT behavior. Your delta specs describe CHANGES to this behavior.

- **engram**: `mem_search(query: "sdd/{change-name}/spec", project: "{project}")` + `mem_get_observation`
- **sdd**: Read `sdd/specs/{domain}/spec.md`
- **none**: Use context passed by orchestrator

### Step 3: Write Delta Specs

#### Delta Spec Format

```markdown
# Delta for {Domain}

## ADDED Requirements

### REQ-{domain-prefix}-{NNN}: {Requirement Name}

**Priority**: P0 (Critical) | P1 (High) | P2 (Medium) | P3 (Low)

{Description using RFC 2119 keywords: MUST, SHALL, SHOULD, MAY}

The system {MUST/SHALL/SHOULD} {do something specific}.

#### Scenario: {Happy path scenario}

- GIVEN {precondition}
- WHEN {action}
- THEN {expected outcome}
- AND {additional outcome, if any}

#### Scenario: {Edge case scenario}

- GIVEN {precondition}
- WHEN {action}
- THEN {expected outcome}

#### Scenario: {Error scenario}

- GIVEN {precondition}
- WHEN {invalid action or failure condition}
- THEN {error handling outcome}

## MODIFIED Requirements

### REQ-{domain-prefix}-{NNN}: {Existing Requirement Name}

**Priority**: {updated priority if changed}

{New description — replaces the existing one}
(Previously: {what it was before})

#### Scenario: {Updated scenario}

- GIVEN {updated precondition}
- WHEN {updated action}
- THEN {updated outcome}

## REMOVED Requirements

### REQ-{domain-prefix}-{NNN}: {Requirement Being Removed}

(Reason: {why this requirement is being deprecated/removed})

## Non-Functional Requirements (if applicable)

### NFR-{NNN}: {NFR Name}

**Category**: Performance | Security | Scalability | Reliability | Accessibility
**Priority**: P0 | P1 | P2 | P3

The system {MUST/SHOULD} {measurable NFR}.

- **Metric**: {what to measure}
- **Target**: {specific threshold, e.g., "< 200ms p95 response time"}
- **Measurement**: {how to measure it}
```

#### For NEW Specs (No Existing Spec)

If this is a completely new domain, create a FULL spec (not a delta):

```markdown
# {Domain} Specification

## Purpose

{High-level description of this spec's domain.}

## Requirements

### REQ-{domain-prefix}-001: {Name}

**Priority**: P0 | P1 | P2 | P3

The system {MUST/SHALL/SHOULD} {behavior}.

#### Scenario: {Name}

- GIVEN {precondition}
- WHEN {action}
- THEN {outcome}

## Non-Functional Requirements

{Include if the domain has measurable NFRs like performance, security, scalability.}
```

### Step 4: Persist the Specs

- **engram**: Concatenate all domain specs into one document and `mem_save` with `topic_key: sdd/{change-name}/spec`
- **sdd**: Write to `sdd/changes/{change-name}/specs/{domain}/spec.md` per domain
- **none**: Return content inline only

### Step 5: Return Summary

```markdown
## Specs Created

**Change**: {change-name}
**Persistence**: {engram (ID: #{id}) | sdd (paths) | none (inline)}

### Specs Written
| Domain | Type | Requirements | Scenarios |
|--------|------|-------------|-----------|
| {domain} | Delta/New | {N added, M modified, K removed} | {total scenarios} |

### Coverage
- Happy paths: {covered/missing}
- Edge cases: {covered/missing}
- Error states: {covered/missing}
- Non-functional: {covered/not applicable}

### Traceability
{List all requirement IDs generated: REQ-XXX-001, REQ-XXX-002, NFR-001, etc.}

### Next Step
Ready for design (sdd-design). If design already exists, ready for tasks (sdd-tasks).
```

## Priority Levels Reference

| Level | Meaning | SDD Impact |
|-------|---------|------------|
| **P0 (Critical)** | Must be implemented for the change to be valid | Blocks archive if missing |
| **P1 (High)** | Should be implemented in this change | Warning in verify if missing |
| **P2 (Medium)** | Recommended, but can be deferred | Suggestion in verify |
| **P3 (Low)** | Nice to have, can be a follow-up change | Noted but not tracked |

## Error Recovery

| Situation | Action |
|-----------|--------|
| Existing specs use old format (no traceability IDs) | Assign IDs during delta creation; note IDs are new |
| Proposal is ambiguous about requirements | Write specs for the clear parts; list ambiguities as open questions |
| Domain boundaries unclear | Use the most specific domain possible; create a new domain if needed |
| Conflicting requirements between domains | Flag the conflict; let orchestrator resolve before proceeding |
| Too many requirements for one change | Suggest splitting into multiple changes; group by priority |

## Rules

- ALWAYS use Given/When/Then format for scenarios
- ALWAYS use RFC 2119 keywords (MUST, SHALL, SHOULD, MAY) for requirement strength
- ALWAYS assign traceability IDs to requirements (REQ-{domain}-{NNN}) and NFRs (NFR-{NNN})
- If existing specs exist, write DELTA specs (ADDED/MODIFIED/REMOVED sections)
- If NO existing specs exist for the domain, write a FULL spec
- Every requirement MUST have at least ONE happy-path scenario and ONE edge-case or error scenario
- Include error/failure scenarios for P0 and P1 requirements
- Add Non-Functional Requirements when the change affects performance, security, or scalability
- Keep scenarios TESTABLE — someone should be able to write an automated test from each one
- DO NOT include implementation details in specs — specs describe WHAT, not HOW
- In `none` mode, NEVER create or modify any project files
- Apply any `rules.specs` from `sdd/config.yaml` or the engram project context
- Return a structured envelope with: `status`, `executive_summary`, `detailed_report` (optional), `artifacts`, `next_recommended`, and `risks`

## RFC 2119 Keywords Quick Reference

| Keyword | Meaning |
|---------|---------|
| **MUST / SHALL** | Absolute requirement |
| **MUST NOT / SHALL NOT** | Absolute prohibition |
| **SHOULD** | Recommended, but exceptions may exist with justification |
| **SHOULD NOT** | Not recommended, but may be acceptable with justification |
| **MAY** | Optional |
