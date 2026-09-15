# Thresholds — rules before LLM

A cluster needs >=3 evidences in the window. Below that, mark `Speculative`.

Classify mechanical vs judgement before picking a lever. Default to the check.

| Signal | Rule | Likely lever |
|---|---|---|
| Re-reads | `worstFileReads >= 5` or `reads >= 15` in one attempt | agent: `graft_file_api` first + pointer in SKILL/AGENTS |
| Denied tools | `invalid >= 1` | agent `permissions.json` / prompt |
| Long trace, flat score | `turns >= p90` with `weighted` unchanged vs baseline | split workflow, delegate, change model |
| Memory miss | `hit_rate < 0.3` or same symptom in `misses` >=3x | `topic_key` hygiene, consolidation, prefs save |
| Human rework | `revert` / `fixup` / tag `retrabajo` >=3x on one path | `askUserQuestion` before code, spec first; if mechanical pattern, prefer check over prompt |
| No guardrail | no pre-commit hook and no CI job running lint/typecheck/test | `code`: cheapest guardrail (linter rule, hook, or CI job) |
| Dead skill | installed but `calls == 0` in window, or called with no behavior change | fix `SKILL.md` trigger, wire into agent `tools`, or delete |
| Cost spike | `totalCostUsd` > 2x baseline with score flat | model profile (`fast`/`balanced`), rounds cap; tool: wrap the expensive call, cut polling |
| Thin window | `sessions < 5` in scope | widen 7→30→all-time AND retitle the report to the window used. Never 30d numbers under a 7d title |
| Thin memory | `evaluable < 10` | mark `Speculative`, never quote `hit_rate` as a KPI |
| Steering no-op | same instruction repeated >=3x with no behavior change, or AGENTS.md carries how-to that belongs in a skill/check | skill|code: move it out, keep pointers only; delete if it changes nothing |
| Missing info | attempt blocked on unavailable logs or access >=3x | tool: tee dev logs, readonly third-party access |

p90 comes from the collected attempts. Never invent it.
