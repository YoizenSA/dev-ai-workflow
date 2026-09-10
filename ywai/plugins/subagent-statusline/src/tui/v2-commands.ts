/**
 * v2 keymap-layer registration for the Subagent Monitor command palette.
 *
 * Port of upstream `tui-commands.ts`. Upstream registered commands through a
 * v1 `keymap.registerLayer` / `command.register` shape; v2 replaces that with
 * `ctx.keymap.layer(input: () => KeymapLayer)`. This module takes the smallest
 * structural slice of that API plus the same callbacks, so it stays pure and
 * host-independent and can be unit-tested with a fake.
 */

export type TuiCommandDispose = () => void;

export type V2KeymapCommand = {
  id: string;
  title: string;
  description: string;
  group: string;
  palette: true;
  bind?: string;
  run: () => void;
};

export type V2KeymapLayer = {
  commands: V2KeymapCommand[];
};

/** Structural slice of `ctx.keymap` this module needs. */
export type V2KeymapLayerApi = {
  layer: (input: () => V2KeymapLayer) => unknown;
};

export type RegisterSubagentCommandsInput = {
  api: V2KeymapLayerApi;
  sectionEnabled: () => boolean;
  toggleSection: (enabled: boolean) => void;
  focusSidebarList: () => void;
  toggleCompletedHistory: () => void;
};

export const TOGGLE_SECTION_COMMAND =
  "subagent-statusline.toggle-sidebar-section";
export const FOCUS_SIDEBAR_LIST_COMMAND =
  "subagent-statusline.focus-sidebar-list";
export const TOGGLE_COMPLETED_HISTORY_COMMAND =
  "subagent-statusline.toggle-completed-history";
export const COMMAND_GROUP = "Subagents";

/** Same metadata as upstream `SHARED_COMMAND_METADATA.toggle`. */
export const TOGGLE_SECTION_METADATA = {
  id: TOGGLE_SECTION_COMMAND,
  title: "Subagents: Toggle sidebar section",
  description: "Toggle the entire subagent sidebar section",
  group: COMMAND_GROUP,
} as const;

/** Same metadata as upstream `SHARED_COMMAND_METADATA.focus`. */
export const FOCUS_SIDEBAR_LIST_METADATA = {
  id: FOCUS_SIDEBAR_LIST_COMMAND,
  title: "Subagents: Focus sidebar list",
  description: "Focus the subagent sidebar list for keyboard navigation",
  group: COMMAND_GROUP,
} as const;

/** Same metadata as upstream `SHARED_COMMAND_METADATA.toggleCompletedHistory`. */
export const TOGGLE_COMPLETED_HISTORY_METADATA = {
  id: TOGGLE_COMPLETED_HISTORY_COMMAND,
  title: "Subagents: Toggle completed history",
  description:
    "Toggle retained completed rows in the subagent sidebar. Shortcut: c while the sidebar list is focused.",
  group: COMMAND_GROUP,
} as const;

/**
 * Build the layer payload. Kept as a named pure function so tests can inspect
 * the commands without a live host.
 */
export function createSubagentKeymapLayer(
  input: RegisterSubagentCommandsInput,
): V2KeymapLayer {
  return {
    commands: [
      {
        id: TOGGLE_SECTION_METADATA.id,
        title: TOGGLE_SECTION_METADATA.title,
        description: TOGGLE_SECTION_METADATA.description,
        group: TOGGLE_SECTION_METADATA.group,
        palette: true,
        run: () => input.toggleSection(!input.sectionEnabled()),
      },
      {
        id: FOCUS_SIDEBAR_LIST_METADATA.id,
        title: FOCUS_SIDEBAR_LIST_METADATA.title,
        description: FOCUS_SIDEBAR_LIST_METADATA.description,
        group: FOCUS_SIDEBAR_LIST_METADATA.group,
        palette: true,
        bind: "alt+b",
        run: input.focusSidebarList,
      },
      {
        id: TOGGLE_COMPLETED_HISTORY_METADATA.id,
        title: TOGGLE_COMPLETED_HISTORY_METADATA.title,
        description: TOGGLE_COMPLETED_HISTORY_METADATA.description,
        group: TOGGLE_COMPLETED_HISTORY_METADATA.group,
        palette: true,
        run: input.toggleCompletedHistory,
      },
    ],
  };
}

/**
 * Register the three palette commands on a v2 keymap layer. The v2 contract
 * types `layer` as returning void, but some hosts hand back an unregister
 * function; when they do, this forwards it as the dispose. Otherwise the
 * layer dies with the component owner that called it.
 */
export function registerSubagentCommandsV2(
  input: RegisterSubagentCommandsInput,
): TuiCommandDispose {
  let dispose: TuiCommandDispose = () => {};
  try {
    const maybeDispose = input.api.layer(() => createSubagentKeymapLayer(input));
    if (typeof maybeDispose === "function") {
      dispose = maybeDispose as TuiCommandDispose;
    }
  } catch {
    // A host without keymap support must not crash the TUI.
  }

  let disposed = false;
  return () => {
    if (disposed) return;
    disposed = true;
    try {
      dispose();
    } catch {
      // Cleanup is best-effort.
    }
  };
}
