# Vendored: browser-use/jev-ultrafast

- Upstream: https://github.com/browser-use/jev-ultrafast (MIT, see LICENSE)
- Commit: `452c1ad2dd628008f1d5608f28158d76e49e6cc0`
- Kept: `jev_ultrafast/{__init__,agent,browser,model,questions}.py`, `snapshot.js`, `tests/test_agent.py`, `pyproject.toml`, `LICENSE`.
- Dropped: `demo.py`, `static/`, `docs/`, `scripts/`, `examples/`, `README.md`, `AGENTS.md`, `uv.lock`.

## Local patches

1. `pyproject.toml`: removed `readme`, the `jev` script (demo), and the `pillow` dev dependency.
2. `tests/test_agent.py`: removed `test_flight_verification_rejects_wrong_trip` (imports the dropped
   `examples/`) and the two generated-text cache tests (the agent no longer generates text).
3. `agent.py` patch A (values): `Agent(..., values=)`. A `fill` target whose label or `name`
   attribute matches a key (case-insensitive, trimmed) is typed from `values`. Any unmatched field stops
   the run as `blocked` with `blocked_reason: "missing value: '<label>'"`. The agent never calls the
   text model (`model.field_text` is left in place but unused), so `TEXT_MODEL_API_KEY` is not needed. Supplied text is recorded as `***`
   in history (history also feeds TypeSafe's `recent_actions`).
4. `agent.py` patch B (goal hygiene): `refuse_leak` raises before any browser or network call when a
   supplied value appears in the goal.
   Matching is a plain substring test, so a very short value (e.g. `"1"`) will refuse most goals.
5. `snapshot.js`: password inputs are observed as fillable `textbox` actions (`secret: true`, value
   always `""`, value excluded from guards); every action carries the element's `name` attribute.
   Password values are still never read into the observation.
6. `__init__.py`: no eager `Agent`/`Browser` imports, so `cli.py` can set `BU_NAME` before browser-harness loads.
7. `cli.py` (new): `python -m jev_ultrafast.cli`. Reads `{url, goal, values, profile_dir?}` on stdin,
   writes `{"type":"state",...}` lines and one final `{"type":"result", status, history, elapsed_ms,
   final_url, error?, reason?}`. Every emitted line has supplied values replaced by `***`.
   Launches its own headless Chrome (`JEV_HEADED=1` for a window) on a fresh temp `--user-data-dir`
   (or `profile_dir`), points browser-harness at it with `BU_CDP_URL` and a per-run `BU_NAME`, and
   stops the daemon, Chrome, and the temp profile afterwards.
9. `snapshot.js`: a filled password field reports `"(filled)"` as its value (never the real value), so
   Jev does not retype it forever.
10. `browser.py`: a CDP "Execution context was destroyed" during observe is a stale read (retried for
   up to 10 s instead of 200 ms), so a post-login navigation does not fail the run. An act is never retried.
11. `cli.py`: the result carries `final_page` (`url`, `title`, `snapshot`: visible text plus
   `- role "label": value` lines, redacted like every other line) so `jev_check_page` scores the Then on
   the real final page, not on the run summary.
12. `snapshot.js` + `agent.py`: actions carry the `placeholder` attribute, and `values` keys also match it.
13. `cli.py`: `final_page` marks checkboxes/radios `[checked]` / `[unchecked]` so the Then can see state.
14. `browser.py`: a supplied value ending in `\n` types the text, then presses Enter. Upstream has
   no key operation, so forms that submit only on Enter (TodoMVC) could not be completed.
15. `snapshot.js` + `agent.py`: `values` keys also match the associated `<label>` text and the input `id`.
16. `agent.py`: when Jev picks BLOCKED, `blocked_reason` records its probability and the runner-up choices.
17. `cli.py`: `final_page` shows a dropdown as one line with its selected value.
18. `agent.py`: `missing value` lists every usable key for the field; the upstream stuck-loop stop
   (3 actions without a page change) now sets `blocked_reason` too.
8. `tests/test_ywai_patches.py` (new): covers patches A, B, and the CLI refusal.

## Known limits

- Non-secret values (e.g. the email) become the field's current value once typed, and the page state
  sent to TypeSafe includes current field values. Only password fields stay unobserved.
- Needs Chrome/Chromium on PATH (or `BH_CHROME_PATH`), and `TYPESAFE_API_KEY`.
