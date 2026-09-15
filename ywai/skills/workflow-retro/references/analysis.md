# Analysis — how to read the bundle without fooling yourself

Rules give the gates; this gives the method. Every item below cost a wrong conclusion once.

## One window

Title, KPIs and tables share the primary window picked in Validate. A 7d title over 30d numbers is a lie even when every number is true.

## Rework, done right

`fix:` commits are normal work under conventional commits, not rework. Quote three numbers, in this order:

1. **Reverts** — pure rework, always quote separately.
2. **Fix-on-recent** — fix commits touching a file already changed <72h earlier. True churn.
3. **Fix-share** — context only, never the headline.

File-level hotspots (`--name-only`, top 10) outrank all three: the same 3 files on top is the finding.

## Early death vs hard task

`turns<=1` with `calls<=2` means the attempt never started — auth, preflight, timeout, restart. Read the attempt `error` field FIRST. Only attempts that ran (turns well above 1) and still missed hard are evidence the task is too big. Never prescribe "split the task" for attempts that died at birth.

## Cost is noise; overhead is signal

Ignore dollar shares under ~$5/window. Instead: `tools/session`, polling loops (`delegation_status` count vs delegations), `tokens_in` concentration in one agent. Name the mechanism, not the bill.

## Join across sources

Single-source clusters are suspects; joined ones are findings:

- eval `sessionId` ↔ session analytics (did the benchmark sessions even register?)
- hotspot files ↔ fix-on-recent files (same names = confirmed churn)
- git authors (one dev or team? changes the lever: habit vs process)
- weekday/time split (weekend spikes = unattended loops misbehaving)

## Navigation

Reads concentrate on one file while the fix lives elsewhere. The fix is a pointer plus `graft_file_api` first, never more docs.

## Steering no-ops

The prompt holds the instruction and behavior never changes. Move how-to out of AGENTS.md into a skill or check. Delete text that changes nothing.

## Missing info

The attempt `error` names logs or access the agent never had. Do not blame the model. Propose tee dev logs or readonly access as the tool experiment.

## Rank, then cut

Score survivors by frequency × impact × fixability. Three Strong max is a ceiling, not a quota — two Strong beats three padded. Everything else is `Worth exploring` or `Speculative` with the next measurement named.
