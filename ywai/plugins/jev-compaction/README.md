# jev-compaction

OpenCode 2 plugin that drops stale tool calls and tool results before each model
request. Jev decides which ones go, through the vendored
[fast-jev-compaction](vendor/fast-jev-compaction/VENDORED.md) library. User and
assistant text, system messages, and reasoning and media parts stay verbatim.

## How it works

1. Registers the v2 `context` session hook, which runs before every request.
2. It reapplies cached decisions, then measures the history. When it is still
   over the threshold (see Configuration) and holds non-pinned tool calls Jev
   has not decided yet, it asks Jev. The first message and the newest 6
   messages are never touched.
3. For each call, Jev's answer means one of three things: keep it, keep the
   call and truncate its result, or drop the call together with its result.
4. Decisions are cached in memory per session and reapplied on every request.
   Nothing is written to the session store; the first decision for a call wins.

## On demand

The `jev_compact` tool schedules a compaction for the session's next request,
ignoring the threshold once.

## Configuration

The threshold is `JEV_COMPACTION_THRESHOLD_TOKENS`, else
`"compactionThresholdTokens"` in the first `jev-gate.json` below that sets it,
else `200000`. Prefer the file: v2 plugins do not inherit the shell's
environment.

## Key

This plugin reads the key the same way as `jev-gate`: `TYPESAFE_API_KEY`, then
`{"apiKey": "..."}` in `<project>/.opencode/jev-gate.json`,
`~/.config/opencode/jev-gate.json` or `~/.ywai/jev-gate.json`.

## Failure behavior

It fails open. With no key, a Jev error, or a history that cannot be fitted, the
messages go through unchanged and one warning is logged per session.

## Develop

```sh
bun test
bun run build   # dist/jev-compaction.js
```
