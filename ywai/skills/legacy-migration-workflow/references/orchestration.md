# Autonomous orchestration and huge legacy surfaces

Reference for `legacy-migration-workflow`. Read this when running
`migration-run` end to end, or when the legacy surface is too big for one plan.

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

