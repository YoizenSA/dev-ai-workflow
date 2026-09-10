import { describe, expect, mock, test } from "bun:test";

import type { ChildSessionState } from "../state.js";

// The host embeds solid-js and @opentui; neither is installed in this plugin
// directory, so the TSX entry is imported dynamically after these doubles are
// registered (a static import would hoist past the mocks). This mirrors
// src/tui/index.test.ts.
const jsx = () => null;
mock.module("solid-js", () => ({
  createRoot: (fn: (dispose: () => void) => void) => fn(() => {}),
  createEffect: () => {},
  createMemo: (fn: () => unknown) => fn,
  createSignal: <T,>(value: T) => {
    let current = value;
    return [
      () => current,
      (next: T | ((prev: T) => T)) => {
        current =
          typeof next === "function" ? (next as (prev: T) => T)(current) : next;
      },
    ];
  },
  onCleanup: () => {},
  For: () => null,
  Show: () => null,
}));
mock.module("@opentui/solid", () => ({ useKeyboard: () => {} }));
mock.module("@opentui/solid/jsx-runtime", () => ({
  jsx,
  jsxs: jsx,
  Fragment: () => null,
}));
mock.module("@opentui/solid/jsx-dev-runtime", () => ({
  jsxDEV: jsx,
  Fragment: () => null,
}));

type TuiModule = typeof import("./index.js");
type V2TuiContext = Parameters<TuiModule["hydrateChildTokensFromData"]>[0];

function childSessionFixture(): ChildSessionState {
  return {
    id: "child-1",
    title: "Child",
    parentID: "parent-1",
    messageID: "message-1",
    status: "running",
    color: "yellow",
    startedAt: "2026-01-01T00:00:00.000Z",
    updatedAt: "2026-01-01T00:00:00.000Z",
  };
}

describe("normalizeV2Message", () => {
  test("maps a v2 assistant model ref onto the v1 info fields", async () => {
    const { normalizeV2Message } = await import("./index.js");

    const normalized = normalizeV2Message(
      { type: "assistant", model: { id: "m1", providerID: "p1" } },
      "s1",
    );
    const info = normalized.info as Record<string, unknown>;

    expect(info.sessionID).toBe("s1");
    expect(info.modelID).toBe("m1");
    expect(info.providerID).toBe("p1");
  });

  test("falls back to a legacy message.sessionID when no session id is passed", async () => {
    const { normalizeV2Message } = await import("./index.js");

    const normalized = normalizeV2Message({
      type: "user",
      sessionID: "legacy-session",
    });
    const info = normalized.info as Record<string, unknown>;

    expect(info.sessionID).toBe("legacy-session");
  });
});

describe("hydrateChildTokensFromData", () => {
  test("syncs each session before listing its messages", async () => {
    const { hydrateChildTokensFromData } = await import("./index.js");
    const order: string[] = [];
    const ctx = {
      data: {
        session: {
          message: {
            sync: async (sessionID: string) => {
              order.push(`sync:${sessionID}`);
            },
            list: (sessionID: string) => {
              order.push(`list:${sessionID}`);
              return [] as unknown[];
            },
          },
        },
      },
    } as unknown as V2TuiContext;

    await hydrateChildTokensFromData(ctx, childSessionFixture());

    // The v2 store does not auto-sync, so every read must sync its own session
    // first. The parent read runs only because the child carries a messageID.
    expect(order).toEqual([
      "sync:child-1",
      "list:child-1",
      "sync:parent-1",
      "list:parent-1",
    ]);
  });
});
