# Thresholds — rules before LLM

A cluster needs >=3 evidences in the window. Below that, mark `Speculative`.

| Signal | Rule | Likely lever |
|---|---|---|
| Re-reads | `worstFileReads >= 5` or `reads >= 15` in one attempt | agent: force `graft_file_api` first; skill `codebase-design` |
| Denied tools | `invalid >= 1` | agent `permissions.json` / prompt |
| Long trace, flat score | `turns >= p90` with `weighted` unchanged vs baseline | split workflow, delegate, change model |
| Memory miss | `hit_rate < 0.3` or same symptom in `misses` >=3x | `topic_key` hygiene, consolidation, prefs save |
| Human rework | `revert` / `fixup` / tag `retrabajo` >=3x on one path | `askUserQuestion` before code, spec first |
| Dead skill | installed but `calls == 0` in window | fix `SKILL.md` trigger, or wire into agent `tools` |
| Cost spike | `totalCostUsd` > 2x baseline with score flat | model profile (`fast`/`balanced`), rounds cap |
| Thin window | `sessions < 5` in scope | widen 7→30→all-time AND retitle the report to the window used. Never 30d numbers under a 7d title |
| Thin memory | `evaluable < 10` | mark `Speculative`, never quote `hit_rate` as a KPI |

p90 comes from the collected attempts. Never invent it.
