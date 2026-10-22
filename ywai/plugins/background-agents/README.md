# ywai background-agents

Supervision for OpenCode v2's **native** background subagents. OpenCode launches
the subagent, picks its model, runs it, and notifies the parent when it finishes.
This plugin adds the controls OpenCode doesn't ship.

## What is native vs. what this adds

| Concern | Owner |
|---|---|
| Launch, `background: true`, per-call `model` | native `subagent` tool |
| Completion notification (`<subagent sessionID=… state=…>` in the parent) | native |
| Result storage | native child session (no copy kept here) |
| Depth limit / anti-recursion | native (`experimental.subagent_depth`) |
| Watchdog timeout (`timeout_minutes`) | this plugin |
| `subagent_stop`, `subagent_steer`, `subagent_status`, `subagent_read` | this plugin |
| Human OS notification / toast | this plugin + `background-agents-notify` TUI sidecar |

## Tools

The id of a subagent is its native child **sessionID**, as returned by
`subagent(background: true)` and shown in its `<subagent>` notification.

| Tool | Maps to |
|---|---|
| `subagent_status(id?)` | No id: every background child of this session (status, elapsed, timeout). With id: live transcript digest from `session.context`. |
| `subagent_read(id)` | Final (or partial) assistant text read from the child session. Use after compaction drops the notification. |
| `subagent_steer(id, message)` | `session.prompt({ delivery: "steer" })` into the child's current run. Resets the timeout window. |
| `subagent_stop(id)` | `session.interrupt` on the child, then returns its partial output. Native still notifies the parent (`state="cancelled"`). |

## Timeout

`timeout_minutes` is added to the native `subagent` schema through the session
`context` hook and stripped again in `tool execute.before`, so native input
decoding never sees it. On timeout the child is interrupted like a stop.

- Default: `BACKGROUND_AGENTS_TIMEOUT_MINUTES` (15). `0` = no timeout.
- Override per call: `subagent({ ..., background: true, timeout_minutes: 45 })`.
- Applies to background runs only. Foreground calls block until they finish.

## How children are tracked

`tool execute.after` on `subagent`: a result whose metadata says
`status: "running"` is a background child. It is recorded with its parent, and
`session.wait` marks it done when its transcript shows a settled assistant turn.

The v2 plugin API has no way to list a session's children, so a small index
(`~/.local/share/opencode/delegations/<project>/subagents.json`, newest 200)
stores ids, parent, agent, description, and status. It does not store outputs:
those are always read live from the child session. Without the index,
`subagent_status` / `subagent_read` would lose children after an OpenCode restart.

## Human channel

When the host supports plugin RPC, each terminal child emits a
`delegation_terminal` event on `ywai-background-agents`. The
`ywai/plugins/tui/background-agents-notify.tsx` sidecar raises an OS notification
(only when the terminal is unfocused) and a toast.

## Also bundled

- Tool loop guard (`src/plugin/tool-loop-guard.ts`): warns when the same tool
  call repeats with identical output, and blocks repeated read-only calls.
- JSON error recovery (`src/plugin/json-error-recovery.ts`).

## Development

```bash
bun install
bun test
bunx tsc --noEmit
bun build src/plugin/background-agents.ts --outfile dist/background-agents.js --target node
```

`ywai/scripts/prepare-embedded.sh` builds the same bundle and embeds it.

## License

MIT. Not affiliated with the OpenCode team.
