import { describe, expect, mock, test } from "bun:test";

import { resolveTuiTheme } from "./theme.js";

// The host embeds solid-js and @opentui; neither is installed in this plugin
// directory, so the smoke test replaces them with minimal doubles before the
// TSX entry is imported dynamically (static imports would hoist past the mocks).
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

type SlotClaim = { render: (input: any) => unknown; [key: string]: unknown };

function createFakeContext() {
  const calls = {
    slots: [] as SlotClaim[],
    slotUnregisters: 0,
    dataOn: [] as string[],
    dataUnregisters: 0,
    keymapDispose: 0,
    toasts: [] as Array<{ message: string; variant?: string }>,
    dialogClears: 0,
    navigations: [] as unknown[],
  };
  let capturedLayer: (() => unknown) | undefined;

  const ctx = {
    location: undefined,
    theme: { primary: "#123456", textMuted: "#888" },
    renderer: { width: 40 },
    data: {
      on: (type: string, _handler: (event: unknown) => void) => {
        calls.dataOn.push(type);
        return () => {
          calls.dataUnregisters += 1;
        };
      },
      session: {
        sync: async () => {},
        family: () => [] as string[],
        root: () => "",
        get: () => undefined,
        list: () => [] as unknown[],
        status: () => "idle" as const,
        message: {
          sync: async () => {},
          list: () => [] as unknown[],
        },
      },
      location: {
        provider: { list: () => [] as unknown[] },
        model: { list: () => [] as unknown[] },
      },
    },
    keymap: {
      layer: (factory: () => unknown) => {
        capturedLayer = factory;
        return () => {
          calls.keymapDispose += 1;
        };
      },
    },
    storage: {
      store: (_key: string, options: { initial: unknown }) =>
        [options.initial, async () => {}] as const,
    },
    ui: {
      slot: (claim: unknown) => {
        calls.slots.push(claim as SlotClaim);
        return () => {
          calls.slotUnregisters += 1;
        };
      },
      toast: {
        show: (options: { message: string; variant?: string }) => {
          calls.toasts.push(options);
        },
      },
      dialog: {
        clear: () => {
          calls.dialogClears += 1;
        },
      },
      router: {
        navigate: (destination: unknown) => {
          calls.navigations.push(destination);
        },
        current: () => ({ type: "home" }),
      },
    },
  };

  return {
    ctx,
    calls,
    getCapturedLayer: () => capturedLayer,
  };
}

describe("subagent-statusline-tui entry", () => {
  test("exports the v2 module shape with the expected id", async () => {
    const mod = await import("./index.js");
    expect(mod.default.id).toBe("subagent-statusline-tui");
    expect(typeof mod.default.setup).toBe("function");
  });

  test("claims sidebar.content, home.footer and an additive prompt.footer.status", async () => {
    const mod = await import("./index.js");
    const { ctx, calls } = createFakeContext();

    const cleanup = mod.default.setup(ctx);
    try {
      const targets = calls.slots.map((claim) => ({
        placement: claim.append
          ? { append: claim.append }
          : claim.after
            ? { after: claim.after }
            : claim.prepend
              ? { prepend: claim.prepend }
              : { replace: claim.replace },
      }));

      expect(targets).toContainEqual({ placement: { append: "sidebar.content" } });
      expect(targets).toContainEqual({ placement: { append: "home.footer" } });
      expect(targets).toContainEqual({
        placement: { after: "prompt.footer.status" },
      });
      // The compact summary must not replace the host status.
      expect(calls.slots.some((claim) => "replace" in claim)).toBe(false);
      expect(calls.slots.every((claim) => typeof claim.render === "function")).toBe(
        true,
      );
    } finally {
      cleanup();
    }
  });

  test("registers the three palette commands and binds alt+b on focus", async () => {
    const mod = await import("./index.js");
    const { ctx, getCapturedLayer } = createFakeContext();

    const cleanup = mod.default.setup(ctx);
    try {
      const layer = getCapturedLayer()?.() as
        | { commands: Array<{ id: string; group: string; bind?: string }> }
        | undefined;

      expect(layer).toBeDefined();
      expect(layer?.commands.map((command) => command.id)).toEqual([
        "subagent-statusline.toggle-sidebar-section",
        "subagent-statusline.focus-sidebar-list",
        "subagent-statusline.toggle-completed-history",
      ]);
      expect(layer?.commands.every((command) => command.group === "Subagents")).toBe(
        true,
      );
      const focus = layer?.commands.find(
        (command) => command.id === "subagent-statusline.focus-sidebar-list",
      );
      expect(focus?.bind).toBe("alt+b");
    } finally {
      cleanup();
    }
  });

  test("subscribes to the v2 event set and unregisters everything on cleanup", async () => {
    const mod = await import("./index.js");
    const { ctx, calls } = createFakeContext();

    const cleanup = mod.default.setup(ctx);
    expect(calls.dataOn).toContain("session.status");
    expect(calls.dataOn).toContain("session.message.content.updated");
    expect(calls.dataOn).toHaveLength(12);
    expect(calls.slots).toHaveLength(3);

    cleanup();
    expect(calls.slotUnregisters).toBe(3);
    expect(calls.dataUnregisters).toBe(12);
    expect(calls.keymapDispose).toBe(1);
  });

  test("theme fallbacks apply to the compact/sidebar chrome", () => {
    const theme = resolveTuiTheme({ primary: "#123456", textMuted: "#888" });
    expect(theme.accent).toBe("#123456");
    expect(theme.textMuted).toBe("#888");
    expect(theme.backgroundElement).toBeUndefined();
  });
});
