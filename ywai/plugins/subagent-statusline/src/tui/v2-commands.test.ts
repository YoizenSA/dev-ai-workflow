import { describe, expect, test } from "bun:test";

import {
  COMMAND_GROUP,
  FOCUS_SIDEBAR_LIST_COMMAND,
  TOGGLE_COMPLETED_HISTORY_COMMAND,
  TOGGLE_SECTION_COMMAND,
  createSubagentKeymapLayer,
  registerSubagentCommandsV2,
  type RegisterSubagentCommandsInput,
  type V2KeymapLayer,
} from "./v2-commands.js";

function createInput(overrides: Partial<RegisterSubagentCommandsInput> = {}) {
  const calls = {
    toggleSection: [] as boolean[],
    focusSidebarList: 0,
    toggleCompletedHistory: 0,
  };
  let enabled = false;

  const input: RegisterSubagentCommandsInput = {
    api: { layer: () => undefined },
    sectionEnabled: () => enabled,
    toggleSection: (next) => {
      enabled = next;
      calls.toggleSection.push(next);
    },
    focusSidebarList: () => {
      calls.focusSidebarList += 1;
    },
    toggleCompletedHistory: () => {
      calls.toggleCompletedHistory += 1;
    },
    ...overrides,
  };

  return { input, calls };
}

describe("createSubagentKeymapLayer", () => {
  test("registers the three upstream commands under the Subagents group", () => {
    const { input } = createInput();
    const layer = createSubagentKeymapLayer(input);

    expect(layer.commands.map((command) => command.id)).toEqual([
      TOGGLE_SECTION_COMMAND,
      FOCUS_SIDEBAR_LIST_COMMAND,
      TOGGLE_COMPLETED_HISTORY_COMMAND,
    ]);
    expect(layer.commands.map((command) => command.group)).toEqual([
      COMMAND_GROUP,
      COMMAND_GROUP,
      COMMAND_GROUP,
    ]);
    expect(layer.commands.every((command) => command.palette === true)).toBe(
      true,
    );
  });

  test("binds only the focus command to alt+b", () => {
    const { input } = createInput();
    const layer = createSubagentKeymapLayer(input);
    const [toggle, focus, history] = layer.commands;

    expect(toggle?.bind).toBeUndefined();
    expect(focus?.bind).toBe("alt+b");
    expect(history?.bind).toBeUndefined();
  });

  test("runs the callbacks with the toggled section state", () => {
    const { input, calls } = createInput();
    const layer = createSubagentKeymapLayer(input);
    const [toggle, focus, history] = layer.commands;

    toggle?.run();
    // sectionEnabled was false, so the toggle enables it.
    expect(calls.toggleSection).toEqual([true]);

    toggle?.run();
    // Now enabled, so the toggle disables it.
    expect(calls.toggleSection).toEqual([true, false]);

    focus?.run();
    expect(calls.focusSidebarList).toBe(1);

    history?.run();
    expect(calls.toggleCompletedHistory).toBe(1);
  });
});

describe("registerSubagentCommandsV2", () => {
  test("passes a layer factory to the host and disposes idempotently", () => {
    let captured: (() => V2KeymapLayer) | undefined;
    let disposed = 0;
    const { input } = createInput({
      api: {
        layer: (factory) => {
          captured = factory;
          return () => {
            disposed += 1;
          };
        },
      },
    });

    const dispose = registerSubagentCommandsV2(input);
    expect(typeof captured).toBe("function");
    expect(captured?.().commands).toHaveLength(3);

    dispose();
    dispose();
    expect(disposed).toBe(1);
  });

  test("survives a host without keymap support", () => {
    const { input } = createInput({
      api: {
        layer: () => {
          throw new Error("unsupported");
        },
      },
    });

    const dispose = registerSubagentCommandsV2(input);
    expect(() => dispose()).not.toThrow();
  });

  test("no-ops when the host returns void", () => {
    const { input } = createInput({ api: { layer: () => undefined } });
    const dispose = registerSubagentCommandsV2(input);
    expect(() => dispose()).not.toThrow();
  });
});
