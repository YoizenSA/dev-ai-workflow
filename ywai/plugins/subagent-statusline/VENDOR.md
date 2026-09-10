# Vendored: opencode-subagent-statusline (Subagent Monitor)

## Provenance

- Upstream: <https://github.com/Joaquinvesapa/sub-agent-statusline> (Subagent Monitor)
- License: MIT — the upstream `LICENSE` text is vendored verbatim in this
  directory (`./LICENSE`).
- Upstream release: **v1.3.0**
- Vendored tree: commit **070fd66354468a5843077464f47e6e2c8c3418fe**
  (commit date 2026-08-14). Note: the `package.json` inside that tree still
  reads `0.7.0`; the v1.3.0 release identifier comes from upstream releases,
  and the local `package.json` here records `1.3.0`.
- Vendored into ywai on: 2026-09-09.
- Purpose: OpenCode **v2** port of both halves. The upstream server half is a
  v1 plugin (`event` hook, `properties` payloads); it is replaced by a v2
  server plugin under `src/server/`. The upstream TUI half is ported as a
  v2-native plugin under `src/tui/` (entry, slots, palette, events, hydration,
  theme, focus). The on-disk state contract is unchanged and shared by both
  halves.

## File inventory

### Vendored unmodified (byte-identical, zero v1-API imports — verified)

Every file below was checked to import only `node:*` builtins, local `./…`
modules, or (tests only) `vitest`; none imports `@opencode-ai/plugin` or any
OpenCode SDK.

- `LICENSE`
- `src/events.ts` + `src/events.test.ts` — event reduction state machine
  (`applySubagentEvent`) consuming the v1-shaped internal events
- `src/state.ts` + `src/state.test.ts` — state.json/status.txt persistence,
  env overrides, child upserts
- `src/reconcile.ts` + `src/reconcile.test.ts` — status derivation/staleness
- `src/render.ts` + `src/render.test.ts` — statusline rendering
- `src/subagent-classification.ts` + `src/subagent-classification.test.ts`
- `src/text-width.ts` + `src/text-width.test.ts`
- `src/i18n.ts` + `src/i18n.test.ts`
- `src/logs.ts` (no co-located test upstream)
- `test/fixtures/events/*.json` — `session-created.json`, `tool-updated.json`,
  `malformed.json`

### Adapted (upstream logic, minimal mechanical change)

- `test/setup.ts` — upstream relies on vitest `globals: true` for
  `afterEach`/`vi`; this copy runs under `bun test` via `bunfig.toml`
  preload, so the hooks are imported from `bun:test`, and it additionally
  calls the harness's `useRealTime()`. Everything else is unchanged.
- `test/helpers/runtime-harness.ts` — upstream `useFrozenTime` calls
  `vi.useFakeTimers()` + `vi.setSystemTime()`; `bun:test`'s `vi` has no
  `setSystemTime` and its fake timers only advance forward while the frozen
  instants lie in the past, so the adapted copy freezes `Date` via a patched
  subclass (`useFrozenTime`/`useRealTime`). Everything else is unchanged.
- `tsconfig.json` (new file, one deviation from the `background-agents`
  precedent): `noUnusedLocals` is `false` because upstream never typechecked
  its test files (its tsconfig excludes `*.test.ts`) and one vendored test
  carries an unused type import; vendored tests must stay byte-identical.

### New (written for this port)

- `src/server/index.ts` — v2 server plugin entry. Default export
  `{ id: "subagent-statusline", setup }`; subscribes `ctx.event.subscribe()`,
  adapts, and reuses the exact upstream load → apply → save flow.
  No `@opencode/plugin` import, even as a type (structural typing only).
- `src/server/v2-event-adapter.ts` — the v2→v1 event translator (see mapping
  table below).
- `src/server/v2-event-adapter.test.ts`, `src/server/index.test.ts`
- `src/tui/tui-focus.ts` — byte-identical vendor of upstream
  `src/tui-focus.ts` (pure logic, zero imports; verified by diff)
- `src/tui/theme.ts` + `src/tui/theme.test.ts` — maps v1 `TuiThemeCurrent`
  field reads onto v2 `ResolvedTheme` (`accent`→`primary`,
  `backgroundElement`→`backgroundPanel`→`background` fallbacks)
- `src/tui/v2-commands.ts` + `src/tui/v2-commands.test.ts` — host-independent
  `keymap.layer` registration of the three upstream palette commands (same
  ids/titles/descriptions; focus command binds `alt+b`)
- `src/tui/index.tsx` + `src/tui/index.test.ts` — v2 entry
  `{ id: "subagent-statusline-tui", setup }`; claims `sidebar.content`
  (append), `home.footer` (append), `prompt.footer.status` (after, additive —
  never replaces host status); 12 v2 event types via `ctx.data.on` adapted
  through the server adapter; hydration via `ctx.data` stores;
  session open via `ctx.ui.router.navigate`; `kv`→`ctx.storage.store`;
  list keyboard kept via `useKeyboard` (focus-guarded) plus a
  list-targeted keymap layer; mouse row handlers kept; `// @ts-nocheck`
  (host-embedded `solid-js`/`@opentui/*` have no local types, same precedent
  as `ywai/plugins/tui/*.tsx`)
- `package.json`, `tsconfig.json`, `bunfig.toml`, `.gitignore`
- `test/types/vitest.d.ts` — type-only `vitest` → `bun:test` alias for tsc
- `VENDOR.md` (this file)

### Intentionally NOT vendored

- `src/index.ts` — the v1 plugin entry (replaced by `src/server/index.ts`).
- `src/tui.tsx`, `src/tui-commands.ts` — ported with v2 adaptations as
  `src/tui/index.tsx`, `src/tui/v2-commands.ts` (v1 Prompt wrappers,
  `TuiPromptRef` focus-return, sqlite/log token rehydration, and v1
  `client`/`route`/`kv`/`event`/`slots` seams have no v2 equivalent or map
  to a different API; see degradations below).
- `src/tui.test.ts`, `src/tui-maintenance.test.ts` — v1-harness-coupled;
  replaced by targeted v2-glue tests (`src/tui/*.test.ts`).
- `test/index.integration.test.ts` — integration test of the v1 entry.
- `tsup.config.ts`, upstream `vitest.config.ts`, `pnpm-*`, docs, scripts —
  build tooling superseded by the ywai plugin layout (`bun test` +
  `tsc --noEmit`, matching `ywai/plugins/background-agents`).

## v2 → internal event mapping (implemented in `src/server/v2-event-adapter.ts`)

v2 envelope is `{ id, created, type, data }` (payload in `data`); a v1-style
`properties` envelope is tolerated. Payload shapes verified against
`@opencode/protocol@0.0.0-beta` (`dist/groups/event.d.ts`).

| v2 event | Internal event emitted | Notes |
| --- | --- | --- |
| `session.created` (with `data.parentID`) | `session.created` | `data.{sessionID,parentID,title,agent}` + envelope `created` → `info.time.created`. Root sessions (no parent) are ignored, same as upstream. |
| `session.status` | `session.status` | `data.status` object (`{type: idle\|retry\|busy, …}`) passed through; the vendored reconciler already maps idle→done, busy/retry→running. |
| `session.idle` | `session.idle` | |
| `session.execution.failed` | `session.error` | `data.error` attached; session marked error. |
| `session.usage.updated` | `message.updated` (synthetic) | v2 `tokens.{input,output,reasoning}` reshaped to `usage.{input,output,reasoning,total}Tokens` so the vendored token-hint walk recognizes them; applied to the tracked child by sessionID. |
| `session.renamed` | `message.updated` (synthetic) | `data.title` → `info.title`; applied to the tracked child by sessionID. |
| `session.step.started` | `message.updated` (synthetic assistant info) | `data.model.{id,providerID,variant}` → `info.{modelID,providerID,variant}` drives `setChildModel` for the session. |
| `session.message.content.updated` | one `message.part.updated` per tool part | v2 part `{id,type:"tool",name,time,state:{status,input,metadata,content}}` → v1 part `{tool: name, state:{status,input,metadata,output,time}}`; status `streaming`→`running`; output from `state.content` text. |
| `session.tool.input.started` | `message.part.updated` (running) | Early running tool child; remembers tool-call id → name (v2 terminal tool events no longer carry the name). |
| `session.tool.called` | `message.part.updated` (running + parsed input) | Only when the id was remembered from `input.started`. |
| `session.tool.success` | `message.part.updated` (completed + output) | Only when remembered; output from `data.content`. |
| `session.tool.failed` | `message.part.updated` (error + error payload) | Only when remembered. |
| anything else | — (ignored) | See gaps below. |

### Known v2 gaps (deliberately unmapped)

- **No `subtask` parts exist in v2** (`session.message.content.updated`
  content union is `tool | text | reasoning`), so the vendored subtask branch
  only still fires through task-tool correlation.
- `session.tool.progress` — progress metadata has no internal consumer.
- `session.tool.input.delta` / `session.tool.input.ended` — the parsed input
  arrives via `session.tool.called` / content snapshots; deltas add nothing.
- `session.text.*`, `session.reasoning.*` — no internal consumer upstream.
- `session.moved`, `session.execution.started` / `.succeeded` /
  `.interrupted`, `session.retry.scheduled`, `permission.*`, `form.*`,
  `tui.*` — no internal consumer upstream (v1 never consumed permission/form
  events either).
- Context-window percent (`contextPercent` token hint): v2 usage events carry
  no context-window figure, so the compact context % display degrades to
  absent unless a later v2 payload provides it.

### TUI degradations (ported behavior, v2 has no equivalent)

- Focus-return-to-prompt after opening a child session (v1 `TuiPromptRef`):
  `focusActivePrompt` is an explicit no-op; the `home_prompt` /
  `session_prompt` Prompt wrappers are dropped.
- Token rehydration: the sqlite `execFileSync` path and the opencode log-file
  scan are deleted (no `node:child_process` import); tokens come from
  data-store messages plus usage events.

## State contract (shared by the server and TUI halves)

Unchanged from upstream — `src/state.ts` is vendored verbatim:

- State file: `state.json`; text file: `status.txt` (same directory).
- Default directory: `$XDG_RUNTIME_DIR/opencode-subagent-statusline/<instance>/`
  (fallback `os.tmpdir()`); instance defaults to `pid-<pid>`.
- Env overrides: `OPENCODE_SUBAGENT_STATUSLINE_STATE` (state path),
  `OPENCODE_SUBAGENT_STATUSLINE_INSTANCE` (instance name),
  `OPENCODE_SUBAGENT_STATUSLINE_PRESERVE_STATE=1` (keep state across
  restarts; otherwise the v2 plugin resets to an empty state on setup).
- Schema: `{ children: Record<id, ChildSessionState>, countedChildIDs,
  totalExecuted, updatedAt }`; child ids are `ses_…` (session source),
  `tool:<partID>` (tool source), `subtask:<partID>` (subtask source).
- Modes: dir `0o700`, files `0o600`, atomic writes (temp + rename).
