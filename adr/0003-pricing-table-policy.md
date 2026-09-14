---
status: accepted (amended 2026-09-11: catalog fetch supersedes the table as primary source)
date: 2026-09-11
decision-makers: "ywai owner (approved evals improvement plan, 2026-09-11)"
---

# Model pricing table policy: code-adjacent, drift-commented, never guessed

## Amendment 2026-09-11 (slice B2): models.dev catalog supersedes the table

The "no live pricing fetch" non-goal below is lifted. Model prices now come from
the models.dev catalog (`https://models.dev/api.json`: provider -> models ->
cost `{input, output}` USD per 1M tokens), fetched live and cached on disk at
`~/.ywai/models-dev-cache.json` with a 7-day TTL. Refresh is bounded (2s HTTP
timeout) at most once per process and only when the cache is stale; scoring
never fails or blocks on the network. Degradation ladder: fresh cache ->
synchronous refresh -> stale cache -> embedded 17-row fallback table
(approximate, frozen early-2026 rates, fallback-only) -> unknowns stay
`(0, false)`. The embedded table is no longer updated in place when rates
change; drift is absorbed by the catalog. Duplicate model ids across providers
resolve deterministically to the sorted-first provider. Implementation:
`ywai/internal/evals/pricing.go` (`modelsDevBaseURL` + `pricingCachePath` are
package vars, injectable for tests); tests use httptest stubs only, no live
network. The `costKnown = false` honesty rule and the cost-gate skip rule below
remain in force unchanged.

## Context and Problem Statement

Phase 1 adds a cost estimate per bench attempt (USD per model from token usage).
Provider prices change without notice, and the repo cannot query a live pricing API
from CI or from a developer machine. A stale table that reports a confident number
is worse than no number: a cost gate on a wrong price blocks releases for nothing,
and a wrong cost column teaches everyone to ignore costs.

## Decision

1. The pricing table lives code-adjacent: next to `LookupCost` in
   `ywai/internal/evals/pricing.go`, not in configuration, not in a data file.
2. The table carries a drift comment that states the price source and the date it
   was last verified, so the next reader knows exactly how stale it may be.
3. An unknown model (or unknown tier of a known model) resolves with
   `costKnown = false`, and the cost fields stay blank (`omitempty`). Unknowns are
   never priced from a "similar" model. The UI renders a blank as an em dash.
4. Cost gates (a later phase) only evaluate when `costKnown` is true for the run;
   an unknown price is a skip, never a zero-dollar pass.

Non-goals: no live pricing fetch, no per-deployment price overrides, no currency
conversion.

## Consequences

* Good, because pricing needs no network access, no new dependency, and no config
  plumbing.
* Good, because drift is visible and bounded: the comment dates the data, and
  `costKnown` prevents stale numbers from wearing a false badge of accuracy.
* Bad, because the table will drift until someone edits it; accepted, and made
  harmless by the `costKnown = false` path for anything uncertain.
* Bad, because a reviewer updating prices must trust the comment's date; PR review
  of a pricing diff should re-check the source.

## Implementation Plan

* **Affected paths**: `ywai/internal/evals/pricing.go` (table + `LookupCost`),
  attempt cost fields computed in the runner (`costUsd`, `costKnown`), golden
  pricing fixtures in a later phase.
* **Dependencies**: none new.
* **Patterns to follow**: prefix-match on model names (family before variant), so
  a new snapshot of an existing model inherits the family price only while it is
  explicitly listed.
* **Patterns to avoid**: estimating costs for unlisted models; storing prices in
  user-facing configuration; silently rounding an unknown to 0.

### Verification

- [ ] `LookupCost("totally-unknown-model", …)` returns `costKnown = false` and no
      dollar figure.
- [ ] A listed model returns its table price with `costKnown = true`.
- [ ] The drift comment names a source and a last-verified date.

## More Information

Approved plan: phased evals bench improvement (Phase 1, pricing slice). Related:
ADR 0001 (weighted scoring) and ADR 0002 (run storage). Revisit when a provider
offers a stable pricing endpoint, or when CI cost gates need fresher data than
manual edits give.
