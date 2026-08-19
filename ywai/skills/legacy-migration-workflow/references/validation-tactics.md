# Validation tactics — dependency audit, token efficiency, evidence artifacts

Reference for `legacy-migration-workflow`. Read this during the validation phase.

## Evidence-first dependency audit

Allowed dependency states:
- `validated`
- `implemented-needs-validation`
- `remediation-needed`
- `planned`
- `missing`
- `partial`
- `blocked`
- `explicitly-deferred`

Mark a dependency `validated` only when specific evidence exists:
- validated plan or documented shared foundation reference
- relevant parity rows have row-level source/test/render evidence
- validation evidence is specific, not blanket evidence
- tracker state is consistent when tracker-visible

Never mark a dependency ready from matching names, existing files, tracker status alone, generic build evidence, or examples in this skill.

## Token-efficient validation

Use full validation for:
- first validation of a plan
- final validation before `status: validated`
- focused validator escalation
- digest/work graph changes
- touched files outside focused scope
- dependency status changed without evidence
- tracker/plan inconsistency
- direct concurrent conflict

Use focused validation after remediation when affected findings, rows, and files are bounded.

Focused validation may return:
- `FOCUSED_APPROVED`
- `FOCUSED_REJECTED`
- `ESCALATE_FULL_VALIDATION`
- `EVIDENCE_GAP`

Focused validation cannot set a parent plan to `validated`; it can only hand off to final full validation when no open findings remain.

## Compact durable sections

Plans should include these token-saving sections:
- `Workflow State`
- `Scope Sizing Gate`
- `Legacy Discovery Digest`
- `Migration Work Graph` for huge/parent plans
- `Phase Handoff`

Agents should read those sections first. If missing or inconsistent, fall back to the full plan.

## Evidence artifact policy

Evidence belongs inside the migration plan artifact. Do not create standalone Markdown files for validation rounds, remediation rounds, findings, evidence logs, handoffs, or command output unless the user explicitly requests a separate artifact.

Allowed plan artifacts:
- one parent plan for a huge surface
- one child/foundation plan per independently validatable reusable capability
- one peer plan only when the peer surface is itself a real migration target

Do not create one-off files like `evidence-v001.md`, `validation-round-1.md`, `remediation-v003.md`, or `handoff-remediation.md` by default.

Validated migration plans must not be deleted automatically. They are retained as the parity contract, validation record, and audit trail. If the user wants less clutter, ask before adopting an archive strategy such as moving validated plans to an archive folder.
