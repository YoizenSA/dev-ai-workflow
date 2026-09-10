/**
 * OpenCode v2 server plugin entry for the vendored subagent statusline.
 *
 * Replaces upstream `src/index.ts` (the v1 `Plugin` with an `event` hook).
 * The v2 server plugin contract is a default export:
 *
 *   { id: string, setup: (ctx) => void | Promise<Cleanup> }
 *
 * `setup` subscribes to the v2 event stream, translates every envelope
 * through `v2-event-adapter.ts` into the v1-shaped internal events the
 * vendored core (`applySubagentEvent`) already reduces, and then reuses the
 * exact upstream persistence flow: load state → apply → if changed,
 * saveState + saveStatusText. On-disk contract (state.json + status.txt,
 * env overrides, startup reset unless OPENCODE_SUBAGENT_STATUSLINE_PRESERVE_STATE=1)
 * is identical to upstream v1 so the TUI half can be ported unchanged.
 *
 * The `@opencode/plugin` package is intentionally NOT imported, even as a
 * type: the context below is structural typing only, matching the other
 * ywai v2 plugins.
 */

import { applySubagentEvent } from "../events.js";
import { renderStatusLine } from "../render.js";
import {
  createEmptyState,
  loadState,
  resolveStatePath,
  resolveTextPath,
  saveState,
  saveStatusText,
  shouldPreserveStateOnStartup,
} from "../state.js";
import { createV2EventAdapter } from "./v2-event-adapter.js";

/** Structural slice of the v2 plugin context this plugin uses. */
type V2PluginContext = {
  event: {
    subscribe(options?: { signal?: AbortSignal }): AsyncIterable<unknown>;
  };
};

/** Structural v2 plugin cleanup: sync or async, or nothing at all. */
type V2PluginCleanup = void | (() => void) | (() => Promise<void>);

export const subagentStatuslinePluginId = "subagent-statusline";

export default {
  id: subagentStatuslinePluginId,
  async setup(ctx: V2PluginContext): Promise<V2PluginCleanup> {
    const statePath = resolveStatePath();
    const textPath = resolveTextPath(statePath);

    // Same startup contract as upstream v1: start from an empty statusline
    // unless the operator explicitly preserves state across restarts.
    if (!shouldPreserveStateOnStartup()) {
      try {
        const emptyState = createEmptyState();
        await saveState(statePath, emptyState);
        await saveStatusText(textPath, renderStatusLine(emptyState));
      } catch {
        // Defensive by design: initialization failure must not crash startup.
      }
    }

    const adapter = createV2EventAdapter();
    const controller = new AbortController();

    void (async () => {
      try {
        for await (const raw of ctx.event.subscribe({
          signal: controller.signal,
        })) {
          try {
            const internalEvents = adapter.adapt(raw);
            if (internalEvents.length === 0) continue;

            const state = await loadState(statePath);
            let changed = false;
            for (const event of internalEvents) {
              changed = applySubagentEvent(state, event) || changed;
            }
            if (changed) {
              await saveState(statePath, state);
              await saveStatusText(textPath, renderStatusLine(state));
            }
          } catch {
            // Defensive by design: one bad event must never kill the stream
            // loop (same posture as the upstream v1 event hook).
          }
        }
      } catch {
        // Stream ended or errored; the statusline keeps its last snapshot.
      }
    })();

    return () => controller.abort();
  },
};
