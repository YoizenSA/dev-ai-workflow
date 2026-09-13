# Queries — what to run for each question

Always `--json`. Save every output under `/tmp/retro-YYYYMMDD/`.

## Sessions: who did what, how much

```bash
ywai eval run --days 7 --worktree <name> --json > sessions.json
ywai eval sessions --project <id> --top 20 --json > sessions-project.json
# Same data as GET /api/evals/session-analytics?days=7
```

Read: `agents / skills / models / tools / projects`, `totalCost`, `tokensInput/Output`, `sessionsWithSkill`.

## Evals: did it work, at what cost

```bash
curl -s "http://localhost:5768/api/evals/runs" | head -c 20000 > eval-runs.json
curl -s "http://localhost:5768/api/evals/summary?taskId=<id>" > eval-summary.json
ywai eval bench --task <id> --model <m> --baseline-run latest --rounds 1
```

Per attempt: `score.weighted`, `gotHard`, `metrics.turns/calls/reads/worstFile/invalid`, `tokensIn/Out`, `costUsd`.

## Memory: does recall work

```bash
curl -s "http://localhost:5768/api/engram/search?q=<symptom>&limit=50" > mem-search.json
curl -s "http://localhost:5768/api/engram/prompts?limit=200" > mem-prompts.json
curl -s "http://localhost:5768/api/engram/observations?limit=200" > mem-obs.json
curl -s -X POST "http://localhost:5768/api/engram/memory-evals" \
  -H 'content-type: application/json' \
  -d '{"sample_size":100,"k":10,"project":"<X>"}' > mem-eval.json
```

Read `hit_rate / MRR / misses[].top_result`. A miss with a long prompt is the interesting one.

## Git: objective human-feedback proxy

```bash
git log --since=7.days --oneline --grep="fix\\|revert\\|retry" > git-fixes.txt
git log --since=7.days --oneline -- <hot-path> > git-hotspot.txt
```

Until session close-tags exist (`ok / retrabajo / alucinó / pidió-de-más / lento`), reverts + fixups are the signal.

## Freshness check (do this first)

The live server writes `session_v2`; the legacy `session` table is v1-era and goes stale. If analytics numbers disagree with raw counts, the reader is on the wrong schema — report it as finding #1, it blinds the whole retro:

```sql
-- row counts per schema (open opencode.db read-only)
select (select count(*) from session_v2), (select count(*) from session);
-- max session date per schema (ms epoch)
select max(time_created) from session_v2;
select max(time_created) from session;
```

If `session_v2` is current and analytics shows days-old data, say so in the report header and do not draw session conclusions.

## Windows (PowerShell)

- Never giant inline one-liners with nested quotes: the parser breaks (`TerminatorExpectedAtEndOfString`). Write `collect.ps1` in the bundle dir and run it.
- `Out-File -Encoding utf8NoBOM` always. Plain `utf8` writes BOM and breaks `json.load`.
- Python must open with `encoding='utf-8-sig'`.
