# jev-gate (experimental)

Jev as a **policy layer** over OpenCode 2, not as a chat model. Jev answers
questions with probabilities; this plugin turns those into review findings.

Opt-in only: `ywai install --jev-gate`, or the "Jev gate (experimental)" row in
the install TUI. That one flag installs everything the plugin needs:

- the bundle, where OpenCode auto-discovers it
- `skills/jev-gate/SKILL.md` into the shared agent skills directory
- a `ywai:jev-gate` section in the agent's `AGENTS.md`
- `~/.ywai/jev-gate.json`, from `TYPESAFE_API_KEY` when it is exported

## Tools

- `jev_review_diff` - screens the working-tree diff, then locates whatever
  screened over threshold (hunk, mechanism, severity, owner).
- `jev_review_path` - the same pipeline over a file or directory, with a
  confirmation step over 30 files and a hard stop at 80.
- `jev_find` - semantic grep: 60-line windows with 10 of overlap, scored one
  question per segment, batched 5 per request. It rejects name-only matches,
  comments and docs by criteria, not by filtering afterwards.
- `jev_route` - which agent should execute a task, chosen over the agents this
  install actually reports (`ctx.agent.list()`), with `inline`, `review` and
  `human` always available. Subagents are excluded: routing to something the
  user cannot switch to is a dead end.
- `jev_check_page` - score a Gherkin Then against an accessibility snapshot.
  Drive the page first (chrome-devtools or Playwright); Jev never clicks.

It also registers a **permission gate**. After a review with blockers, or a
route to a non-writing agent, the next write asks for confirmation. The gate
can only harden (`allow` -> `ask`): it never relaxes a permission, never
touches a `deny`, and ignores any decision that did not come from Jev.

Both return a short markdown summary; the full report goes to `ctx.storage`
under its `runId`. On OpenCode 2.0.6 they are reached through Code Mode:

```ts
return await tools.jev_review_diff({})
```

## The key

The plugin process does **not** inherit the environment of the `opencode2`
command - plugins run in a long-lived server. The key is resolved in this
order:

1. `TYPESAFE_API_KEY` in the server's environment
2. `<project>/.opencode/jev-gate.json`
3. `~/.config/opencode/jev-gate.json`
4. `~/.ywai/jev-gate.json`

File form is `{ "apiKey": "..." }`. Git-ignore it; `**/jev-gate.json` is in the
plugin's own deny list, so it is never sent to Jev even if it lands in a diff.

Without a key the tools **refuse**. A review Jev did not run must never be
reported as a clean review.

## What it does not do

No approvals (`clean` is not `approved`), no writing fixes, no findings of its
own. `compatibility` is screened but never opened as a finding: it scored
highest on a clean rename in the Fase 0 eval. See
`ywai/experiments/jev-lab/FINDINGS.md` for the measurements behind every
threshold.

## Probes

The Fase 0 host probes still ship, behind `JEV_GATE_PROBE=1`; they log tool
rosters, schema sizes and permission actions to `~/.jev-gate-spike/log.jsonl`.
