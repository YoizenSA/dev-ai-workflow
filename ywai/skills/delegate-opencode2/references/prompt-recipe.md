# Prompt recipe for opencode2

The agent starts cold. Everything it needs must be in the prompt or reachable
from a path the prompt names.

## Anatomy

1. **One sentence of scope**, including the branch and `do not commit`.
2. **READ FIRST** — exact file paths, with a note on what to take from each.
   Name the file whose pattern it should copy. An agent that reads the right
   three files writes code that looks like the codebase.
3. **The task**, stated as behaviour, not implementation.
4. **The traps** — every non-obvious constraint you already know. These are
   worth more than everything else combined.
5. **MUST NOT CHANGE** — the blast radius.
6. **Tests** — say what to assert. "Assert ordering, never wall-clock timing,
   no sleeps" prevents flaky tests.
7. **Verification** — when it matters, demand a live check and say plainly:
   *if you cannot run it, say so rather than reporting test results as if they
   were the live check.*
8. **Baseline** — "Baseline is 964 passed; it must stay green." A number the
   agent can check itself.

## Traps are the highest-value lines

Real examples that changed the outcome:

- *"Do NOT use `async with X.create(...)`: its `finally` calls cleanup(), which
  would destroy the cached instance on block exit."*
- *"Publish the gate into the ContextVar BEFORE the task is created — tasks copy
  the context they are created from."*
- *"`create_router()` deletes the index first, so reusing a version destroys the
  live collection. Refuse with 409."*

Each of those is a bug the agent would have shipped.

## Pin cross-boundary contracts

The costliest bug of the session: the producer wrote `execution_key`, the
consumer parsed `executionKey`. Both sides passed their own unit tests because
each was mocked against its own idea of the format.

When two jobs touch opposite ends of a wire, **write the field names once, in
both prompts, verbatim** — casing included — and demand one live round-trip.

## Failure modes seen

| Symptom | Cause |
|---|---|
| `Error: Transport` | provider failing; switch model and relaunch |
| `The user dismissed this question` | the prompt held a contradiction; the agent asked and died |
| Job wrote nothing | it stopped at the contradiction before editing |
| `uv run` fails with 401 | it re-syncs against a private feed; use the existing `.venv` |

## Reviewing

Mutation testing is the review. Break the logic on purpose:

- a guard → assert it fails
- a precedence order → assert it fails
- an error containment → assert it fails

A surviving mutation is an untested claim, not a passing test. Two real cases:
a `try/except` whose behaviour was actually guaranteed by `create_task`, and a
cache bound whose constant no test pinned.

Watch for measurement traps in your own review:

- `git stash` without `-u` leaves new files behind; the "clean" baseline is
  contaminated and the comparison is meaningless.
- Replacing a symbol across a file can break compilation, so the runner
  executes *fewer* suites and still reports green. A lower test count is not a
  pass.
