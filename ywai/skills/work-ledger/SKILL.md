---
name: work-ledger
description: "Compact task ledger for long-horizon work. Trigger: task spans many files/turns, needs checkpoints or recovery."
---

# Work ledger

This is the entry. Everything else is read from here, on demand.

You already have mode (`solo` | `thin` | `full`) on the orchestrator. This skill
does not invent a second table. It decides how much **task-local state** the
current work earns, then loads only that.

## Before anything non-trivial

Restate the requirement in one line, in your own words. Not for the user — for
you. Then classify.

## The gate

State the pass in one inner or ledger line, then load only what that pass needs.
Loading machinery you do not need is a failure of the gate.

| Pass | When | Load |
|---|---|---|
| **solo** | One step, or a result you can check in one glance | Nothing. Answer. |
| **thin** | Two to four steps, one deliverable, verifiable in one reading | [modules/ship.md](modules/ship.md) before anything leaves |
| **full** | Multiple stages, files, turns, or state you must carry | [modules/ledger.md](modules/ledger.md) + [modules/seams.md](modules/seams.md); ship before delivery; [modules/resume.md](modules/resume.md) after a gap |

If you cannot check the answer in one glance, it is not **solo**.

A request for brevity shortens the outer response. It never lowers verification
below the floor. Escalate the pass as soon as the work is harder than it looked.

Read [modules/gate.md](modules/gate.md) only when the classification is not obvious.

## Routing

| When | Read |
|---|---|
| You are carrying state across turns, or a third live idea needs the stage | [modules/ledger.md](modules/ledger.md) |
| A sub-task finished, a file is about to be written, or you are about to address the user | [modules/seams.md](modules/seams.md) |
| Something is about to leave (reply, handoff, commit message, export) | [modules/ship.md](modules/ship.md) |
| Compaction, summarisation, or a session gap wiped the middle | [modules/resume.md](modules/resume.md) |

Handoff and review fences stay where they already live:
`agents/sections/orchestrator-contracts-full.md`. Do not invent a second JSON shape.

## Invariants (check at seams)

1. Something was called verified without naming the verifier and what it covered.
2. A checkpoint was declared and nothing was written down.
3. Inner scratch (`// ledger:` or half-compressed notes) appears in something a person or a task-facing tool reads.
4. You called the work finished without reading the goal back.

Any hit is a finding. Name it, fix it, continue.

## Controller (optional)

`ywai ledger` records state in `.ywai/ledger.json` of the **task cwd**. It
decides nothing. Short work must not run it.

```
ywai ledger note --goal "what done means" --next "first action"
ywai ledger note --next "..."
ywai ledger note --core "live constraint"
ywai ledger note --check "what holds" --by "verifier and coverage"
ywai ledger note --open "question" --settled-by "what would settle it"
ywai ledger note --close 1 --check "what holds" --by "verifier and coverage"
ywai ledger seam
ywai ledger ship FILE
ywai ledger resume
```

Every command has a hand-executable equivalent in the modules. If the binary is
missing, restate the ledger in the conversation at each seam. The page was never
the point. Re-reading was.
