# Subagent Monitor → opencode v2: Port Plan

Full 1:1 port of [opencode-subagent-statusline](https://github.com/Joaquinvesapa/sub-agent-statusline)
(Subagent Monitor, MIT, upstream v1.3.0, commit `070fd66`) to the opencode v2 beta
(`opencode2`, plugin scope `@opencode/plugin@0.0.0-beta-*`), vendored into ywai and
installed by `ywai install` whenever the v2 check passes (`OpenCodeIsV2()`).

Upstream peers on `@opencode-ai/plugin >=1.14.50 <2`, so the npm package cannot load
on v2. Today ywai installs the npm package on v1 only and skips v2 (`cmd/ywai/root.go`,
the `sub-agent-statusline` gate), shipping only a minimal v2 footer
(`plugins/tui/ywai-statusline.tsx`). This port replaces that gap with the full monitor.

---

## Phase 0 — v2 API spike ✅ DONE

Authoritative sources: cached `@opencode/plugin@0.0.0-beta-19296` type defs in
`~/.bun/install/cache/` (protocol/event types), the `opencode2` beta-19381 binary,
and ywai's live v2 TUI plugins. Summary:

| v1 mechanism | v2 beta equivalent |
|---|---|
| TUI module entry | default export `{ id, setup(ctx) }`, optional cleanup return |
| TUI context | `ctx`: `data` (typed session/message/cost/status store), `keymap`, `ui.slot`, `theme`, `client`, `storage`, `renderer`, `event` |
| Sidebar | ✅ slot anchors `sidebar.content`, `sidebar.footer`, `sidebar.context` |
| Home/footer summary | ✅ `home.footer`, `prompt.footer.status` |
| Slot claims | `ui.slot({ render } + prepend\|append\|before\|after\|replace: Path)`; returns unregister fn; unpublished path degrades to nearest ancestor (additive) or is suppressed (replace) |
| Command palette | ✅ `ctx.keymap.layer()` with `palette: true` commands, optional `bind` (Alt+B) and `slash` |
| Keyboard capture while focused | ✅ keymap layer with `target: () => Renderable` |
| Mouse | ✅ opentui `onMouseDown` props + `cli.json` `"mouse": true` |
| Server `event` hook | 🔄 replaced by `ctx.event.subscribe()` async iterable; **event types renamed** (adapter required) |
| `TuiPromptRef` (focus prompt) | ❌ **no v2 equivalent** — documented degradation |
| Plugin scope | `@opencode/plugin@0.0.0-beta-*` (was `@opencode-ai/plugin`) |

Event rename map used by the server adapter: `session.error` → `session.execution.failed`;
`message.updated` → `session.message.content.updated`; `session.updated` →
`session.usage.updated` / `session.renamed` / `session.moved`; `message.part.updated` →
`session.text.*` / `session.reasoning.*` / `session.step.*`; `tool.*` →
`session.tool.input.*` + `session.tool.{called,progress,success,failed}`.
Payload shapes read from `@opencode/protocol/dist/groups/event.d.ts` in the bun cache.

Packaging rule: the host embeds `solid-js`, `@opentui/core`, `@opentui/solid` —
**never bundle peers**; externalize them in any build. Server plugins on v2 load from
`~/.config/opencode/plugins/` (auto-discovery; absolute paths in config are rejected).
TUI plugins are registered by absolute path in `cli.json` under the **`"plugin"` key
(singular)** — some ywai docs claiming `"plugins"` for cli.json are stale and get fixed
in phase 3.

## Phase 1 — Vendor + server half ✅ DONE

Created `ywai/plugins/subagent-statusline/`:

- **Vendored unmodified** (byte-identical, zero v1 API imports): `events.ts`,
  `state.ts`, `reconcile.ts`, `render.ts`, `subagent-classification.ts`,
  `text-width.ts`, `i18n.ts`, `logs.ts` + 7 co-located test files + event fixtures +
  MIT `LICENSE`. Provenance: `VENDOR.md`.
- **New**: `src/server/index.ts` (v2 `{ id: "subagent-statusline", setup }` entry),
  `src/server/v2-event-adapter.ts` (v2 → internal v1-shaped events),
  adapter + entry tests, package/tsconfig/bunfig, `test/helpers/runtime-harness.ts` adapted.
- **Evidence**: `bun test` → 148 pass / 0 fail; `tsc --noEmit` → clean.
- **Adapter gaps (documented in VENDOR.md)**: v2 has no `subtask` part type; unmapped
  (no upstream consumer): `session.tool.progress`, `session.tool.input.delta/ended`,
  `session.text.*`, `session.reasoning.*`, `session.moved`,
  `session.execution.started/succeeded/interrupted`, `session.retry.scheduled`,
  `permission.*`, `form.*`, `tui.*`; context-window % degraded (v2 usage lacks the figure).
- **State contract (frozen for the TUI phase)**: `state.json` + `status.txt` in
  `$XDG_RUNTIME_DIR/opencode-subagent-statusline/<instance>/` (default instance
  `pid-<pid>`); env overrides `OPENCODE_SUBAGENT_STATUSLINE_STATE` / `_INSTANCE` /
  `_PRESERVE_STATE=1`; dir `0o700`, files `0o600`, atomic writes; child ids
  `ses_…` / `tool:<partID>` / `subtask:<partID>`.

## Phase 2 — TUI half port ✅ DONE (2026-09-10)

Delivered as a **v2-native** plugin in `src/tui/` (not a line-for-line port —
the v1 `client`/`route`/`kv`/`event`/`slots` seams map to different v2 APIs):

- `src/tui/tui-focus.ts` — byte-identical vendor of upstream (pure, zero imports).
- `src/tui/theme.ts` — maps v1 `TuiThemeCurrent` reads onto v2 `ResolvedTheme`
  (`accent`→`primary`, `backgroundElement`→`backgroundPanel`→`background`).
- `src/tui/v2-commands.ts` — host-independent `keymap.layer` registration of
  the three palette commands (same ids/titles; focus binds `alt+b`).
- `src/tui/index.tsx` — entry `{ id: "subagent-statusline-tui", setup }`
  (solid-js root, full cleanup); slots `sidebar.content` (append),
  `home.footer` (append), `prompt.footer.status` (**after**, additive — never
  replaces host status); 12 v2 event types via `ctx.data.on` adapted through
  the server adapter; hydration via `ctx.data` stores (+ flat-list fallback);
  session open via `ctx.ui.router.navigate`; `kv`→`ctx.storage.store`;
  `useKeyboard` list handler (focus-guarded) + list-targeted keymap layer;
  mouse row handlers kept. `// @ts-nocheck` (host UI deps lack local types,
  same precedent as `ywai/plugins/tui/*.tsx`).
- Degradations (in code + `VENDOR.md`): no focus-return-to-prompt
  (`TuiPromptRef` dropped with the Prompt wrappers); no sqlite/log token
  rehydration (data-store messages + usage events only); context % may be
  absent.
- Packaging: `bun run build:tui` → `dist/subagent-statusline-tui.js`
  (166 KB; externals `solid-js`, `@opentui/core`, `@opentui/solid`,
  `@opentui/solid/*`). No `@opencode/plugin` import (verified in bundle).
- Tests: 16 new TUI-glue tests (`theme`, `v2-commands`, entry smoke with fake
  ctx: id, slot claims, palette + `alt+b`, 12 subscriptions, cleanup).
  Upstream `tui.test.ts`/`tui-maintenance.test.ts` intentionally not ported
  (v1-harness-coupled).
- Evidence: `bun test` → 164 pass / 0 fail; `tsc --noEmit` → clean.

## Phase 3 — Go installer + wiring ✅ DONE (2026-09-10)

- `scripts/prepare-embedded.sh`: bun build both halves; copy server + TUI bundles into
  `cmd/ywai/embedded_data/plugins/`.
- `internal/config/data.go`: `SubagentStatuslineServerBundlePath()` /
  `SubagentStatuslineTuiBundlePath()` resolvers (source checkout → seeded → embedded),
  matching the background-agents resolver pattern.
- `internal/plugins/subagent_statusline.go`: `InstallSubagentStatusline(configPath)`:
  - v2: server bundle → `<configdir>/plugins/subagent-statusline/` (auto-discovery,
    strip explicit entries — same semantics as `installBackgroundAgentsV2`);
    TUI bundle → `<configdir>/tui-plugins/`, register via `patchTuiPlugin`
    (cli.json `"plugin"` key), ensure `"mouse": true`.
  - Supersede: remove the `ywai-statusline` TUI entry when the full monitor installs
    (avoids duplicate footer status); keep the file removal idempotent and safe.
- `cmd/ywai/root.go`: replace the current v2 skip (~:583-592) with: v2 →
  `InstallSubagentStatusline`; v1 → unchanged npm package path.
- Update `uninstall.go` / `v1_only_cleanup.go` lists accordingly.
- Fix stale docs: cli.json key is `"plugin"` (singular) — `ywai/docs/opencode-v1-v2-switching.md`,
  `docs/src/content/docs/configuration/index.mdx`.
- Go unit tests beside the installer (`tui_logo_test.go` / `background_agents_test.go`
  patterns): v2 install idempotency, supersede behavior, config-shape assertions.

## Phase 4 — Tests, verification, delivery ⬜ TODO

- Full `bash scripts/dev.sh check` (lint → test → build-full → verify).
- Manual smoke against the installed `opencode2` beta-19381: sidebar renders,
  running/completed/failed rows, elapsed time, palette command, Alt+B, mouse toggle,
  footer summary, state files written by the server half.
- Receipt-driven review gate before any commit (user decides delivery).

## Known risks / gaps ledger

1. **Beta drift**: type defs are beta-19296, installed binary is beta-19422;
   slot catalog already drifted once (richer in the binary). Treat `Context`
   fields as stable, slot catalog as fast-moving; re-check on opencode2 upgrade.
2. **No prompt-focus equivalent** in v2 — degraded feature (phase 2).
3. **Event coverage**: subtask tracking and context-window % degraded by v2 event
   shapes (phase 1 ledger).
4. **cli.json key discrepancy** ✅ RESOLVED (2026-09-10): the plan had it
   backwards. The v2 key is `"plugins"` (plural) — verified against the
   published `https://opencode.ai/v2/cli.json` schema (no singular `plugin`
   key) and the beta-19422 migration (legacy tui.json `plugin` → `plugins`,
   `-`-prefixed disable directives). The flavor-aware installer
   (`openCodePlugins`/`writePlugins`) and both docs were already correct;
   only this plan was wrong.
5. **Upstream TUI test suite not ported 1:1** (v1 harness-coupled); v2 glue gets its
   own targeted tests instead.
