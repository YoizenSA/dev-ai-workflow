---
name: qa-evidence
description: "Run BDD scenarios against a running app and file the proof. Trigger: executing scenarios, capturing test evidence, screenshots, reporting QA results on a work item."
---

# QA Evidence

A scenario run that leaves nothing behind is a claim, not a result. This is how to run one so the outcome survives the session: what to capture, where to put it, and how to file it against the work item.

## Pick the instrument per scenario

Read the `Then`. Whatever it asserts is what you must observe — that decides the tool, not habit.

| The `Then` asserts | Run it as | Why |
|---|---|---|
| A status code, payload, or persisted record | HTTP call | A browser adds a rendering layer between you and the assertion, and every flake it brings is yours to debug |
| Something the user sees, reaches, or is told | Browser | The rendering *is* the behaviour |
| A log line, metric, or emitted event | Trigger it any way, assert on the log/metric | The UI may look fine while the side effect never fired |

Never assert on a fixed sleep. Wait for the condition — see `condition-based-waiting`.

## Evidence layout

One directory per run, referenced by path in every report:

```
.evidence/<run-id>/
  report.md                       verdict per scenario + failure detail
  <scenario-slug>.png             the screenshot at the assertion
  <scenario-slug>.http            request and response for API scenarios
  <scenario-slug>.log             the log excerpt around a failure
```

**The screenshot goes at the moment of the assertion**, not at the end of the run. A green screen captured afterwards proves the app survived; it says nothing about the step that mattered. For a failure, capture *before* any cleanup or navigation — the state that failed is the evidence, and it is gone one click later.

Name files after the scenario, not `screenshot-1`. Six months from now the name is all anyone has.

## Logs

Scope every query to the run window. A whole day of logs is not evidence, it is a haystack.

**Local (Docker):**
```sh
docker compose logs --since 5m --no-log-prefix <service>
docker logs --since 5m <container>          # bare container
```

**Shared environment (Grafana MCP):** query Loki over the run window, filtered to the service and to error level first. Widen only if that comes back empty. Check the error-rate and latency panels for the same window — a scenario that passed while the error rate tripled is a finding too.

If nothing is containerised and there is no aggregator, say where the logs actually are rather than reporting none.

## Filing QA feedback on the work item

Results belong on the ticket, not only in the chat. Use the `ado` skill.

**Every run gets a comment on the parent work item**: the verdict line per scenario, the run id, and where the evidence lives. Markdown for comments, HTML for fields — the `ado` skill's templates reference has the rule.

```sh
ado wi comment <parent-id> --comment "<run summary>"
```

**Every failing scenario gets its own Bug**, child of the same parent:

```sh
ado wi create-child --parent <parent-id> --type Bug --title "<scenario name> fails: <what happened>"
```

One Bug per scenario, never one Bug listing five failures — they get fixed by different people at different times, and a shared ticket closes when the first one is done. The body carries: the scenario as written, expected versus actual, the evidence paths, and the log excerpt.

Do not file a Bug for a scenario that is **blocked** (the environment was down, a dependency was missing). That is not a defect in the change; report it as blocked and say what stopped it.

If work item creation is disabled for the project, put the same content in the report and say the Bugs were not filed.

## What not to do

- Do not edit, relax or skip a scenario to make it pass. It removes the signal and leaves the bug.
- Do not report `PASS` for a step you could not observe. That verdict is `BLOCKED`.
- Do not summarise away a failure ("minor rendering issue"). Record what happened; severity is someone else's call.
