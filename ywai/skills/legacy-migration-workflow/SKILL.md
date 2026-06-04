---
name: legacy-migration-workflow
description: Use when running ASPX to WebApi plus Angular migrations with mandatory soft gates between planning, build, and validation.
---

# Legacy Migration Workflow (Soft Gates)

Use this workflow for migration tasks in this repository.

## Objective

Enforce the ordered migration workflow with repository-level agents:
1. `migration-orchestrator` (autonomous coordinator)
2. `migration-scope` (evidence-first reconnaissance)
3. `migration-planner`
4. `build` (initial implementation)
5. `migration-validator` (full validation)
6. `migration-validator-focused` (post-remediation focused validation)
7. `build` (remediation loop, when needed)

No phase can be skipped.

## Gate model

- Planning phase writes `docs/migrations/plans/<legacy-page-slug>.md` with `status: planned`.
- `planned` means the plan is directly executable by `migration-build`; there is no approval gate.
- A plan is not executable unless it contains a row-by-row Legacy Parity Contract, Visual Parity Inventory for UI-bearing pages, Enum/Icon Matrix for mapped values, i18n render gate, permission/license parity, and foundation dependency gates.
- Planning may be iterative. If the planner has doubts, it must return `AWAITING_INPUT` with targeted questions and must not write or update the plan until the answers make the scope executable.
- Build phase only runs when status is `planned`, then sets `status: implemented` after attaching implementation/test/build evidence to the plan.
- Validation phase runs when status is `implemented` or `remediation-needed`.
- If validation fails, it sets `status: remediation-needed` and hands off to remediation.
- Remediation phase only runs when status is `remediation-needed`.
- Only validation can set `status: validated`.
- Build and remediation phases must not claim final parity. They may only mark parity rows as implemented-with-evidence and request `migration-validate`.
- `blocked` is reserved for missing human decisions, not for missing code that can be added to the plan.

If a gate is not met, return:

`BLOCKED: <reason and expected next command>`

## Commands

1. `migration-plan <legacy-page>`
2. `migration-build <legacy-page>`
3. `migration-validate <legacy-page>`
4. `migration-remediate <legacy-page>`
5. `migration-run <legacy-page>`
6. `migration-scope <legacy-page>`
7. `migration-validate-focused <legacy-page>`

## Planning loop

- `migration-plan` owns discovery, user clarification, and final scope definition.
- The planner should iterate with the user before writing the plan whenever behavior, naming, parity impact, or scope boundaries are unclear.
- Use `AWAITING_INPUT` for targeted questions.
- Do not create or update the plan while awaiting clarifications.
- Once written, the plan must be decent, complete enough, and directly executable by `migration-build`.

## Mandatory parity coverage

- Legacy business logic parity (`WebMethod`, `RegisterJsonVariable`, Page_Load branches, hidden branches, default values, validation, side effects).
- UI/visual parity: filters, table columns/order, buttons/actions, row actions, dialogs/popups, tabs/sections, scroll containers, empty/loading/error states, and approved deviations.
- Shared/global UI and API capabilities discovered from legacy helpers, popups, drill-ins, catalogs, and reusable surfaces.
- Permission parity.
- License parity.
- `isSuper` parity.
- i18n parity across every supported language and rendered UI key validation.
- Enum/icon parity with exact legacy enum source and numeric values.
- DTO reuse-first policy.

## Shared / foundation scope expansion

- Missing shared/global capabilities are not blockers by default.
- The planner must add them to `Shared / foundation dependencies` as required migration work with stable `F###` IDs.
- The build phase implements page-local scope plus all in-scope shared/foundation dependencies.
- The validator validates page parity plus shared/foundation dependency completeness.
- Validation must fail if a required shared/foundation dependency is missing, partial, unwired, or untested.
- Defer shared/foundation work only when the user explicitly approves the parity impact.

## Evidence requirements

- Every plan must contain a Legacy Parity Contract table. Each row maps `ID`, `Legacy element`, `Legacy source`, `Modern API/DTO`, `Modern UI`, `Test/evidence`, and `Status`.
- Every UI-bearing migration must contain a Visual Parity Inventory. The inventory must cite the legacy markup/script source and the modern template/style source.
- Every enum/status/icon/label mapping must contain an Enum/Icon Matrix with exact enum file, member name, numeric value, modern mapping, i18n key, icon/badge class, and evidence.
- Every visible string added or reused by the migration must be present in all supported language bundles. Validation fails on raw rendered translation keys.
- Build/remediation must append an Evidence Log entry with commands run, results, files changed, and affected parity rows before requesting validation.
- Validation reads the evidence but must independently inspect source and tests; evidence alone is not proof of parity.

## Mandatory governance coverage

- Unit tests required.
- Integration tests where feasible.
- `Yoizen.Legacy/migration-progress-tracker.md` update required in same change set.

## Tracker validation requirement

Validation must fail if any of these is missing or inconsistent:
- Migrated page line status was not updated.
- `### Overall Progress` counters/percentages mismatch page status totals.
- `### By Module Progress` counters/percentages mismatch module totals.
- `*Last Updated:*` not refreshed after tracker modification.

## Artifacts

- Plan: `docs/migrations/plans/<legacy-page-slug>.md`
- Tracker: `Yoizen.Legacy/migration-progress-tracker.md`

## Notes

- This is a soft workflow. Enforcement is contract-based via status files and command behavior.
- The validator never edits app source code; it only validates and updates findings in the plan.
- Validation findings should carry run metadata so repeated validation rounds do not create ambiguous comments.
- `migration-approve` was removed from the happy path; `migration-plan` now produces a directly buildable plan.

## Autonomous orchestration

Use `migration-run <legacy-page>` when the user wants the full workflow to continue without manually running each phase command.

`migration-run` coordinates the existing phases only. It must not collapse their responsibilities:
- planning questions still stop the workflow with `AWAITING_INPUT`
- implementation still runs through `build`
- validation still runs through `migration-validator`
- remediation still runs through `build`
- only validation can set `status: validated`

Autonomous flow:

```text
missing plan -> plan
planned -> build -> validate
implemented -> validate
remediation-needed -> remediate -> validate
validated -> stop completed
blocked/AWAITING_INPUT -> stop for user input
```

`migration-run` must be durable and resumable. It must decide the next phase by reading the current plan, tracker, findings, remediation tasks, resolution log, evidence log, and worktree state. It must not rely on chat memory or a previous subtask result.

Recommended plan frontmatter metadata:

```yaml
workflow:
  runId: MR-20260601-01
  phase: validating
  validationRound: 2
  maxValidationRounds: 5
  lastStartedAt: 2026-06-01T00:00:00Z
  lastCompletedAt: null
  lastFindingFingerprint: V001|V003|V004
```

Loop guards:
- default maximum validation rounds: `5`
- stop with `LOOP_GUARD` when the same open finding fingerprint appears after remediation
- stop with `LOOP_GUARD` when remediation produces no evidence, resolution log update, finding/task state change, or relevant source/test diff
- stop with `MAX_ROUNDS_REACHED` when the validation round limit is reached

Interruption behavior:
- interrupted planning resumes by rerunning planning unless a complete `planned` artifact exists
- interrupted build resumes by inspecting current code and evidence before continuing idempotently
- interrupted validation can be rerun because validation does not edit application source
- interrupted remediation resumes from open findings/tasks and verifies partial fixes before adding more changes

## Huge legacy surfaces and work graphs

Concrete page names in examples are illustrative only. Agents must not infer dependencies from examples or prior conversations. Dependencies must be supported by evidence in the requested legacy source, related scripts, handlers, existing plans, or modern source references.

Before creating an executable plan, classify the requested legacy surface:
- `small`
- `medium`
- `large-cohesive`
- `huge-split-required`
- `ambiguous-needs-input`

Automatic split is sequencing only and does not require user approval when parity is unchanged. Deferral is reduced parity and requires explicit user approval.

Automatic split indicators:
- estimated LPC rows `>= 30`
- foundation dependencies `>= 4`
- independent UI surfaces `>= 3`
- global helper/drill-in dependencies `>= 3`
- AJAX/WebMethod/handler clusters `>= 4`
- complex renderers: conversation, chat, survey, logs, exports, overlays, attachments
- required modern foundations are `missing`, `partial`, or `requires-extension`
- peer legacy surfaces or shared drill-ins are discovered from evidence
- repeated structural validation findings already exist

Huge surfaces use a `Migration Work Graph` in the parent plan. Graph nodes may be:
- `parent`
- `foundation`
- `peer-foundation`
- `parent-composition`

Work graph rules:
- parent validation cannot pass while required graph nodes are not `validated`
- existing plans are reused instead of duplicated
- missing dependencies become child/foundation plans unless a human decision is required
- graph cycles stop with `GRAPH_CONFLICT`
- final parent validation is always full validation

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
