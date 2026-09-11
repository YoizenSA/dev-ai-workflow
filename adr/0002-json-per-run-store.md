---
status: accepted
date: 2026-09-11
decision-makers: "ywai owner (approved evals improvement plan, 2026-09-11)"
---

# Store eval runs as one JSON file per run

## Context and Problem Statement

Phase 2 of the evals plan moves run persistence out of the control server into
`internal/evals`. Today runs accumulate in a single JSON file under `~/.ywai/`
(`eval-runs.json`), rewritten whole on every change. That file grows without bound,
every attempt rewrites everything, and it is one concurrent-write bug away from a
truncated history. The bench is also gaining per-model summaries, baselines, and a
leaderboard (Phases 1–2), all of which want cheap reads of one run or a recent
window of runs — not a full-file scan.

`modernc.org/sqlite` is already in `go.mod`, so a real database is one import away.
The choice had to be made deliberately, not by inertia.

## Decision

Store one JSON file per run:

- Layout: `~/.ywai/eval-runs/<id>.json` for each run, plus `index.json` holding the
  run headers (id, task, date, models) for listing.
- Migration: a one-time split of the legacy single file — it is renamed to
  `eval-runs.json.imported` (never deleted) and its runs are written out as
  individual files; the index is rebuilt by scanning the directory if needed.
- Retention: keep the most recent 500 runs, tunable with `EVAL_RUNS_KEEP`.
- Concurrency: a single writer, guaranteed by the existing bench in-flight lock;
  no file-level locking is added.

Rationale for JSON-per-run over sqlite: run blobs are small (kilobytes), the access
pattern is rewrite-per-attempt of exactly one run, and the file layout carries no
schema or migration overhead — a corrupted run file costs one run, not the store.
sqlite's strengths (indexed queries over large data, transactions) match none of
those needs at this scale.

Non-goals: no query language, no embedded server, no concurrent writers, no
cross-machine sync of the store.

## Consequences

* Good, because reading or rewriting one run touches one small file.
* Good, because the store has zero schema/migration surface going forward.
* Good, because the migration is reversible by construction (rename, not delete).
* Bad, because listing depends on the index file, so index and run files can drift
  if a crash lands between writes; the rebuild-by-scan path covers this.
* Bad, because aggregate queries (leaderboard over 30 days) read many files; at
  500 runs this is fine, and it is the trigger to revisit.

## Implementation Plan

* **Affected paths**: `ywai/internal/evals/store.go` (new: `Store{list, get,
  upsert}`), control-server persistence call sites (moved, not kept in parallel),
  `~/.ywai/eval-runs/`.
* **Dependencies**: none new (standard library only).
* **Patterns to follow**: `LoadTasks`-style tolerant reads (one unreadable run
  must not hide the rest); env-var overrides like existing ywai configuration.
* **Patterns to avoid**: rewriting the index on every attempt; deleting the legacy
  file during migration.

### Verification

- [ ] Starting against a legacy `eval-runs.json` produces `eval-runs.json.imported`
      plus one file per run, and the old file content is intact.
- [ ] A corrupted run file is skipped on list, not fatal.
- [ ] Retention trims to `EVAL_RUNS_KEEP` and the index stays consistent.

## More Information

Approved plan: phased evals bench improvement (Phase 2, tracking). Related: ADR 0001
(weighted scoring) and ADR 0003 (pricing policy). Revisit if run volume grows past
the retention window by an order of magnitude or cross-run queries become hot.
