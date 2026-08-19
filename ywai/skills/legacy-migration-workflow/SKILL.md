---
name: legacy-migration-workflow
description: "ASPX to WebApi + Angular migrations with gates between planning, build, and validation."
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

## Artifacts

- Plan: `docs/migrations/plans/<legacy-page-slug>.md`
- Tracker: `Yoizen.Legacy/migration-progress-tracker.md`

## Notes

- This is a soft workflow. Enforcement is contract-based via status files and command behavior.
- The validator never edits app source code; it only validates and updates findings in the plan.
- Validation findings should carry run metadata so repeated validation rounds do not create ambiguous comments.
- `migration-approve` was removed from the happy path; `migration-plan` now produces a directly buildable plan.

## References — read the one the phase needs

| Phase | Reference |
|-------|-----------|
| **Planning / validating a plan** | [references/parity-and-evidence.md](references/parity-and-evidence.md) — the Legacy Parity Contract, Visual Parity Inventory, Enum/Icon Matrix, i18n and permission parity, governance and tracker requirements. A plan without these is not executable. |
| **Running the full loop, or a surface too big for one plan** | [references/orchestration.md](references/orchestration.md) — autonomous coordination and work graphs for huge legacy surfaces. |
| **Validating** | [references/validation-tactics.md](references/validation-tactics.md) — evidence-first dependency audit, token-efficient validation, evidence artifact policy. |

No phase can be skipped, and no gate can be met from memory: open the reference
for the phase you are in before claiming its status.
