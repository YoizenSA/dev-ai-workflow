---
name: workflow-retro
description: "Weekly workflow retro over sessions, evals, memory. Trigger: retro, auditar semana."
---

# Workflow Retro

One question: what friction repeated this week, and what single change removes it?

Query via `ywai`. Raw DB reads only for the freshness check in [references/queries.md](references/queries.md).

## Rules

1. Fix scope first: `days` (default 7), `project` or `worktree`. No scope, no retro.
2. Collect every source before concluding. One failed source never stops the run.
3. Validate freshness before analyzing. A blind tracker is finding #1, not a footnote.
4. Cluster only on >=3 evidences. One anecdote is not a finding.
5. Falsify before proposing. Each hypothesis states what would disprove it; check that first.
6. One friction = one lever (skill|agent|tool|code) + one experiment. Never two changes in one experiment. Classify first: mechanical (fixed syntactic pattern, banned API, import shape, file-location rule) goes to a deterministic check (lever code|tool: repo linter rule, pre-commit hook, or CI job); judgement call (cross-file consistency, matches surrounding style, needs intent) goes to skill|agent. Default to building the check. Place the fix where it runs. Review imposes standards. Implementation follows pointers. AGENTS.md holds pointers only.
7. Report as HTML (see [references/report.md](references/report.md)). OS temp dir, open it, report the absolute path. Nothing lands in the repo.
8. `mem_save` only the top 3 as `pattern`. The detail lives in the report.

## Process

### 1. Scope

Ask at most: project/worktree + days. Default `days=7`. Record the exact filters in the report header.

### 2. Collect

Run in order, `--json` always. Full command list in [references/queries.md](references/queries.md):

- Freshness: `session_v2` vs `session` counts + max dates, raw SQL, read-only.
- `ywai eval run --days <n> --worktree <w> --json` — sessions aggregate.
- `GET /api/evals/runs` + `/api/evals/summary` — hard scores, turns, invalid, cost, attempt errors.
- `GET /api/engram/search?q=<symptom>` + `/prompts` + `/observations` + `POST /api/engram/memory-evals` — recall quality.
- `git log --since=<n>.days` — oneline + name-only + authors.
- Guardrail: repo check command + hooks + CI workflow (`lint`/`check` scripts, pre-commit hook, CI lint/typecheck/test job). Read before proposing a new check. An unwired or broken check is the finding, not a reinvention.

### 3. Validate

Apply [references/thresholds.md](references/thresholds.md): thin window → widen AND retitle; stale tracker → finding #1; thin memory → `Speculative`. Pick ONE primary window here; everything downstream uses it.

### 4. Cluster

Rules first ([references/thresholds.md](references/thresholds.md)), LLM second. Method in [references/analysis.md](references/analysis.md): rework done right, early-death vs hard-task, cost-vs-overhead, cross-source joins.

### 5. Interrogate

For each cluster, write the disproof before the proposal: the one check that would kill your hypothesis (attempt `error` field, revert list, author split). Run it. Dead hypotheses die here, silently.

### 6. Propose

Survivors become cards: evidence (3 sessions + metrics) / hypothesis / lever / ONE experiment / success metric. Rank by frequency × impact × fixability. Fewer Strong findings beats padded ones. A repo with no guardrail (no pre-commit hook and no CI job running lint/typecheck/test) is itself a finding, not a neutral default.

### 7. Report + save

Render `assets/report-template.html` with the data. Buttons for PDF + PNG ship in the template. Then `mem_save` top 3.
