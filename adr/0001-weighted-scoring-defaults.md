---
status: accepted
date: 2026-09-11
decision-makers: "ywai owner (approved evals improvement plan, 2026-09-11)"
---

# Weighted scoring defaults for eval expectations

## Context and Problem Statement

The evals bench scores a model response by counting which expectations (needles) its
answer contains. Every expectation counts the same, so a model that misses the one
hard, multi-hop expectation can still out-score a model that got it — the raw score
cannot tell a shallow answer from a real trace. The `hard` flag exists but only sets
a boolean (`GotHard`), so it never affects ranking.

Phase 1 of the approved evals plan adds a weighted score. Two questions had to be
settled before any code: what a weighted expectation weighs when the task author
said nothing, and what happens to runs scored before weighting existed.

## Decision

1. The effective weight of an expectation is:
   - its explicit `Weight` when greater than 0 (the author wins, including over the
     hard default);
   - otherwise `2` when `Hard` is set (the hard expectation separates a real trace
     from a shallow one, so it counts double);
   - otherwise `1`.
2. `Score.Weighted` is `hitWeight / totalWeight` over all expectations of the task,
   a share from 0 to 1. An unanswered attempt scores `Weighted` 0 and keeps
   `answered = false`: a missing measurement is never turned into a wrong answer.
3. History is never backfilled. `Weighted` is `omitempty`, so runs scored before
   Phase 1 carry no weighted value at all. Old and new runs must not be compared
   weighted-to-blank; any future delta feature must treat a blank as "not
   comparable", not as zero.

Non-goals: no change to what counts as a hit (matching stays a lower-cased
substring test); no per-task normalization configuration; no backfill tooling.

## Consequences

* Good, because tasks become comparable within a class: hitting the hard part now
  moves the number, not just a flag.
* Good, because the additive `omitempty` field breaks neither old run JSON nor old
  task JSON.
* Bad, because pre-Phase-1 runs are not weighted-comparable with new runs; this is
  accepted and made visible by the blank field instead of hidden by a zero.
* Bad, because an author can set an unreviewed weight that dominates a task;
  mitigated by tasks being versioned in the repo and reviewable in a PR.

## Implementation Plan

* **Affected paths**: `ywai/internal/evals/task.go` (fields `Task.Tags`,
  `Task.Difficulty`, `Expectation.Weight`, `Score.Weighted`, wiring in
  `Task.Score`), `ywai/internal/evals/scoring.go` (weight rules), task JSON files.
* **Dependencies**: none new.
* **Patterns to follow**: additive optional JSON fields with `omitempty`, matching
  the existing `Description` / `Hard` style.
* **Patterns to avoid**: backfilling weighted values onto historical runs; storing
  the weight rules in more than one place (they live only in `scoring.go`).

### Verification

- [x] A hard expectation and a soft one weigh 2 and 1; hitting only the soft one
      scores 1/3 weighted (`TestWeightedScoreCountsHardDouble`).
- [x] An explicit weight overrides the hard default
      (`TestWeightedScoreHonorsExplicitWeight`).
- [x] An unanswered response scores weighted 0 with `answered` false
      (`TestWeightedScoreIsZeroWhenUnanswered`).
- [x] A pre-Phase-1 score JSON without `weighted` unmarshals
      (`TestScoreUnmarshalsLegacyRunWithoutWeighted`).

## More Information

Approved plan: phased evals bench improvement (Phase 1, foundation). Related:
ADR 0002 (run storage) and ADR 0003 (pricing table policy) from the same plan.
Revisit if task authors find the 2:1 hard default too coarse for a whole task
class.
