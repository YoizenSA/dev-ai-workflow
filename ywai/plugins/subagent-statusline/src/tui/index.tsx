// @ts-nocheck
/**
 * OpenCode v2 TUI half of the Subagent Monitor.
 *
 * Runs on the v2 `{ id, setup(ctx) }` TUI module contract. The pure modules
 * (`events`, `state`, `reconcile`, `render`, `text-width`, `i18n`) carry the
 * logic; only the host glue lives here:
 *
 * - Entry: default export `{ id, setup }`; reactive work runs inside a
 *   solid-js `createRoot` that `setup` returns a cleanup for.
 * - Slots: `append sidebar.content`, `append home.footer`, and an additive
 *   `after prompt.footer.status` compact summary (never replaces host status).
 * - Events: v2 `ctx.data.on(type, handler)` fed through the server half's
 *   `createV2EventAdapter()` into `applySubagentEvent`.
 * - Hydration: defensive reads over `ctx.data` stores instead of the v1 client.
 * - Storage: `ctx.storage.store` replaces v1 kv reads/writes.
 *
 * Documented degradations:
 * - Focus-return-to-prompt: v1 used `TuiPromptRef`; v2 has no equivalent, so
 *   `focusActivePrompt` is a no-op (the `home_prompt`/`session_prompt` Prompt
 *   wrappers are dropped with it).
 * - Context percent: v2 usage payloads carry no context-window figure, so the
 *   compact `%` display may be absent.
 * - Token rehydration: the sqlite shell-out and opencode log-file scan are
 *   deleted; tokens come from data-store messages and usage events only.
 *
 * `@ts-nocheck` matches the repo precedent for TUI files: the host embeds
 * solid-js / @opentui, and those packages are not resolvable from this plugin
 * directory, so their types are unavailable locally.
 */
/** @jsxImportSource @opentui/solid */
import type {
  BoxRenderable,
  KeyEvent,
  MouseEvent,
  ScrollBoxRenderable,
} from "@opentui/core";
import { useKeyboard } from "@opentui/solid";
import { appendFileSync, mkdirSync } from "node:fs";
import { createRequire } from "node:module";
import os from "node:os";
import { dirname, join } from "node:path";
import {
  For,
  Show,
  createRoot,
  createEffect,
  createMemo,
  createSignal,
  onCleanup,
} from "solid-js";
import type { Accessor } from "solid-js";

import {
  applySubagentEvent,
  extractChildDetails,
  extractLatestAssistantModel,
  extractTaskToolEvidence,
} from "../events.js";
import {
  byPriority,
  formatDuration,
  renderStatusLine,
  visibleSubagentWorkItems,
} from "../render.js";
import {
  canSafelyCloseNoTargetPersistedCandidate,
  capCandidates,
  deriveOpenCodeSessionStatus,
  hasRecentMessageActivity,
  nextBackoffState,
  parseStaleRunningThresholdMs as parseConfiguredStaleRunningThresholdMs,
  resolvePersistedStaleSubtaskFromParentMessages,
  resolveSessionStatusWithMessageSummary,
  shouldApplyStaleRunningFallback,
  shouldSkipCandidateForBackoff,
  summarizeSessionMessages,
  type PersistedStaleSubtaskCandidate,
  type RunningReconcileCacheEntry,
  type RunningReconcileEvidence,
  type SessionMessageSummary,
} from "../reconcile.js";
import {
  createEmptyState,
  countCountedSubagentExecutions,
  countHistoricalSubagentExecutions,
  countRetainedSubagentStatuses,
  markChildStatus,
  refreshDerivedFields,
  resolveStatePath,
  resolveTextPath,
  saveState,
  saveStatusText,
  setChildModel,
  upsertChildDetails,
  type ChildTokenState,
  type ChildSessionState,
  type StatusCounts,
  type StatuslineState,
} from "../state.js";
import { takeColumns, textColumns, truncateToColumns } from "../text-width.js";
import { createV2EventAdapter } from "../server/v2-event-adapter.js";
import {
  resolveSidebarReturnFocusAction,
  resolveSiblingSidebarRefocus,
  shouldReleaseSidebarListFocus,
  type PendingSidebarRefocus,
} from "./tui-focus.js";
import {
  COMMAND_GROUP,
  registerSubagentCommandsV2,
  type TuiCommandDispose,
  type V2KeymapLayerApi,
} from "./v2-commands.js";
import { resolveTuiTheme, type TuiTheme } from "./theme.js";
import { t } from "../i18n.js";

const TUI_PLUGIN_ID = "subagent-statusline-tui";
const ELAPSED_TICK_MS = 1000;
const FALLBACK_SIDEBAR_WIDTH = 34;
const MIN_ROW_WIDTH = 24;
const MIN_LABEL_WIDTH = 8;
const MAINTENANCE_TICK_MS = 2000;
const HYDRATE_RETRY_BASE_DELAY_MS = 1000;
const HYDRATE_RETRY_MAX_DELAY_MS = 30_000;
const HYDRATE_RETRY_MAX_ATTEMPTS = 6;
const RUNNING_RECONCILE_MAINTENANCE_INTERVAL_MS = 10 * 60_000;
const RUNNING_RECONCILE_MAX_CANDIDATES = 8;
const RUNNING_RECONCILE_INITIAL_BACKOFF_MS = 15_000;
const RUNNING_RECONCILE_MAX_BACKOFF_MS = 5 * 60_000;
const RUNNING_RECONCILE_MESSAGE_AGE_GATE_MS = 60_000;
const RUNNING_RECONCILE_OLD_CANDIDATE_AGE_MS = 5 * 60_000;
const CLOCK_ICON = "";
const TOKEN_ICON = "";
const SIDEBAR_ARROW_EXPANDED = "▼";
const SIDEBAR_ARROW_COLLAPSED = "▶";
const SUBAGENTS_EXPANDED_KV_KEY = "subagents.sidebar.expanded";
const SUBAGENTS_SECTION_ENABLED_KV_KEY = "subagents.sidebar.enabled";
const SUBAGENTS_MAX_VISIBLE_ROWS = 5;
const SUBAGENTS_RUNNING_ROW_HEIGHT = 3;
const SUBAGENTS_TERMINAL_ROW_HEIGHT = 2;
const SUBAGENTS_MODEL_ROW_HEIGHT = 1;
const SUBAGENTS_ROW_GAP = 0;
const SUBAGENTS_ROW_MARKER_WIDTH = 4;
const SUBAGENTS_MAX_LIST_HEIGHT =
  SUBAGENTS_MAX_VISIBLE_ROWS *
    (SUBAGENTS_RUNNING_ROW_HEIGHT + SUBAGENTS_MODEL_ROW_HEIGHT) +
  (SUBAGENTS_MAX_VISIBLE_ROWS - 1) * SUBAGENTS_ROW_GAP;
const INACTIVE_SUBAGENT_OPACITY = 0.65;
const SIDEBAR_VERSION_OPACITY = 0.7;
const SIDEBAR_FOCUS_INDICATOR = "●";

const V2_SUBSCRIBED_EVENTS = [
  "session.created",
  "session.status",
  "session.idle",
  "session.execution.failed",
  "session.usage.updated",
  "session.renamed",
  "session.step.started",
  "session.message.content.updated",
  "session.tool.input.started",
  "session.tool.called",
  "session.tool.success",
  "session.tool.failed",
];

const packageRequire = createRequire(import.meta.url);

function readPluginVersion(): string | undefined {
  try {
    const metadata = packageRequire("../package.json") as { version?: unknown };
    return typeof metadata.version === "string" && metadata.version.length > 0
      ? metadata.version
      : undefined;
  } catch {
    return undefined;
  }
}

const PLUGIN_VERSION = readPluginVersion();

/**
 * Structural slice of the v2 TUI context this half uses. The `@opencode/plugin`
 * package is never imported, even as a type: the contract is matched
 * structurally, and `@ts-nocheck` keeps the missing host types out of the way.
 */
type V2TuiContext = {
  location?: { directory?: string } | undefined;
  theme: unknown;
  renderer?: unknown;
  data: {
    on: (type: string, handler: (event: unknown) => void) => () => void;
    session: {
      sync: (sessionID: string) => Promise<void>;
      family: (sessionID: string) => string[];
      root: (sessionID: string) => string;
      get: (sessionID: string) => unknown;
      list: () => unknown[];
      status: (sessionID: string) => "idle" | "running";
      message: {
        sync: (sessionID: string) => Promise<void>;
        list: (sessionID: string) => unknown[];
      };
    };
    location: {
      provider: { list: () => unknown[] | undefined };
      model: { list: () => unknown[] | undefined };
    };
  };
  keymap: V2KeymapLayerApi & { layer: (input: () => unknown) => unknown };
  storage: {
    store: (
      key: string,
      options: { initial: unknown },
    ) => readonly [unknown, ((mutation: (draft: unknown) => void) => Promise<void>) | undefined];
  };
  ui: {
    slot: (claim: unknown) => () => void;
    toast: { show: (options: { message: string; variant?: string }) => void };
    dialog: { clear: () => void };
    router: {
      navigate: (destination: unknown) => void;
      current: () => { type?: string; sessionID?: string } | undefined;
    };
  };
};

interface SidebarScrollRegistration {
  getScrollbox: () => ScrollBoxRenderable | undefined;
  getAnchor: () => SidebarScrollAnchor | undefined;
  getRows: () => SidebarScrollRowLayout[];
  getLeadingHeight: () => number;
  offsetTop: number;
  anchor?: SidebarScrollAnchor;
  restoreFramesRemaining: number;
}

export interface SidebarScrollAnchor {
  childIDs: string[];
  intraRowOffset: number;
}

export interface SidebarScrollRowLayout {
  id: string;
  height: number;
}

interface SidebarListFocusRegistration {
  focusList: (preferredChildID?: string) => boolean;
  blurList: () => boolean;
  isListFocusModeActive: () => boolean;
}

interface SidebarCompletedHistoryRegistration {
  toggleCompletedHistory: () => boolean;
}

const sidebarScrollRegistrations = new Set<SidebarScrollRegistration>();
const sidebarListFocusRegistrations = new Set<SidebarListFocusRegistration>();
const sidebarCompletedHistoryRegistrations =
  new Set<SidebarCompletedHistoryRegistration>();
const SIDEBAR_SCROLL_RESTORE_FRAME_BUDGET = 2;

function focusVisibleSidebarSubagentList(preferredChildID?: string): boolean {
  for (const registration of [...sidebarListFocusRegistrations].reverse()) {
    if (registration.focusList(preferredChildID)) return true;
  }
  return false;
}

function blurVisibleSidebarSubagentList(): boolean {
  for (const registration of [...sidebarListFocusRegistrations].reverse()) {
    if (registration.blurList()) return true;
  }
  return false;
}

function isAnySidebarSubagentListFocused(): boolean {
  return [...sidebarListFocusRegistrations].some((registration) =>
    registration.isListFocusModeActive(),
  );
}

function toggleVisibleSidebarCompletedHistory(): boolean {
  for (const registration of [
    ...sidebarCompletedHistoryRegistrations,
  ].reverse()) {
    if (registration.toggleCompletedHistory()) return true;
  }
  return false;
}

function maxScrollTop(scrollbox: ScrollBoxRenderable): number {
  return Math.max(0, scrollbox.scrollHeight - scrollbox.viewport.height);
}

function clampedScrollTop(
  scrollbox: ScrollBoxRenderable,
  value: number,
): number {
  return Math.max(0, Math.min(value, maxScrollTop(scrollbox)));
}

function snapshotSidebarScrollOffsets(): void {
  for (const registration of sidebarScrollRegistrations) {
    const scrollbox = registration.getScrollbox();
    if (!scrollbox) continue;
    registration.offsetTop = clampedScrollTop(scrollbox, scrollbox.scrollTop);
    registration.anchor = registration.getAnchor();
    registration.restoreFramesRemaining = SIDEBAR_SCROLL_RESTORE_FRAME_BUDGET;
  }
}

function resolveSidebarAnchorScrollTop(input: {
  expanded: boolean;
  anchor?: SidebarScrollAnchor;
  rows: SidebarScrollRowLayout[];
  leadingHeight: number;
  scrollTop: number;
  scrollHeight: number;
  viewportHeight: number;
}): { matched: boolean; offsetTop?: number; scrollTop?: number } {
  if (!input.expanded || !input.anchor || input.anchor.childIDs.length === 0) {
    return { matched: false };
  }

  let top = input.leadingHeight;
  const rowTops = new Map<string, number>();
  for (const row of input.rows) {
    rowTops.set(row.id, top);
    top += row.height + SUBAGENTS_ROW_GAP;
  }

  for (const [index, childID] of input.anchor.childIDs.entries()) {
    const rowTop = rowTops.get(childID);
    if (rowTop === undefined) continue;

    const desiredTop = rowTop + (index === 0 ? input.anchor.intraRowOffset : 0);
    const maxTop = Math.max(0, input.scrollHeight - input.viewportHeight);
    const nextTop = Math.max(0, Math.min(desiredTop, maxTop));
    return {
      matched: true,
      offsetTop: nextTop,
      scrollTop: input.scrollTop !== nextTop ? nextTop : undefined,
    };
  }

  return { matched: false };
}

export function preservedSidebarAnchorScrollTop(input: {
  expanded: boolean;
  anchor?: SidebarScrollAnchor;
  rows: SidebarScrollRowLayout[];
  leadingHeight?: number;
  scrollTop: number;
  scrollHeight: number;
  viewportHeight: number;
}): number | undefined {
  return resolveSidebarAnchorScrollTop({
    ...input,
    leadingHeight: input.leadingHeight ?? 0,
  }).scrollTop;
}

export function preservedSidebarScrollTop(input: {
  expanded: boolean;
  offsetTop: number;
  anchor?: SidebarScrollAnchor;
  rows?: SidebarScrollRowLayout[];
  leadingHeight?: number;
  scrollTop: number;
  scrollHeight: number;
  viewportHeight: number;
}): number | undefined {
  if (!input.expanded) return undefined;

  const anchorTop = resolveSidebarAnchorScrollTop({
    expanded: input.expanded,
    anchor: input.anchor,
    rows: input.rows ?? [],
    leadingHeight: input.leadingHeight ?? 0,
    scrollTop: input.scrollTop,
    scrollHeight: input.scrollHeight,
    viewportHeight: input.viewportHeight,
  });
  if (anchorTop.matched) return anchorTop.scrollTop;

  const maxTop = Math.max(0, input.scrollHeight - input.viewportHeight);
  const top = Math.max(0, Math.min(input.offsetTop, maxTop));
  return top > 0 && input.scrollTop !== top ? top : undefined;
}

interface RunningReconcileCandidate {
  childID: string;
  targetSessionID?: string;
  parentID?: string;
  messageID?: string;
  source?: ChildSessionState["source"];
  title?: string;
  summary?: string;
  agentName?: string;
  startedMs: number;
  updatedMs: number;
}

function debugLog(input: Record<string, unknown>): void {
  if (!process.env.OPENCODE_SUBAGENT_STATUSLINE_DEBUG_EVENTS) return;
  try {
    const path = join(
      process.env.XDG_RUNTIME_DIR ?? os.tmpdir(),
      "opencode-subagent-statusline",
      "tui-events.log",
    );
    mkdirSync(dirname(path), { recursive: true });
    const line = JSON.stringify({ time: new Date().toISOString(), ...input });
    appendFileSync(path, `${line}\n`, "utf8");
  } catch {
    // Debug logging must never crash the TUI.
  }
}

function debugEvent(event: unknown): void {
  const e = event as {
    type?: unknown;
    properties?: { sessionID?: unknown; part?: unknown; info?: unknown };
  };
  const part = e?.properties?.part as
    | { type?: unknown; tool?: unknown; state?: { status?: unknown } }
    | undefined;
  debugLog({
    kind: "event",
    type: e?.type,
    sessionID: e?.properties?.sessionID,
    partType: part?.type,
    tool: part?.tool,
    toolStatus: part?.state?.status,
  });
}

function cloneState(state: StatuslineState): StatuslineState {
  return {
    updatedAt: state.updatedAt,
    totalExecuted: state.totalExecuted,
    countedChildIDs: { ...state.countedChildIDs },
    children: Object.fromEntries(
      Object.entries(state.children).map(([id, child]) => [
        id,
        {
          ...child,
          tokens: child.tokens ? { ...child.tokens } : undefined,
          model: child.model ? { ...child.model } : undefined,
        },
      ]),
    ),
  };
}

function mergeTokenState(
  existing: ChildTokenState | undefined,
  incoming: ChildTokenState | undefined,
): ChildTokenState | undefined {
  if (!existing && !incoming) return undefined;
  return {
    input: incoming?.input ?? existing?.input,
    output: incoming?.output ?? existing?.output,
    total: incoming?.total ?? existing?.total,
    contextPercent: incoming?.contextPercent ?? existing?.contextPercent,
  };
}

function hasTokenTotal(tokens: ChildTokenState | undefined): boolean {
  return typeof tokens?.total === "number" && Number.isFinite(tokens.total);
}

function sameTokens(
  left: ChildTokenState | undefined,
  right: ChildTokenState | undefined,
): boolean {
  return JSON.stringify(left) === JSON.stringify(right);
}

function asRecord(value: unknown): Record<string, unknown> | undefined {
  return value && typeof value === "object"
    ? (value as Record<string, unknown>)
    : undefined;
}

function asString(value: unknown): string | undefined {
  return typeof value === "string" && value.length > 0 ? value : undefined;
}

function safeRead<Value>(read: () => Value): Value | undefined {
  try {
    return read();
  } catch {
    return undefined;
  }
}

async function safeReadAsync<Value>(
  read: () => Promise<Value>,
): Promise<Value | undefined> {
  try {
    return await read();
  } catch {
    return undefined;
  }
}

function messageIDOf(message: unknown): string | undefined {
  const record = asRecord(message);
  if (!record) return undefined;
  const id = record.id ?? record.messageID ?? record.messageId;
  return typeof id === "string" && id.length > 0 ? id : undefined;
}

/** Join the text entries of a v2 content array into one output string. */
function contentText(value: unknown): string | undefined {
  if (!Array.isArray(value)) return undefined;
  const texts: string[] = [];
  for (const entry of value) {
    const record = asRecord(entry);
    if (record?.type === "text" && typeof record.text === "string") {
      texts.push(record.text);
    }
  }
  return texts.length > 0 ? texts.join("\n") : undefined;
}

/** Normalize a v2 tool state into the v1 `part.state` shape. */
function normalizeToolState(rawState: unknown): Record<string, unknown> {
  const state = asRecord(rawState) ?? {};
  const time = asRecord(state.time);
  const hasTime =
    time &&
    (typeof time.created === "number" || typeof time.completed === "number");
  return {
    status: asString(state.status),
    input: asRecord(state.input) ?? {},
    metadata: asRecord(state.metadata),
    output: contentText(state.content) ?? asString(state.output),
    error: state.error,
    time: hasTime
      ? {
          created: time.created,
          completed: time.completed,
          updated: time.completed ?? time.created,
        }
      : undefined,
  };
}

function normalizeContentPart(
  rawPart: unknown,
): Record<string, unknown> | undefined {
  const part = asRecord(rawPart);
  if (!part) return undefined;
  const type = asString(part.type);
  if (type === "tool") {
    return {
      type: "tool",
      tool: asString(part.name),
      id: asString(part.id),
      state: normalizeToolState(part.state),
      time: asRecord(part.time),
    };
  }
  return {
    type,
    id: asString(part.id),
    text: part.text,
    time: asRecord(part.time),
  };
}

/**
 * Normalize one v2 `SessionMessageInfo` into the v1 `{ info, parts }` shape the
 * reconcile helpers already understand.
 *
 * The v2 protocol carries no `sessionID` on messages (verified against
 * `@opencode/protocol` `message.d.ts`), so the caller passes the known session
 * id and the normalizer keeps it in `info.sessionID`. This is what
 * `extractLatestAssistantModel` needs to attribute a model to a child session.
 * The assistant `model` is a `ModelRef` `{ id, providerID, variant? }`, so
 * `info.modelID` is read from `model.id`.
 *
 * Defensive: unknown message types become an empty user message instead of
 * throwing.
 */
export function normalizeV2Message(
  raw: unknown,
  sessionID?: string,
): Record<string, unknown> {
  const message = asRecord(raw);
  if (!message) return { info: {}, parts: [] };
  const type = asString(message.type);
  const time = asRecord(message.time);
  const id = asString(message.id);
  const fallbackSessionID = asString(message.sessionID) ?? sessionID;

  if (type === "assistant") {
    const model = asRecord(message.model);
    const content = Array.isArray(message.content) ? message.content : [];
    return {
      info: {
        id,
        role: "assistant",
        sessionID: fallbackSessionID,
        modelID: asString(model?.id),
        providerID: asString(model?.providerID),
        variant: asString(model?.variant),
        error: message.error,
        time: time
          ? {
              created: time.created,
              completed: time.completed,
              updated: time.completed ?? time.streamed,
            }
          : undefined,
      },
      parts: content.map(normalizeContentPart).filter(Boolean),
    };
  }

  return {
    info: {
      id,
      role: type,
      sessionID: fallbackSessionID,
      time,
    },
    parts: [],
  };
}

/** Normalize a v2 `SessionInfo` into the fields hydration reads. */
function normalizeSessionInfo(
  raw: unknown,
): Record<string, unknown> | undefined {
  const session = asRecord(raw);
  if (!session) return undefined;
  const id = asString(session.id);
  if (!id) return undefined;
  return {
    id,
    parentID: asString(session.parentID),
    title: asString(session.title),
    agent: asString(session.agent),
    time: asRecord(session.time),
    model: asRecord(session.model),
  };
}

/**
 * Read + prime one session's messages from `ctx.data`. Returns `undefined` when
 * the store is unavailable, which callers treat as a failed read (never a
 * reason to crash).
 */
async function readSessionMessages(
  ctx: V2TuiContext,
  sessionID: string,
): Promise<unknown[] | undefined> {
  const synced = await safeReadAsync(async () => {
    await ctx.data.session.message.sync(sessionID);
    return true;
  });
  if (!synced) return undefined;
  const list = safeRead(() => ctx.data.session.message.list(sessionID));
  return Array.isArray(list) ? list : [];
}

/**
 * Token hydration from data-store messages only. The v1 sqlite shell-out and
 * opencode log scan are deleted (see the file header): v2 usage events feed
 * tokens directly and the store carries the rest. Context percent may be absent.
 *
 * The v2 client store does not auto-sync before reads: `session.message.list`
 * returns only what events and explicit `session.message.sync` calls populated
 * (verified in `@opencode/client` `solid/data.d.ts`). Every session is synced
 * before listing, reusing `readSessionMessages`, the file's sync-before-list
 * pattern.
 */
export async function hydrateChildTokensFromData(
  ctx: V2TuiContext,
  child: ChildSessionState,
): Promise<ChildTokenState | undefined> {
  const candidates: unknown[] = [];

  const messages = (await readSessionMessages(ctx, child.id)) ?? [];
  for (const message of messages) {
    candidates.push(normalizeV2Message(message, child.id));
  }

  if (child.messageID) {
    const parentMessages =
      (await readSessionMessages(ctx, child.parentID)) ?? [];
    const parentMessage = parentMessages.find(
      (message) => messageIDOf(message) === child.messageID,
    );
    if (parentMessage) {
      candidates.push(normalizeV2Message(parentMessage, child.parentID));
    }
  }

  let tokens: ChildTokenState | undefined;
  for (const candidate of candidates) {
    tokens = mergeTokenState(
      tokens,
      extractChildDetails(
        candidate as Parameters<typeof extractChildDetails>[0],
      ).tokens,
    );
  }

  return tokens;
}

/**
 * Hydrate token deltas for every eligible child, mutating `state` in place.
 * The data-store reads are async (sync-before-list), so a fetch phase runs
 * first and a synchronous merge phase applies the results. Returns whether
 * any child token state changed.
 */
async function hydrateStateTokensFromData(
  ctx: V2TuiContext,
  state: StatuslineState,
): Promise<boolean> {
  const fetched = new Map<string, ChildTokenState | undefined>();
  for (const child of Object.values(state.children)) {
    if (child.status !== "running" && hasTokenTotal(child.tokens)) continue;
    fetched.set(child.id, await hydrateChildTokensFromData(ctx, child));
  }

  let changed = false;
  for (const [childID, hydrated] of fetched) {
    const child = state.children[childID];
    if (!child) continue;
    const nextTokens = mergeTokenState(child.tokens, hydrated);
    if (!sameTokens(child.tokens, nextTokens)) {
      child.tokens = nextTokens;
      child.updatedAt = new Date().toISOString();
      changed = true;
    }
  }

  if (changed) {
    state.updatedAt = new Date().toISOString();
    debugLog({
      kind: "state.tokens.hydrated",
      children: Object.values(state.children).map((child) => ({
        id: child.id,
        title: child.title,
        tokens: child.tokens,
      })),
    });
  }

  return changed;
}

/**
 * Merge only the token deltas a probe computed onto a live state clone. This
 * is the safe way to apply async hydration results: the live state is never
 * replaced by the probe, so events that land during the reads survive.
 */
function mergeHydratedTokens(
  target: StatuslineState,
  source: StatuslineState,
): boolean {
  let changed = false;
  for (const child of Object.values(target.children)) {
    const sourceChild = source.children[child.id];
    if (!sourceChild?.tokens) continue;
    const nextTokens = mergeTokenState(child.tokens, sourceChild.tokens);
    if (!sameTokens(child.tokens, nextTokens)) {
      child.tokens = nextTokens;
      child.updatedAt = new Date().toISOString();
      changed = true;
    }
  }
  if (changed) target.updatedAt = new Date().toISOString();
  return changed;
}

function persistStateSnapshot(
  statePath: string,
  textPath: string,
  state: StatuslineState,
): void {
  const snapshot = cloneState(state);
  void (async () => {
    try {
      await saveState(statePath, snapshot);
      await saveStatusText(textPath, renderStatusLine(snapshot));
    } catch {
      // Persistence is best-effort; TUI rendering must not fail because of files.
    }
  })();
}

function refreshLiveState(state: StatuslineState): boolean {
  const beforeChildIDs = new Set(Object.keys(state.children));
  refreshDerivedFields(state);

  if (Object.keys(state.children).length !== beforeChildIDs.size) {
    return true;
  }

  for (const childID of beforeChildIDs) {
    if (!state.children[childID]) return true;
  }

  return false;
}

/**
 * One maintenance pass: hydrate tokens from the data store, then refresh the
 * derived fields. Returns the mutated clone, or `current` when nothing
 * changed. The data-store reads are async (sync-before-list).
 */
export async function runTuiStateMaintenance(
  ctx: V2TuiContext,
  current: StatuslineState,
): Promise<StatuslineState> {
  const next = cloneState(current);
  const hydrated = await hydrateStateTokensFromData(ctx, next);
  const refreshed = refreshLiveState(next);
  return hydrated || refreshed ? next : current;
}

export function createTuiMaintenanceTimers(input: {
  onElapsedTick: () => void;
  onMaintenanceTick: () => void;
}): {
  syncElapsedTimer: (hasRunningChild: boolean) => void;
  dispose: () => void;
} {
  let elapsedTimer: ReturnType<typeof setInterval> | undefined;
  const maintenanceTimer = setInterval(
    input.onMaintenanceTick,
    MAINTENANCE_TICK_MS,
  );

  return {
    syncElapsedTimer(hasRunningChild) {
      if (hasRunningChild && !elapsedTimer) {
        elapsedTimer = setInterval(input.onElapsedTick, ELAPSED_TICK_MS);
      } else if (!hasRunningChild && elapsedTimer) {
        clearInterval(elapsedTimer);
        elapsedTimer = undefined;
      }
    },
    dispose() {
      if (elapsedTimer) clearInterval(elapsedTimer);
      clearInterval(maintenanceTimer);
      elapsedTimer = undefined;
    },
  };
}

function elapsedMs(child: ChildSessionState, nowMs: number): number {
  if (child.status !== "running") {
    return child.elapsedMs ?? 0;
  }
  const started = Date.parse(child.startedAt);
  if (Number.isNaN(started)) return child.elapsedMs ?? 0;
  return Math.max(0, nowMs - started);
}

function taskStatusMarker(status: ChildSessionState["status"]): string {
  if (status === "done") return "[✓]";
  if (status === "error") return "[x]";
  return "[ ]";
}

function statusColor(
  status: ChildSessionState["status"],
  theme: TuiTheme,
): string {
  if (status === "done") return theme.success;
  if (status === "error") return theme.error;
  return theme.warning;
}

function isSessionTarget(value: unknown): value is string {
  return typeof value === "string" && value.startsWith("ses_");
}

function resolveChildTargetSessionID(
  child: ChildSessionState,
): string | undefined {
  if (isSessionTarget(child.targetSessionID)) {
    return child.targetSessionID;
  }
  if (child.id.startsWith("ses_")) {
    return child.id;
  }
  return undefined;
}

function resolveSyntheticTargetFromHydratedState(
  state: StatuslineState,
  synthetic: ChildSessionState,
): string | undefined {
  const messageMatches = Object.values(state.children).filter(
    (candidate) =>
      candidate.id.startsWith("ses_") &&
      candidate.parentID === synthetic.parentID &&
      synthetic.messageID &&
      candidate.messageID === synthetic.messageID,
  );
  if (messageMatches.length === 1) return messageMatches[0].id;

  const parentMatches = Object.values(state.children).filter(
    (candidate) =>
      candidate.id.startsWith("ses_") &&
      candidate.parentID === synthetic.parentID,
  );
  if (parentMatches.length === 1) return parentMatches[0].id;

  return undefined;
}

export function backfillHydratedTargetSessionIDs(
  state: StatuslineState,
  parentSessionID: string,
): boolean {
  let changed = false;

  for (const child of Object.values(state.children)) {
    if (child.parentID !== parentSessionID) continue;
    if (resolveChildTargetSessionID(child)) continue;
    if (child.source === "session" || child.id.startsWith("ses_")) {
      child.targetSessionID = child.id;
      changed = true;
      continue;
    }

    const syntheticTarget = resolveSyntheticTargetFromHydratedState(
      state,
      child,
    );
    if (syntheticTarget) {
      child.targetSessionID = syntheticTarget;
      changed = true;
    }
  }

  if (changed) {
    state.updatedAt = new Date().toISOString();
  }

  return changed;
}

function navigateToSessionTarget(
  ctx: V2TuiContext,
  targetSessionID: string | undefined,
): void {
  if (!isSessionTarget(targetSessionID)) return;
  // v2 router: a typed destination rather than v1 `route.navigate(name, params)`.
  safeRead(() =>
    ctx.ui.router.navigate({ type: "session", sessionID: targetSessionID }),
  );
}

function toFinitePositiveInt(value: unknown): number | undefined {
  if (typeof value !== "number" || !Number.isFinite(value)) return undefined;
  const rounded = Math.floor(value);
  return rounded > 0 ? rounded : undefined;
}

function parseStaleRunningThresholdMs(): number {
  return parseConfiguredStaleRunningThresholdMs(
    process.env.OPENCODE_SUBAGENT_STATUSLINE_STALE_RUNNING_MS,
  );
}

const STALE_RUNNING_THRESHOLD_MS = parseStaleRunningThresholdMs();

function resolveSidebarWidth(ctx: unknown): number | undefined {
  const source = asRecord(ctx);
  if (!source) return undefined;

  const direct =
    toFinitePositiveInt(source.width) ??
    toFinitePositiveInt(source.columns) ??
    toFinitePositiveInt(source.cols);
  if (direct) return direct;

  const size = asRecord(source.size);
  const viewport = asRecord(source.viewport);
  const bounds = asRecord(source.bounds);

  return (
    toFinitePositiveInt(size?.width) ??
    toFinitePositiveInt(viewport?.width) ??
    toFinitePositiveInt(bounds?.width)
  );
}

function ellipsize(value: string, maxColumns: number): string {
  return truncateToColumns(value, maxColumns);
}

function splitParentheticalTitle(title: string): {
  label: string;
  parenthetical?: string;
} {
  const match = title.match(/^(.*?)\s*(\([^)]*\))\s*$/);
  if (!match) return { label: title };

  const label = match[1]?.trim();
  const parenthetical = match[2]?.trim();
  if (!label || !parenthetical) return { label: title };

  return { label, parenthetical };
}

function childParenthetical(child: ChildSessionState): string | undefined {
  if (child.agentName?.trim()) return `(${child.agentName.trim()})`;

  const primary = splitParentheticalTitle(childPrimaryText(child));
  if (primary.parenthetical) return primary.parenthetical;

  return splitParentheticalTitle(child.title).parenthetical;
}

function formatSecondaryLine(
  continuation: string | undefined,
  parenthetical: string | undefined,
  width: number,
): string | undefined {
  if (!continuation) return parenthetical;
  if (!parenthetical) return continuation;

  const parentheticalWidth = Math.min(textColumns(parenthetical), width);
  const continuationWidth = width - parentheticalWidth - 1;
  if (continuationWidth >= MIN_LABEL_WIDTH) {
    return `${ellipsize(continuation, continuationWidth)} ${ellipsize(parenthetical, parentheticalWidth)}`;
  }

  return ellipsize(parenthetical, width);
}

function childPrimaryText(child: ChildSessionState): string {
  return child.summary?.trim() || child.title;
}

function resolveTokenTotal(child: ChildSessionState): number | undefined {
  const total = child.tokens?.total;
  if (typeof total === "number" && Number.isFinite(total)) {
    return total;
  }
  const input = child.tokens?.input;
  const output = child.tokens?.output;
  if (typeof input === "number" || typeof output === "number") {
    return Math.max(0, (input ?? 0) + (output ?? 0));
  }
  return undefined;
}

function formatCompactTokenCount(total: number): string {
  const value = Math.max(0, total);
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M ctx`;
  if (value >= 1_000) return `${(value / 1_000).toFixed(1)}k ctx`;
  return `${Math.round(value)} ctx`;
}

function formatCompactPercent(percent: number): string {
  return `${Math.max(0, Math.round(percent))}%`;
}

function contextVariants(child: ChildSessionState): string[] {
  const total = resolveTokenTotal(child);
  const percent = child.tokens?.contextPercent;
  const hasTotal = typeof total === "number" && Number.isFinite(total);
  const hasPercent = typeof percent === "number" && Number.isFinite(percent);

  if (!hasTotal && !hasPercent) return [""];

  const tokenPart = hasTotal ? formatCompactTokenCount(total) : "";
  const percentPart = hasPercent ? formatCompactPercent(percent) : "";

  if (tokenPart && percentPart) {
    return [`${tokenPart} ${percentPart}`, percentPart, tokenPart, ""];
  }

  return [tokenPart || percentPart, ""];
}

function rowWidthBudget(sidebarWidth: number | undefined): number {
  const width = sidebarWidth ?? FALLBACK_SIDEBAR_WIDTH;
  const innerWidth = width - 4;
  return Math.max(MIN_ROW_WIDTH, Math.min(innerWidth, 52));
}

export function wrapCompactText(
  value: string,
  width: number,
  maxLines: number,
): string[] {
  const normalized = value.replace(/\s+/g, " ").trim();
  if (!normalized) return [""];

  const lines: string[] = [];
  let remaining = normalized;

  while (textColumns(remaining) > width && lines.length < maxLines - 1) {
    const probe = takeColumns(remaining, width + 1);
    const breakAt = probe.lastIndexOf(" ");
    const breakPrefix = breakAt >= 0 ? probe.slice(0, breakAt) : "";
    const fit = takeColumns(remaining, width);
    const take =
      breakAt >= 0 &&
      textColumns(breakPrefix) >= MIN_LABEL_WIDTH &&
      textColumns(breakPrefix) <= width
        ? breakAt
        : fit.length;
    if (take <= 0) break;

    lines.push(remaining.slice(0, take).trimEnd());
    remaining = remaining.slice(take).trimStart();
  }

  lines.push(
    lines.length === maxLines - 1
      ? ellipsize(remaining, Math.max(1, width))
      : remaining,
  );
  return lines;
}

function formatChildRowLine(input: {
  child: ChildSessionState;
  nowMs: number;
  sidebarWidth?: number;
  reservedWidth?: number;
}): {
  labelLines: string[];
  secondaryLine?: string;
  elapsed: string;
  meta: string;
} {
  const elapsed = formatDuration(elapsedMs(input.child, input.nowMs));
  const width = Math.max(
    MIN_ROW_WIDTH,
    rowWidthBudget(input.sidebarWidth) - (input.reservedWidth ?? 0),
  );
  const title = splitParentheticalTitle(childPrimaryText(input.child));
  const parenthetical = childParenthetical(input.child);

  for (const meta of contextVariants(input.child)) {
    const detailChars =
      2 + textColumns(elapsed) + (meta ? 3 + textColumns(meta) : 0);
    const labelBudget = Math.min(
      width - 2,
      width - Math.max(0, detailChars - width),
    );
    if (labelBudget >= MIN_LABEL_WIDTH || textColumns(meta) === 0) {
      const labelLines = wrapCompactText(
        title.label,
        Math.max(1, labelBudget),
        2,
      );
      return {
        labelLines,
        secondaryLine: formatSecondaryLine(
          labelLines[1],
          parenthetical,
          Math.max(1, labelBudget),
        ),
        elapsed,
        meta,
      };
    }
  }

  const labelLines = wrapCompactText(title.label, MIN_LABEL_WIDTH, 2);
  return {
    labelLines,
    secondaryLine: formatSecondaryLine(
      labelLines[1],
      parenthetical,
      MIN_LABEL_WIDTH,
    ),
    elapsed,
    meta: "",
  };
}

function formatTerminalChildRowLine(input: {
  child: ChildSessionState;
  nowMs: number;
  sidebarWidth?: number;
  reservedWidth?: number;
}): {
  label: string;
  meta: string;
} {
  const elapsed = formatDuration(elapsedMs(input.child, input.nowMs));
  const width = Math.max(MIN_ROW_WIDTH, rowWidthBudget(input.sidebarWidth));
  const title = splitParentheticalTitle(childPrimaryText(input.child));
  const parenthetical = childParenthetical(input.child);
  const labelSource = parenthetical
    ? `${title.label} ${parenthetical}`
    : title.label;
  const context = contextVariants(input.child).find(
    (variant) => variant.length > 0,
  );

  return {
    label: ellipsize(
      labelSource,
      Math.max(1, width - (input.reservedWidth ?? 0)),
    ),
    meta: context ? `${elapsed} ${context}` : elapsed,
  };
}

export function subagentRowHeight(input: {
  child: ChildSessionState;
  nowMs: number;
  sidebarWidth?: number;
  reservedWidth?: number;
}): number {
  const modelHeight = input.child.model?.variant
    ? SUBAGENTS_MODEL_ROW_HEIGHT
    : 0;
  if (input.child.status !== "running") {
    return SUBAGENTS_TERMINAL_ROW_HEIGHT + modelHeight;
  }

  const line = formatChildRowLine(input);
  return (
    (line.secondaryLine
      ? SUBAGENTS_RUNNING_ROW_HEIGHT
      : SUBAGENTS_RUNNING_ROW_HEIGHT - 1) + modelHeight
  );
}

/**
 * Model display line. v2 keeps providers and models in separate collections,
 * so the provider name is looked up through the model collection by
 * `providerID` + `modelID` (v1 nested models under the provider).
 */
export function formatChildModelLine(
  child: ChildSessionState,
  models: unknown[],
  width: number,
): string | undefined {
  if (!child.model?.variant) return undefined;
  const match = models.find((candidate) => {
    const record = asRecord(candidate);
    return (
      record?.providerID === child.model?.providerID &&
      record?.modelID === child.model?.modelID
    );
  });
  const name = asString(asRecord(match)?.name) || child.model.modelID;
  return ellipsize(`${name} · ${child.model.variant}`, Math.max(1, width));
}

export interface TuiSubagentSnapshot {
  visibleChildren: ChildSessionState[];
  visibleCounts: StatusCounts;
  totalExecuted: number;
  showingOtherSessions: boolean;
}

export function resolveTuiSubagentSnapshot(input: {
  state: StatuslineState;
  sessionID?: string;
  nowMs?: number;
  showCompletedHistory?: boolean;
}): TuiSubagentSnapshot {
  const allChildren = Object.values(input.state.children);
  const options = { showCompletedHistory: input.showCompletedHistory };
  const nowMs = input.nowMs ?? Date.now();
  const ownChildren = input.sessionID
    ? allChildren.filter((child) => child.parentID === input.sessionID)
    : allChildren;
  const ownVisibleChildren = visibleSubagentWorkItems(
    ownChildren,
    nowMs,
    options,
  ).sort(byPriority);
  const totalExecuted = input.sessionID
    ? countCountedSubagentExecutions({
        children: allChildren,
        countedChildIDs: input.state.countedChildIDs,
        parentSessionID: input.sessionID,
      })
    : countHistoricalSubagentExecutions({ children: allChildren });

  return {
    visibleChildren: ownVisibleChildren,
    visibleCounts: countRetainedSubagentStatuses({
      children: allChildren,
      parentSessionID: input.sessionID,
    }),
    totalExecuted,
    showingOtherSessions: false,
  };
}

export function resolveSidebarSubagentSnapshot(input: {
  state: StatuslineState;
  sessionID: string;
  nowMs?: number;
  showCompletedHistory?: boolean;
}): TuiSubagentSnapshot {
  return resolveTuiSubagentSnapshot(input);
}

function SidebarSubagents(props: {
  sessionID: string;
  state: () => StatuslineState;
  nowMs: () => number;
  models: () => unknown[];
  expanded: () => boolean;
  onToggleExpanded: () => void;
  onSetExpanded: (expanded: boolean) => void;
  onReturnFocus: () => void;
  onToggleListFocus: () => void;
  onOpenSession: (sessionID: string) => void;
  onNavigateToChild: (input: {
    parentSessionID: string;
    childSessionID: string;
    childRowID: string;
    showCompletedHistory: boolean;
  }) => void;
  sidebarWidth?: () => number | undefined;
  theme: TuiTheme;
  keymap: V2KeymapLayerApi | undefined;
  restoreFromChild?: {
    childRowID: string;
    showCompletedHistory: boolean;
  };
}) {
  const [showCompletedHistory, setShowCompletedHistory] = createSignal(
    props.restoreFromChild?.showCompletedHistory ?? false,
  );
  const completedHistoryOptions = () => ({
    showCompletedHistory: showCompletedHistory(),
  });
  const snapshot = createMemo(() =>
    resolveSidebarSubagentSnapshot({
      state: props.state(),
      sessionID: props.sessionID,
      nowMs: props.nowMs(),
      ...completedHistoryOptions(),
    }),
  );
  const visibleChildren = createMemo(() => snapshot().visibleChildren);
  const counts = createMemo(() => snapshot().visibleCounts);
  const totalExecuted = createMemo(() => snapshot().totalExecuted);

  const visibleChildIDs = createMemo(() =>
    visibleChildren().map((child) => child.id),
  );
  const [selectedChildID, setSelectedChildID] = createSignal<
    string | undefined
  >(props.restoreFromChild?.childRowID);
  let restoreChildRowID = props.restoreFromChild?.childRowID;
  const [mouseDownChildID, setMouseDownChildID] = createSignal<
    string | undefined
  >();
  const [listFocused, setListFocused] = createSignal(false);
  const [listFocusModeActive, setListFocusModeActive] = createSignal(false);

  const visibleChildLayoutSignature = createMemo(() =>
    visibleChildren()
      .map((child) =>
        JSON.stringify([
          child.id,
          child.status,
          child.title,
          child.summary ?? "",
          child.agentName ?? "",
          child.tokens?.input ?? "",
          child.tokens?.output ?? "",
          child.tokens?.total ?? "",
          child.tokens?.contextPercent ?? "",
          child.model?.providerID ?? "",
          child.model?.modelID ?? "",
          child.model?.variant ?? "",
        ]),
      )
      .join("|"),
  );

  const listHeight = createMemo(() => {
    const nowMs = props.nowMs();
    const sidebarWidth = props.sidebarWidth?.();
    const contentHeight =
      visibleChildren().reduce(
        (height, child) =>
          height +
          subagentRowHeight({
            child,
            nowMs,
            sidebarWidth,
            reservedWidth: SUBAGENTS_ROW_MARKER_WIDTH,
          }),
        0,
      ) +
      Math.max(0, visibleChildren().length - 1) * SUBAGENTS_ROW_GAP;

    return Math.max(1, Math.min(SUBAGENTS_MAX_LIST_HEIGHT, contentHeight));
  });

  let listContainer: BoxRenderable | undefined;
  let scrollbox: ScrollBoxRenderable | undefined;
  const scrollRegistration: SidebarScrollRegistration = {
    getScrollbox: () => scrollbox,
    getAnchor: () => currentSidebarScrollAnchor(),
    getRows: () => rowLayouts(),
    getLeadingHeight: () => 0,
    offsetTop: 0,
    restoreFramesRemaining: 0,
  };
  sidebarScrollRegistrations.add(scrollRegistration);
  const focusRegistration: SidebarListFocusRegistration = {
    focusList: (preferredChildID?: string) => {
      if (!listContainer) return false;
      const ids = visibleChildIDs();
      if (preferredChildID && ids.includes(preferredChildID)) {
        setSelectedChildID(preferredChildID);
      } else if (!selectedChildID() && ids[0]) {
        setSelectedChildID(ids[0]);
      }
      listContainer.focus();
      setListFocused(true);
      setListFocusModeActive(true);
      return true;
    },
    blurList: () => {
      if (!listFocused() && !listFocusModeActive()) return false;
      listContainer?.blur();
      setListFocused(false);
      setListFocusModeActive(false);
      return true;
    },
    isListFocusModeActive: () => listFocusModeActive(),
  };
  sidebarListFocusRegistrations.add(focusRegistration);
  let previousRunningCount: number | undefined;
  createEffect(() => {
    const runningCount = counts().running;
    const shouldReleaseFocus = shouldReleaseSidebarListFocus({
      previousRunningCount,
      runningCount,
      listFocusModeActive: listFocusModeActive(),
    });
    previousRunningCount = runningCount;
    if (!shouldReleaseFocus) return;

    focusRegistration.blurList();
    props.onReturnFocus();
  });
  const completedHistoryRegistration: SidebarCompletedHistoryRegistration = {
    toggleCompletedHistory: () => {
      setShowCompletedHistory((current) => !current);
      return true;
    },
  };
  sidebarCompletedHistoryRegistrations.add(completedHistoryRegistration);
  onCleanup(() => {
    sidebarScrollRegistrations.delete(scrollRegistration);
    sidebarListFocusRegistrations.delete(focusRegistration);
    sidebarCompletedHistoryRegistrations.delete(completedHistoryRegistration);
  });

  createEffect(() => {
    const ids = visibleChildIDs();
    const current = selectedChildID();
    if (ids.length === 0) {
      if (current) setSelectedChildID(undefined);
      return;
    }
    if (!current || !ids.includes(current)) setSelectedChildID(ids[0]);
  });

  const refreshListFocused = (): void => {
    if (listFocused() && !listContainer) {
      setListFocused(false);
      return;
    }
    const focused = Boolean(
      listContainer?.focused || listContainer?.hasFocusedDescendant,
    );
    if (!focused && listFocused()) setListFocused(false);
  };

  const rowTopForIndex = (index: number): number => {
    let top = 0;
    const nowMs = props.nowMs();
    const sidebarWidth = props.sidebarWidth?.();
    for (let i = 0; i < index; i += 1) {
      const child = visibleChildren()[i];
      if (child) {
        top +=
          subagentRowHeight({
            child,
            nowMs,
            sidebarWidth,
            reservedWidth: SUBAGENTS_ROW_MARKER_WIDTH,
          }) + SUBAGENTS_ROW_GAP;
      }
    }
    return top;
  };

  const rowLayouts = (): SidebarScrollRowLayout[] => {
    const nowMs = props.nowMs();
    const sidebarWidth = props.sidebarWidth?.();
    return visibleChildren().map((child) => ({
      id: child.id,
      height: subagentRowHeight({
        child,
        nowMs,
        sidebarWidth,
        reservedWidth: SUBAGENTS_ROW_MARKER_WIDTH,
      }),
    }));
  };

  const currentSidebarScrollAnchor = (): SidebarScrollAnchor | undefined => {
    if (!scrollbox) return undefined;
    const rows = rowLayouts();
    if (rows.length === 0) return undefined;

    const viewportTop = clampedScrollTop(scrollbox, scrollbox.scrollTop);
    let top = 0;
    for (let index = 0; index < rows.length; index += 1) {
      const row = rows[index];
      if (!row) continue;
      const rowBottom = top + row.height;
      if (rowBottom > viewportTop) {
        return {
          childIDs: rows.slice(index).map((candidate) => candidate.id),
          intraRowOffset: Math.max(0, viewportTop - top),
        };
      }
      top = rowBottom + SUBAGENTS_ROW_GAP;
    }

    const lastRow = rows[rows.length - 1];
    return lastRow ? { childIDs: [lastRow.id], intraRowOffset: 0 } : undefined;
  };

  const scrollChildIntoView = (childID: string | undefined): void => {
    if (!scrollbox) return;
    const selectedIndex = visibleChildIDs().findIndex((id) => id === childID);
    if (selectedIndex < 0) return;
    const selectedChild = visibleChildren()[selectedIndex];
    if (!selectedChild) return;

    const rowTop = rowTopForIndex(selectedIndex);
    const rowBottom =
      rowTop +
      subagentRowHeight({
        child: selectedChild,
        nowMs: props.nowMs(),
        sidebarWidth: props.sidebarWidth?.(),
        reservedWidth: SUBAGENTS_ROW_MARKER_WIDTH,
      });
    const viewportTop = scrollbox.scrollTop;
    const viewportBottom = viewportTop + listHeight();

    if (rowTop < viewportTop) {
      const nextTop = clampedScrollTop(scrollbox, rowTop);
      scrollRegistration.offsetTop = nextTop;
      scrollbox.scrollTop = nextTop;
    } else if (rowBottom > viewportBottom) {
      const nextTop = clampedScrollTop(scrollbox, rowBottom - listHeight());
      scrollRegistration.offsetTop = nextTop;
      scrollbox.scrollTop = nextTop;
    }
  };

  const scrollSelectedChildIntoView = (): void => {
    if (!listFocusModeActive()) return;
    scrollChildIntoView(selectedChildID());
  };

  const moveSelection = (delta: number): void => {
    const ids = visibleChildIDs();
    if (ids.length === 0) return;
    const currentIndex = ids.findIndex((id) => id === selectedChildID());
    const fallbackIndex = delta > 0 ? 0 : ids.length - 1;
    const nextIndex = Math.max(
      0,
      Math.min(
        ids.length - 1,
        currentIndex < 0 ? fallbackIndex : currentIndex + delta,
      ),
    );
    setSelectedChildID(ids[nextIndex]);
    scrollChildIntoView(ids[nextIndex]);
  };

  const rowActivations = new Map<string, () => void>();

  const resolveNavigableChildTargetSessionID = (
    child: ChildSessionState,
  ): string | undefined =>
    resolveChildTargetSessionID(child) ??
    resolveSyntheticTargetFromHydratedState(props.state(), child);

  const selectedTargetSessionID = (): string | undefined => {
    const selected = visibleChildren().find(
      (child) => child.id === selectedChildID(),
    );
    return selected
      ? resolveNavigableChildTargetSessionID(selected)
      : undefined;
  };

  const activateSelectedChild = (): void => {
    const selectedID = selectedChildID();
    const activateRow = selectedID ? rowActivations.get(selectedID) : undefined;
    if (activateRow) {
      activateRow();
      return;
    }
    navigateToSessionTargetByID(selectedTargetSessionID());
  };

  const navigateToSessionTargetByID = (target: string | undefined): void => {
    if (!isSessionTarget(target)) return;
    props.onOpenSession(target);
  };

  const toggleCompletedHistory = (): void => {
    completedHistoryRegistration.toggleCompletedHistory();
  };

  createEffect(() => {
    selectedChildID();
    listHeight();
    if (!listFocused()) return;
    scrollSelectedChildIntoView();
  });

  /**
   * The `useKeyboard` handler stays the authority for the list keys: it
   * needs `listFocused()` and `event.preventDefault()/stopPropagation()`, which
   * a keymap command cannot express. A keymap layer is still registered with
   * `target` bound to the list container so OpenCode's command discovery knows
   * the actions; the commands are marked `bind: false` because binding
   * j/k/arrows/Enter/c/h/l/Escape here would double-dispatch alongside the
   * retained handler.
   */
  props.keymap?.layer?.(() => ({
    target: () => listContainer ?? null,
    enabled: () => listFocusModeActive(),
    commands: [
      {
        id: "subagent-statusline.list.next",
        title: "Subagents: Select next row",
        group: COMMAND_GROUP,
        bind: false,
        run: () => moveSelection(1),
      },
      {
        id: "subagent-statusline.list.previous",
        title: "Subagents: Select previous row",
        group: COMMAND_GROUP,
        bind: false,
        run: () => moveSelection(-1),
      },
      {
        id: "subagent-statusline.list.open",
        title: "Subagents: Open selected session",
        group: COMMAND_GROUP,
        bind: false,
        run: () => activateSelectedChild(),
      },
      {
        id: "subagent-statusline.list.collapse",
        title: "Subagents: Collapse list",
        group: COMMAND_GROUP,
        bind: false,
        run: () => props.onSetExpanded(false),
      },
      {
        id: "subagent-statusline.list.expand",
        title: "Subagents: Expand list",
        group: COMMAND_GROUP,
        bind: false,
        run: () => props.onSetExpanded(true),
      },
      {
        id: "subagent-statusline.list.toggle-completed-history",
        title: "Subagents: Toggle completed history",
        group: COMMAND_GROUP,
        bind: false,
        run: () => toggleCompletedHistory(),
      },
      {
        id: "subagent-statusline.list.blur",
        title: "Subagents: Leave list",
        group: COMMAND_GROUP,
        bind: false,
        run: () => {
          focusRegistration.blurList();
          props.onReturnFocus();
        },
      },
    ],
  }));

  const handleListKeyDown = (event: KeyEvent): void => {
    if (!listFocused()) return;
    const name = event.name.toLowerCase();
    if ((event.meta || event.option) && name === "b") {
      props.onToggleListFocus();
    } else if (name === "j" || name === "down" || name === "arrowdown") {
      moveSelection(1);
    } else if (name === "k" || name === "up" || name === "arrowup") {
      moveSelection(-1);
    } else if (name === "return" || name === "enter") {
      activateSelectedChild();
    } else if (name === "h" || name === "left" || name === "arrowleft") {
      if (props.expanded()) props.onSetExpanded(false);
    } else if (name === "l" || name === "right" || name === "arrowright") {
      if (!props.expanded()) props.onSetExpanded(true);
    } else if (name === "c") {
      toggleCompletedHistory();
    } else if (name === "escape" || name === "esc") {
      focusRegistration.blurList();
      props.onReturnFocus();
    } else {
      return;
    }

    event.preventDefault();
    event.stopPropagation();
  };

  useKeyboard(handleListKeyDown);

  const restorePreservedScroll = (): void => {
    if (!scrollbox) return;
    if (scrollRegistration.restoreFramesRemaining <= 0) return;
    scrollRegistration.restoreFramesRemaining -= 1;

    if (restoreChildRowID) {
      const childRowID = restoreChildRowID;
      restoreChildRowID = undefined;
      scrollRegistration.restoreFramesRemaining = 0;
      if (visibleChildIDs().includes(childRowID)) {
        scrollChildIntoView(childRowID);
      } else {
        scrollbox.scrollTop = 0;
      }
      return;
    }

    const top = preservedSidebarScrollTop({
      expanded: props.expanded(),
      offsetTop: scrollRegistration.offsetTop,
      anchor: scrollRegistration.anchor,
      rows: scrollRegistration.getRows(),
      leadingHeight: scrollRegistration.getLeadingHeight(),
      scrollTop: scrollbox.scrollTop,
      scrollHeight: scrollbox.scrollHeight,
      viewportHeight: scrollbox.viewport.height,
    });
    if (top === undefined) return;
    scrollRegistration.offsetTop = top;
    scrollbox.scrollTop = top;
  };

  createEffect(() => {
    props.expanded();
    visibleChildIDs().join("|");
    visibleChildLayoutSignature();
    props.sidebarWidth?.();

    restorePreservedScroll();
  });

  const ChildRow = (rowProps: { childID: string }) => {
    const child = createMemo(() =>
      visibleChildren().find((candidate) => candidate.id === rowProps.childID),
    );
    const [hovered, setHovered] = createSignal(false);
    const [focused, setFocused] = createSignal(false);
    const targetSessionID = createMemo(() => {
      const currentChild = child();
      return currentChild
        ? resolveNavigableChildTargetSessionID(currentChild)
        : undefined;
    });
    const clickable = createMemo(() => isSessionTarget(targetSessionID()));
    const selected = createMemo(
      () => listFocused() && selectedChildID() === rowProps.childID,
    );
    const emphasized = createMemo(
      () => clickable() && (hovered() || focused() || selected()),
    );
    const status = createMemo<ChildSessionState["status"]>(
      () => child()?.status ?? "running",
    );
    const muted = createMemo(
      () => status() !== "running" && clickable() && !emphasized(),
    );
    const rowOpacity = createMemo(() =>
      status() === "running" ? 1 : INACTIVE_SUBAGENT_OPACITY,
    );
    const line = createMemo(() => {
      const currentChild = child();
      if (!currentChild) {
        return { labelLines: [""], elapsed: "00:00", meta: "" };
      }
      return formatChildRowLine({
        child: currentChild,
        nowMs: props.nowMs(),
        sidebarWidth: props.sidebarWidth?.(),
        reservedWidth: SUBAGENTS_ROW_MARKER_WIDTH,
      });
    });
    const terminalLine = createMemo(() => {
      const currentChild = child();
      if (!currentChild) return { label: "", meta: "00:00" };
      return formatTerminalChildRowLine({
        child: currentChild,
        nowMs: props.nowMs(),
        sidebarWidth: props.sidebarWidth?.(),
        reservedWidth: SUBAGENTS_ROW_MARKER_WIDTH,
      });
    });
    const rowHeight = createMemo(() => {
      const currentChild = child();
      if (!currentChild) return SUBAGENTS_TERMINAL_ROW_HEIGHT;
      return subagentRowHeight({
        child: currentChild,
        nowMs: props.nowMs(),
        sidebarWidth: props.sidebarWidth?.(),
        reservedWidth: SUBAGENTS_ROW_MARKER_WIDTH,
      });
    });
    const modelLine = createMemo(() => {
      const currentChild = child();
      if (!currentChild) return undefined;
      return formatChildModelLine(
        currentChild,
        props.models(),
        rowWidthBudget(props.sidebarWidth?.()) - SUBAGENTS_ROW_MARKER_WIDTH,
      );
    });
    const activate = () => {
      const target = targetSessionID();
      if (target) {
        props.onNavigateToChild({
          parentSessionID: props.sessionID,
          childSessionID: target,
          childRowID: rowProps.childID,
          showCompletedHistory: showCompletedHistory(),
        });
        props.onOpenSession(target);
      }
      snapshotSidebarScrollOffsets();
    };
    rowActivations.set(rowProps.childID, activate);
    onCleanup(() => {
      rowActivations.delete(rowProps.childID);
    });
    const handleKeyDown = (event: KeyEvent): void => {
      if (!clickable()) return;
      setFocused(true);
      if (event.name === "return" || event.name === "space") {
        activate();
        event.preventDefault();
        event.stopPropagation();
      }
    };

    return (
      <box
        flexDirection="column"
        height={rowHeight()}
        opacity={rowOpacity()}
        backgroundColor={selected() ? props.theme.backgroundElement : undefined}
        onMouseOver={clickable() ? () => setHovered(true) : undefined}
        onMouseOut={
          clickable()
            ? () => {
                setHovered(false);
                setFocused(false);
                setMouseDownChildID(undefined);
              }
            : undefined
        }
        onMouseDown={
          clickable()
            ? (event: MouseEvent) => {
                event.stopPropagation();
                setSelectedChildID(rowProps.childID);
                setMouseDownChildID(rowProps.childID);
              }
            : undefined
        }
        onMouseUp={
          clickable()
            ? (event: MouseEvent) => {
                if (mouseDownChildID() === rowProps.childID) {
                  event.stopPropagation();
                  activate();
                }
                setMouseDownChildID(undefined);
              }
            : undefined
        }
        onKeyDown={clickable() ? handleKeyDown : undefined}
        focusable={clickable()}
        focused={clickable() && focused()}
      >
        <Show
          when={status() === "running"}
          fallback={
            <box flexDirection="column">
              <box flexDirection="row">
                <text
                  fg={selected() ? props.theme.accent : props.theme.textMuted}
                >
                  {selected() ? "›" : " "}
                </text>
                <text fg={statusColor(status(), props.theme)}>
                  {taskStatusMarker(status())}
                </text>
                <text
                  fg={
                    selected()
                      ? props.theme.text
                      : muted()
                        ? props.theme.textMuted
                        : props.theme.text
                  }
                >{` ${terminalLine().label}`}</text>
              </box>
              <text
                fg={emphasized() ? props.theme.text : props.theme.textMuted}
              >{`    ↳ ${CLOCK_ICON} ${terminalLine().meta}`}</text>
              <Show when={modelLine()}>
                {(metadata: Accessor<string>) => (
                  <text fg={props.theme.textMuted}>{`    ${metadata()}`}</text>
                )}
              </Show>
            </box>
          }
        >
          <box flexDirection="column">
            <box flexDirection="row">
              <text
                fg={selected() ? props.theme.accent : props.theme.textMuted}
              >
                {selected() ? "›" : " "}
              </text>
              <text fg={statusColor(status(), props.theme)}>
                {taskStatusMarker(status())}
              </text>
              <text
                fg={
                  selected()
                    ? props.theme.text
                    : muted()
                      ? props.theme.textMuted
                      : props.theme.text
                }
              >{` ${line().labelLines[0] ?? ""}`}</text>
            </box>
            <Show when={line().secondaryLine}>
              {(secondaryLine: Accessor<string>) => (
                <text
                  fg={muted() ? props.theme.textMuted : props.theme.text}
                >{`    ${secondaryLine()}`}</text>
              )}
            </Show>
            <box flexDirection="row" paddingLeft={4}>
              <text
                fg={emphasized() ? props.theme.text : props.theme.textMuted}
              >{`↳ ${CLOCK_ICON} ${line().elapsed}`}</text>
              <Show when={line().meta.length > 0}>
                <text
                  fg={emphasized() ? props.theme.text : props.theme.textMuted}
                >{` ${TOKEN_ICON} ${line().meta}`}</text>
              </Show>
            </box>
            <Show when={modelLine()}>
              {(metadata: Accessor<string>) => (
                <text fg={props.theme.textMuted}>{`    ${metadata()}`}</text>
              )}
            </Show>
          </box>
        </Show>
      </box>
    );
  };

  const AggregateBar = () => (
    <box flexDirection="row" paddingRight={1}>
      <text fg={props.theme.warning}>{`● ${counts().running} run`}</text>
      <text fg={props.theme.textMuted}> · </text>
      <text fg={props.theme.success}>{`✓ ${counts().done} done`}</text>
      <text fg={props.theme.textMuted}> · </text>
      <text fg={props.theme.error}>{`✕ ${counts().error} err`}</text>
      <text fg={props.theme.textMuted}> · </text>
      <text
        fg={showCompletedHistory() ? props.theme.accent : props.theme.text}
        selectable={false}
        onMouseDown={toggleCompletedHistory}
      >{`Σ ${totalExecuted()}`}</text>
    </box>
  );

  return (
    <box
      ref={(element) => {
        listContainer = element;
        if (!element) setListFocused(false);
      }}
      flexDirection="column"
      backgroundColor={listFocused() ? props.theme.backgroundPanel : undefined}
      focusable
      focused={listFocused()}
      renderBefore={() => {
        refreshListFocused();
        restorePreservedScroll();
      }}
    >
      <box flexDirection="row">
        <text
          fg={props.theme.text}
          selectable={false}
          onMouseDown={props.onToggleExpanded}
        >{`${props.expanded() ? SIDEBAR_ARROW_EXPANDED : SIDEBAR_ARROW_COLLAPSED} ${t("subagents")}`}</text>
        <Show when={PLUGIN_VERSION}>
          {(version: Accessor<string>) => (
            <box flexDirection="row">
              <text
                fg={props.theme.textMuted}
                opacity={SIDEBAR_VERSION_OPACITY}
                selectable={false}
                onMouseDown={props.onToggleExpanded}
              >{` ${version()}`}</text>
              <Show when={listFocused()}>
                <text
                  fg={props.theme.accent}
                  selectable={false}
                  onMouseDown={props.onToggleExpanded}
                >{` ${SIDEBAR_FOCUS_INDICATOR}`}</text>
              </Show>
            </box>
          )}
        </Show>
      </box>
      <AggregateBar />

      <Show when={props.expanded()}>
        <scrollbox
          ref={(element) => {
            scrollbox = element;
            restorePreservedScroll();
          }}
          height={listHeight()}
          scrollY
          viewportCulling={false}
        >
          <box flexDirection="column" rowGap={SUBAGENTS_ROW_GAP}>
            <For each={visibleChildIDs()}>
              {(childID: string) => <ChildRow childID={childID} />}
            </For>
          </box>
        </scrollbox>
      </Show>
    </box>
  );
}

function HomeBottomStatus(props: {
  state: () => StatuslineState;
  theme: TuiTheme;
}) {
  const snapshot = createMemo(() =>
    resolveTuiSubagentSnapshot({ state: props.state() }),
  );
  const counts = createMemo(() => snapshot().visibleCounts);
  const totalExecuted = createMemo(() => snapshot().totalExecuted);
  const visible = createMemo(
    () => counts().running > 0 || counts().error > 0 || totalExecuted() > 0,
  );

  return (
    <Show when={visible()}>
      <box paddingLeft={1} paddingRight={1}>
        <box flexDirection="row">
          <text fg={props.theme.warning}>{`● ${counts().running}`}</text>
          <text fg={props.theme.textMuted}> · </text>
          <text fg={props.theme.success}>{`✓ ${counts().done}`}</text>
          <text fg={props.theme.textMuted}> · </text>
          <text fg={props.theme.error}>{`✕ ${counts().error}`}</text>
          <text fg={props.theme.textMuted}> · </text>
          <text fg={props.theme.text}>{`Σ ${totalExecuted()}`}</text>
        </box>
      </box>
    </Show>
  );
}

/** Strip ANSI color codes: a TUI text node renders them literally. */
function stripAnsi(value: string): string {
  return value.replace(/\u001B\[[0-9;]*m/g, "");
}

/**
 * Compact status summary for `prompt.footer.status`, claimed `after` so the
 * host's own status content is never suppressed. Reuses `renderStatusLine`.
 */
function CompactSummary(props: {
  state: () => StatuslineState;
  theme: TuiTheme;
}) {
  const text = createMemo(() => stripAnsi(renderStatusLine(props.state())));
  return (
    <Show when={text().length > 0}>
      <text fg={props.theme.textMuted}>{text()}</text>
    </Show>
  );
}

function deriveSessionChildStatus(
  status: unknown,
): ChildSessionState["status"] | undefined {
  return deriveOpenCodeSessionStatus(status);
}

function sessionTimestamp(
  session: Record<string, unknown>,
  key: string,
): string | undefined {
  const time = asRecord(session.time);
  return timestampFromUnknown(time?.[key]);
}

function timestampFromUnknown(value: unknown): string | undefined {
  const millis = timestampMillisFromUnknown(value);
  return millis === undefined ? undefined : new Date(millis).toISOString();
}

function timestampMillisFromUnknown(value: unknown): number | undefined {
  if (typeof value === "string") {
    const parsed = Date.parse(value);
    return Number.isNaN(parsed) ? undefined : parsed;
  }
  if (typeof value === "number" && Number.isFinite(value) && value > 0) {
    const millis = value < 10_000_000_000 ? value * 1000 : value;
    const parsed = new Date(millis);
    return Number.isNaN(parsed.getTime()) ? undefined : millis;
  }
  return undefined;
}

/** v2 current session, read reactively from the router. */
function resolveRouteSessionID(ctx: V2TuiContext): string | undefined {
  const route = safeRead(() => ctx.ui.router.current());
  return route?.type === "session" && typeof route.sessionID === "string"
    ? route.sessionID
    : undefined;
}

function shouldHydrateSessionChild(input: {
  childID: string;
  sessionStatus?: ChildSessionState["status"];
  childSummary?: SessionMessageSummary;
  parentTaskEvidenceByChildID: ReadonlyMap<string, ParentTaskEvidence>;
}): boolean {
  if (input.sessionStatus) return true;
  if (input.parentTaskEvidenceByChildID.has(input.childID)) return true;

  const summary = input.childSummary;
  if (!summary || summary.fetchFailed) return false;

  return (
    summary.hasError === true ||
    typeof summary.completedAt === "string" ||
    typeof summary.evidenceAt === "string" ||
    typeof summary.latestAssistantActivityAt === "string" ||
    typeof summary.latestMessageActivityAt === "string"
  );
}

type ParentTaskEvidence = {
  status: ChildSessionState["status"];
  endedAt?: string;
};

function collectParentTaskEvidenceByChildSessionID(
  messages: unknown[],
  parentSessionID: string,
): Map<string, ParentTaskEvidence> {
  const evidenceByID = new Map<string, ParentTaskEvidence>();
  for (const rawMessage of messages) {
    const message = asRecord(rawMessage);
    const info = asRecord(message?.info);
    const parts = Array.isArray(message?.parts) ? message.parts : [];
    for (const rawPart of parts) {
      const part = asRecord(rawPart);
      if (!part || part.type !== "tool" || part.tool !== "task") continue;
      const state = asRecord(part.state);
      const metadata = asRecord(state?.metadata);
      const childID =
        typeof metadata?.sessionId === "string"
          ? metadata.sessionId
          : undefined;
      if (!childID || childID === parentSessionID) continue;

      const taskEvidence = extractTaskToolEvidence({
        type: "message.part.updated",
        properties: {
          sessionID: parentSessionID,
          info: {
            time: info?.time,
          },
          part: rawPart,
        },
      });
      evidenceByID.set(childID, {
        status: taskEvidence?.status ?? "running",
        endedAt: taskEvidence?.endedAt,
      });
    }
  }
  return evidenceByID;
}

/**
 * v2 hydration. Replaces the v1 client calls (`session.children/messages/status`)
 * with defensive reads over `ctx.data` stores: sync the parent session, then
 * walk `session.family` / `get` for children, `session.status(id)` for status,
 * and `session.message.sync/list` for messages. `ctx.location.directory` may be
 * undefined; every read is best-effort and never throws.
 */
export async function hydratePreviousSubagents(
  ctx: V2TuiContext,
  currentSessionID: string,
  statePath: string,
  textPath: string,
  setState: (fn: (prev: StatuslineState) => StatuslineState) => void,
): Promise<boolean> {
  if (!currentSessionID) return false;

  try {
    let topLevelHydrationFailed = false;
    let statusHydrationFailed = false;
    let parentMessageHydrationFailed = false;

    const synced = await safeReadAsync(async () => {
      await ctx.data.session.sync(currentSessionID);
      return true;
    });
    if (!synced) topLevelHydrationFailed = true;

    const rootID = safeRead(() => ctx.data.session.root(currentSessionID));

    const family =
      safeRead(() => ctx.data.session.family(rootID ?? currentSessionID)) ?? [];
    let childIDs = family.filter((id) => id !== currentSessionID);
    if (childIDs.length === 0) {
      // Fallback for stores that do not track a family: scan the flat list.
      const all = safeRead(() => ctx.data.session.list()) ?? [];
      childIDs = all
        .map((session) => asRecord(session))
        .filter((session) => session?.parentID === currentSessionID)
        .map((session) => asString(session?.id))
        .filter((id): id is string => !!id);
    }

    const allStatuses: Record<string, unknown> = {};
    for (const id of childIDs) {
      const status = safeRead(() => ctx.data.session.status(id));
      if (status === undefined) statusHydrationFailed = true;
      else allStatuses[id] = status;
    }

    const children = childIDs
      .map((id) => safeRead(() => ctx.data.session.get(id)))
      .map(normalizeSessionInfo)
      .filter((session): session is Record<string, unknown> => !!session);

    const parentRaw = await readSessionMessages(ctx, currentSessionID);
    if (parentRaw === undefined) {
      topLevelHydrationFailed = true;
      parentMessageHydrationFailed = true;
    }
    const messages = (parentRaw ?? []).map((message) =>
      normalizeV2Message(message, currentSessionID),
    );
    const parentTaskEvidenceByChildID =
      collectParentTaskEvidenceByChildSessionID(messages, currentSessionID);

    let childHydrationFailed = false;
    const childMessageResults: Array<
      SessionMessageSummary & {
        childID?: string;
        fetchFailed: boolean;
        model?: ReturnType<typeof extractLatestAssistantModel>;
      }
    > = await Promise.all(
      children.map(async (session) => {
        const childID = session.id as string;
        const raw = await readSessionMessages(ctx, childID);
        let fetchFailed = false;
        if (raw === undefined) {
          childHydrationFailed = true;
          fetchFailed = true;
        }
        const childMessages = (raw ?? []).map((message) =>
          normalizeV2Message(message, childID),
        );
        return {
          childID,
          ...summarizeSessionMessages(childMessages),
          model: extractLatestAssistantModel(childMessages),
          fetchFailed,
        };
      }),
    );
    const childMessageSummaryByID = new Map(
      childMessageResults
        .filter((result) => result.childID)
        .map((result) => [result.childID as string, result]),
    );

    snapshotSidebarScrollOffsets();
    setState((current) => {
      const next = cloneState(current);
      let changed = false;

      for (const session of children) {
        const status = allStatuses[session.id as string];
        const sessionStatus = deriveSessionChildStatus(status);
        const childSummary = childMessageSummaryByID.get(session.id as string);
        const hasHydrationEvidence = shouldHydrateSessionChild({
          childID: session.id as string,
          sessionStatus,
          childSummary,
          parentTaskEvidenceByChildID,
        });
        const parentTaskEvidence = parentTaskEvidenceByChildID.get(
          session.id as string,
        );
        const explicitCompletionEvidence =
          !!childSummary &&
          !childSummary.fetchFailed &&
          (typeof childSummary.completedAt === "string" ||
            childSummary.hasError);
        const fallbackEndedAt =
          childSummary?.completedAt ?? childSummary?.evidenceAt;
        const statusEndedAt =
          fallbackEndedAt ??
          sessionTimestamp(session, "completed") ??
          sessionTimestamp(session, "updated");

        if (!hasHydrationEvidence) {
          const existing = next.children[session.id as string];
          if (
            !statusHydrationFailed &&
            !parentMessageHydrationFailed &&
            !!childSummary &&
            !childSummary.fetchFailed &&
            existing?.parentID === currentSessionID &&
            existing.source === "session" &&
            existing.status === "running"
          ) {
            delete next.children[session.id as string];
            changed = true;
          }
          continue;
        }

        const fakeEvent = {
          type: "session.created",
          properties: {
            sessionID: session.id,
            info: session,
          },
        };
        if (applySubagentEvent(next, fakeEvent)) changed = true;
        if (childSummary?.model) {
          changed =
            setChildModel(
              next,
              session.id as string,
              childSummary.model.model,
              childSummary.model.updatedAt,
            ) || changed;
        }

        const resolvedStatus = resolveSessionStatusWithMessageSummary({
          status: sessionStatus ?? parentTaskEvidence?.status,
          summary: childSummary,
        });

        if (
          resolvedStatus.status === "done" ||
          resolvedStatus.status === "error"
        ) {
          if (
            markChildStatus(
              next,
              session.id as string,
              resolvedStatus.status,
              resolvedStatus.endedAt ??
                parentTaskEvidence?.endedAt ??
                statusEndedAt,
            )
          )
            changed = true;
          continue;
        }

        if (
          !sessionStatus &&
          !statusHydrationFailed &&
          explicitCompletionEvidence
        ) {
          const childStatus = childSummary?.hasError ? "error" : "done";
          if (
            markChildStatus(
              next,
              session.id as string,
              childStatus,
              fallbackEndedAt,
            )
          )
            changed = true;
        }
      }

      for (const message of messages) {
        const info = asRecord(message?.info);
        const parts = Array.isArray(message?.parts) ? message.parts : [];
        const parentMessageID = messageIDOf(message);
        const isAssistant = info?.role === "assistant";
        const time = asRecord(info?.time);
        const eventInfo = {
          id: typeof info?.id === "string" ? info.id : undefined,
          role: typeof info?.role === "string" ? info.role : undefined,
          parentID:
            typeof info?.parentID === "string" ? info.parentID : undefined,
          time,
        };
        const completedAt = timestampFromUnknown(time?.completed);
        const isCompleted = typeof completedAt === "string";
        const hasError = !!info?.error;

        for (const rawPart of parts) {
          const part = asRecord(rawPart);
          if (!part) continue;
          const partWithMessageID =
            typeof part.messageID === "string" && part.messageID.length > 0
              ? part
              : parentMessageID
                ? { ...part, messageID: parentMessageID }
                : part;
          if (
            part.type === "subtask" ||
            (part.type === "tool" &&
              (part.tool === "delegate" || part.tool === "task"))
          ) {
            const fakeEvent = {
              type: "message.part.updated",
              properties: {
                sessionID: currentSessionID,
                info: eventInfo,
                part: partWithMessageID,
              },
            };
            if (applySubagentEvent(next, fakeEvent)) changed = true;

            if (part.type === "subtask" && isAssistant && isCompleted) {
              const childID = `subtask:${part.id}`;
              const status = hasError ? "error" : "done";
              if (markChildStatus(next, childID, status, completedAt))
                changed = true;
            }
          }
        }
      }

      if (backfillHydratedTargetSessionIDs(next, currentSessionID)) {
        changed = true;
      }

      const refreshed = refreshLiveState(next);
      if (!changed && !refreshed) return current;
      persistStateSnapshot(statePath, textPath, next);
      return next;
    });
    if (topLevelHydrationFailed || childHydrationFailed) return false;
    return true;
  } catch (err) {
    debugLog({
      kind: "hydration.error",
      sessionID: currentSessionID,
      error: String(err),
    });
    return false;
  }
}

function resolveRunningChildAgeMillis(
  child: ChildSessionState,
  nowMs: number,
): {
  startedMs: number;
  updatedMs: number;
} {
  const startedMs = Date.parse(child.startedAt);
  const updatedMs = Date.parse(child.updatedAt);
  return {
    startedMs: Number.isNaN(startedMs) ? 0 : Math.max(0, nowMs - startedMs),
    updatedMs: Number.isNaN(updatedMs) ? 0 : Math.max(0, nowMs - updatedMs),
  };
}

function resolveReconcileTargetSessionID(
  state: StatuslineState,
  child: ChildSessionState,
): string | undefined {
  return (
    resolveChildTargetSessionID(child) ??
    resolveSyntheticTargetFromHydratedState(state, child)
  );
}

function selectRunningReconcileCandidates(input: {
  state: StatuslineState;
  currentSessionID?: string;
  nowMs: number;
  maxCandidates: number;
}): RunningReconcileCandidate[] {
  const runningChildren = Object.values(input.state.children).filter(
    (child) => child.status === "running",
  );
  if (runningChildren.length === 0) return [];

  const prioritized = visibleSubagentWorkItems(
    runningChildren,
    input.nowMs,
  ).sort(byPriority);
  const prioritizedForSession = prioritized.filter((child) =>
    input.currentSessionID ? child.parentID === input.currentSessionID : true,
  );

  const veryOldIDs = new Set(
    runningChildren
      .filter((child) => {
        const age = resolveRunningChildAgeMillis(child, input.nowMs);
        return (
          age.startedMs >= RUNNING_RECONCILE_OLD_CANDIDATE_AGE_MS ||
          age.updatedMs >= RUNNING_RECONCILE_OLD_CANDIDATE_AGE_MS
        );
      })
      .map((child) => child.id),
  );

  const ordered = [
    ...prioritizedForSession,
    ...runningChildren.filter((child) => veryOldIDs.has(child.id)),
  ];

  const selected: RunningReconcileCandidate[] = [];
  const seen = new Set<string>();
  for (const child of ordered) {
    if (seen.has(child.id)) continue;
    seen.add(child.id);
    const age = resolveRunningChildAgeMillis(child, input.nowMs);
    const targetSessionID = resolveReconcileTargetSessionID(input.state, child);
    const canProbePersistedSubtask =
      child.source === "subtask" &&
      !targetSessionID &&
      typeof child.parentID === "string" &&
      child.parentID.length > 0 &&
      typeof child.messageID === "string" &&
      child.messageID.length > 0 &&
      (age.startedMs >= RUNNING_RECONCILE_OLD_CANDIDATE_AGE_MS ||
        age.updatedMs >= RUNNING_RECONCILE_OLD_CANDIDATE_AGE_MS);
    if (!targetSessionID && !canProbePersistedSubtask) continue;
    selected.push({
      childID: child.id,
      targetSessionID,
      parentID: child.parentID,
      messageID: child.messageID,
      source: child.source,
      title: child.title,
      summary: child.summary,
      agentName: child.agentName,
      startedMs: age.startedMs,
      updatedMs: age.updatedMs,
    });
    if (selected.length >= input.maxCandidates) break;
  }

  return capCandidates(selected, input.maxCandidates);
}

export async function probeRunningEvidence(input: {
  ctx: V2TuiContext;
  targetSessionID: string;
  candidateAgeMs: number;
  nowMs: number;
}): Promise<RunningReconcileEvidence> {
  let probeFailed = false;

  const directStatus = safeRead(() =>
    input.ctx.data.session.status(input.targetSessionID),
  );
  if (directStatus === undefined) probeFailed = true;
  const statusFromState = deriveSessionChildStatus(directStatus);
  if (statusFromState === "error") {
    return { status: statusFromState, endedAt: new Date().toISOString() };
  }
  if (statusFromState === "running") {
    return { status: "running", sawRunningEvidence: true };
  }

  const hasDoneStatus = statusFromState === "done";

  if (
    !hasDoneStatus &&
    input.candidateAgeMs < RUNNING_RECONCILE_MESSAGE_AGE_GATE_MS
  ) {
    return { probeFailed, canApplyStaleFallback: false };
  }

  const raw = await readSessionMessages(input.ctx, input.targetSessionID);
  if (raw === undefined) {
    if (hasDoneStatus) {
      return {
        status: "done",
        endedAt: new Date().toISOString(),
        checkedMessages: false,
        probeFailed: true,
        canApplyStaleFallback: false,
      };
    }
    return {
      checkedMessages: false,
      probeFailed: true,
      canApplyStaleFallback: false,
    };
  }
  const messages = raw.map((message) =>
    normalizeV2Message(message, input.targetSessionID),
  );
  const summary = summarizeSessionMessages(messages);
  const resolvedStatus = resolveSessionStatusWithMessageSummary({
    status: hasDoneStatus ? "done" : undefined,
    summary,
  });

  if (resolvedStatus.status === "error") {
    return {
      status: "error",
      endedAt: resolvedStatus.endedAt,
      checkedMessages: true,
      canApplyStaleFallback: false,
    };
  }

  if (resolvedStatus.status === "done") {
    return {
      status: "done",
      endedAt: resolvedStatus.endedAt ?? new Date().toISOString(),
      checkedMessages: true,
      canApplyStaleFallback: false,
    };
  }

  if (
    hasRecentMessageActivity({
      nowMs: input.nowMs,
      latestMessageActivityAtMs: summary.latestMessageActivityAtMs,
      staleThresholdMs: STALE_RUNNING_THRESHOLD_MS,
    })
  ) {
    return {
      checkedMessages: true,
      sawRunningEvidence: true,
      endedAt: summary.latestMessageActivityAt,
      probeFailed,
      canApplyStaleFallback: false,
    };
  }

  return {
    checkedMessages: true,
    probeFailed,
    canApplyStaleFallback: !probeFailed,
  };
}

/**
 * v2 preference store. `ctx.storage.store` needs an object value, so booleans
 * are wrapped in `{ value }` while the key strings stay identical to v1 and the
 * default stays `true`.
 */
function createBooleanPreference(
  ctx: V2TuiContext,
  key: string,
): { get: () => boolean; set: (value: boolean) => void } {
  let store: unknown = { value: true };
  let setStore:
    | ((mutation: (draft: unknown) => void) => Promise<void>)
    | undefined;
  const result = safeRead(() => ctx.storage.store(key, { initial: { value: true } }));
  if (result) {
    store = result[0];
    setStore = result[1];
  }

  return {
    get: () => asRecord(store)?.value !== false,
    set: (value: boolean) => {
      if (!setStore) return;
      // The async setter is deliberately fire-and-forget, like v1 kv.set.
      void safeReadAsync(async () => {
        await setStore?.((draft: unknown) => {
          const record = asRecord(draft);
          if (record) record.value = value;
        });
      });
    },
  };
}

function initializeTui(ctx: V2TuiContext): () => void {
  const statePath = resolveStatePath();
  const textPath = resolveTextPath(statePath);
  const [state, setState] = createSignal<StatuslineState>(createEmptyState());
  const [nowMs, setNowMs] = createSignal(Date.now());
  const [hydratedSessions, setHydratedSessions] = createSignal<Set<string>>(
    new Set(),
  );
  const [hydratingSessions, setHydratingSessions] = createSignal<Set<string>>(
    new Set(),
  );
  const [hydrateRetryPendingSessions, setHydrateRetryPendingSessions] =
    createSignal<Set<string>>(new Set());
  const [hydrateRetryAttempts, setHydrateRetryAttempts] = createSignal<
    Map<string, number>
  >(new Map());
  const [hydrateRetryTick, setHydrateRetryTick] = createSignal(0);

  const expandedPreference = createBooleanPreference(
    ctx,
    SUBAGENTS_EXPANDED_KV_KEY,
  );
  const sectionPreference = createBooleanPreference(
    ctx,
    SUBAGENTS_SECTION_ENABLED_KV_KEY,
  );
  const subagentsExpanded = () => expandedPreference.get();
  const subagentsSectionEnabled = () => sectionPreference.get();

  const hydrateRetryTimeouts = new Map<string, ReturnType<typeof setTimeout>>();
  const runningReconcileBackoff = new Map<string, RunningReconcileCacheEntry>();
  let reconcileInFlight = false;
  let lastRunningReconcileAtMs = 0;
  let disposed = false;
  let previousRouteSessionID: string | undefined;
  let pendingSidebarRefocus: PendingSidebarRefocus | undefined;
  let pendingRefocusConsumed = false;

  const theme = createMemo(() => resolveTuiTheme(ctx.theme));
  const models = () =>
    safeRead(() => ctx.data.location.model.list()) ?? [];

  const consumePendingSidebarRefocus = ():
    | PendingSidebarRefocus
    | undefined => {
    if (pendingRefocusConsumed) return undefined;
    pendingRefocusConsumed = true;
    return pendingSidebarRefocus;
  };

  /**
   * Degradation: v1 returned focus to the prompt through `TuiPromptRef` after
   * closing the sidebar list. v2 exposes no prompt-focus API, so this is
   * intentionally a no-op; the list still blurs and selection still restores.
   */
  const focusActivePrompt = (): void => {};

  const rememberSidebarChildNavigation = (input: {
    parentSessionID: string;
    childSessionID: string;
    childRowID: string;
    showCompletedHistory: boolean;
  }): void => {
    pendingSidebarRefocus = input;
  };

  const setSubagentsExpandedPreference = (expanded: boolean): void => {
    expandedPreference.set(expanded);
    safeRead(() =>
      ctx.ui.toast.show({
        message: expanded
          ? "Subagent list expanded"
          : "Subagent list collapsed",
        variant: "info",
      }),
    );
  };

  const setSubagentsExpandedSilently = (expanded: boolean): void => {
    expandedPreference.set(expanded);
  };

  const setSubagentsSectionEnabledPreference = (enabled: boolean): void => {
    sectionPreference.set(enabled);
    safeRead(() =>
      ctx.ui.toast.show({
        message: enabled
          ? "Subagent section enabled"
          : "Subagent section disabled",
        variant: "info",
      }),
    );
  };

  const toggleSidebarListFocus = (): void => {
    safeRead(() => ctx.ui.dialog.clear());
    if (isAnySidebarSubagentListFocused()) {
      blurVisibleSidebarSubagentList();
      focusActivePrompt();
      return;
    }

    setSubagentsSectionEnabledPreference(true);
    setSubagentsExpandedSilently(true);
    setTimeout(() => {
      focusVisibleSidebarSubagentList();
    }, 0);
  };

  const toggleSidebarCompletedHistory = (): void => {
    safeRead(() => ctx.ui.dialog.clear());
    sectionPreference.set(true);
    expandedPreference.set(true);
    setTimeout(() => {
      toggleVisibleSidebarCompletedHistory();
    }, 0);
  };

  const commandDispose: TuiCommandDispose = registerSubagentCommandsV2({
    api: ctx.keymap,
    sectionEnabled: subagentsSectionEnabled,
    toggleSection: setSubagentsSectionEnabledPreference,
    focusSidebarList: toggleSidebarListFocus,
    toggleCompletedHistory: toggleSidebarCompletedHistory,
  });

  const clearHydrateRetryTimeout = (sessionID: string): void => {
    const timeout = hydrateRetryTimeouts.get(sessionID);
    if (timeout) {
      clearTimeout(timeout);
      hydrateRetryTimeouts.delete(sessionID);
    }
  };

  const resetHydrateRetry = (sessionID: string | undefined): void => {
    if (!sessionID) return;
    clearHydrateRetryTimeout(sessionID);
    setHydrateRetryPendingSessions((prev) => {
      if (!prev.has(sessionID)) return prev;
      const next = new Set(prev);
      next.delete(sessionID);
      return next;
    });
    setHydrateRetryAttempts((prev) => {
      if (!prev.has(sessionID)) return prev;
      const next = new Map(prev);
      next.delete(sessionID);
      return next;
    });
  };

  createEffect(() => {
    hydrateRetryTick();
    // Router current() is reactive; reading it here tracks session switches.
    const routeSessionID = resolveRouteSessionID(ctx);

    if (previousRouteSessionID && previousRouteSessionID !== routeSessionID) {
      resetHydrateRetry(previousRouteSessionID);
    }

    const siblingRefocus = resolveSiblingSidebarRefocus({
      pendingSidebarRefocus,
      routeSessionID,
      children: state().children,
    });
    if (siblingRefocus && pendingSidebarRefocus) {
      pendingSidebarRefocus = {
        ...pendingSidebarRefocus,
        ...siblingRefocus,
      };
    }

    const sidebarReturnAction = resolveSidebarReturnFocusAction({
      pendingSidebarRefocus,
      previousRouteSessionID,
      routeSessionID,
    });
    pendingRefocusConsumed = false;
    if (sidebarReturnAction === "focus-prompt") {
      blurVisibleSidebarSubagentList();
      focusActivePrompt();
    } else if (sidebarReturnAction === "clear-pending") {
      pendingSidebarRefocus = undefined;
    }

    previousRouteSessionID = routeSessionID;

    if (!routeSessionID) return;

    const sessionID = routeSessionID;
    if (
      hydratedSessions().has(sessionID) ||
      hydratingSessions().has(sessionID) ||
      hydrateRetryPendingSessions().has(sessionID)
    ) {
      return;
    }

    setHydratingSessions((prev) => {
      const next = new Set(prev);
      next.add(sessionID);
      return next;
    });

    void (async () => {
      const finishHydrating = (): void => {
        setHydratingSessions((prev) => {
          const next = new Set(prev);
          next.delete(sessionID);
          return next;
        });
      };

      const hydrated = await hydratePreviousSubagents(
        ctx,
        sessionID,
        statePath,
        textPath,
        setState,
      );
      if (disposed) {
        clearHydrateRetryTimeout(sessionID);
        finishHydrating();
        return;
      }
      if (hydrated) {
        resetHydrateRetry(sessionID);
        setHydratedSessions((prev) => {
          const next = new Set(prev);
          next.add(sessionID);
          return next;
        });
        finishHydrating();
        return;
      }

      const attempts = hydrateRetryAttempts().get(sessionID) ?? 0;

      const delayMs = Math.min(
        HYDRATE_RETRY_MAX_DELAY_MS,
        HYDRATE_RETRY_BASE_DELAY_MS * 2 ** attempts,
      );

      setHydrateRetryAttempts((prev) => {
        const next = new Map(prev);
        next.set(sessionID, Math.min(attempts + 1, HYDRATE_RETRY_MAX_ATTEMPTS));
        return next;
      });

      setHydrateRetryPendingSessions((prev) => {
        const next = new Set(prev);
        next.add(sessionID);
        return next;
      });
      finishHydrating();

      clearHydrateRetryTimeout(sessionID);
      const timeout = setTimeout(() => {
        hydrateRetryTimeouts.delete(sessionID);
        setHydrateRetryPendingSessions((prev) => {
          if (!prev.has(sessionID)) return prev;
          const next = new Set(prev);
          next.delete(sessionID);
          return next;
        });
        if (disposed) return;
        setHydrateRetryTick((value) => value + 1);
      }, delayMs);
      hydrateRetryTimeouts.set(sessionID, timeout);
    })();
  });

  const reconcileRunningChildren = async (): Promise<void> => {
    if (reconcileInFlight || disposed) return;
    reconcileInFlight = true;
    lastRunningReconcileAtMs = Date.now();

    try {
      const snapshot = cloneState(state());
      const nowMs = Date.now();
      const currentSessionID = resolveRouteSessionID(ctx);

      const selected = selectRunningReconcileCandidates({
        state: snapshot,
        currentSessionID,
        nowMs,
        maxCandidates: RUNNING_RECONCILE_MAX_CANDIDATES,
      });

      const mutations: Array<{
        childID: string;
        targetSessionID: string;
        status: "done" | "error";
        endedAt?: string;
        reconcileWithoutTargetSessionID?: boolean;
      }> = [];

      const parentMessagesCache = new Map<string, unknown[] | null>();

      for (const candidate of selected) {
        const key = candidate.targetSessionID ?? candidate.childID;
        const cache = runningReconcileBackoff.get(key);
        if (shouldSkipCandidateForBackoff(cache, nowMs)) continue;

        if (!candidate.targetSessionID) {
          const isPersistedSubtaskCandidate =
            candidate.source === "subtask" &&
            typeof candidate.parentID === "string" &&
            candidate.parentID.length > 0 &&
            typeof candidate.messageID === "string" &&
            candidate.messageID.length > 0;
          if (!isPersistedSubtaskCandidate) continue;

          const parentSessionID = candidate.parentID as string;
          let parentMessages = parentMessagesCache.get(parentSessionID);
          if (parentMessages === undefined) {
            const raw = await readSessionMessages(ctx, parentSessionID);
            parentMessages =
              raw === undefined ? null : raw.map(normalizeV2Message);
            parentMessagesCache.set(parentSessionID, parentMessages);
          }
          if (parentMessages === null) {
            runningReconcileBackoff.set(
              key,
              nextBackoffState({
                cache,
                nowMs,
                initialBackoffMs: RUNNING_RECONCILE_INITIAL_BACKOFF_MS,
                maxBackoffMs: RUNNING_RECONCILE_MAX_BACKOFF_MS,
              }),
            );
            continue;
          }

          const evidence = resolvePersistedStaleSubtaskFromParentMessages({
            candidate: {
              childID: candidate.childID,
              parentID: candidate.parentID as string,
              messageID: candidate.messageID as string,
              title: candidate.title,
              summary: candidate.summary,
              agentName: candidate.agentName,
            } satisfies PersistedStaleSubtaskCandidate,
            messages: parentMessages,
          });
          if (!evidence) {
            const parentSummary = summarizeSessionMessages(parentMessages);
            const canSafelyFallbackByParentInactivity =
              canSafelyCloseNoTargetPersistedCandidate({
                nowMs,
                staleThresholdMs: STALE_RUNNING_THRESHOLD_MS,
                startedMs: candidate.startedMs,
                updatedMs: candidate.updatedMs,
                latestMessageActivityAtMs:
                  parentSummary.latestMessageActivityAtMs,
              });
            if (canSafelyFallbackByParentInactivity) {
              mutations.push({
                childID: candidate.childID,
                targetSessionID: candidate.childID,
                status: "done",
                endedAt:
                  parentSummary.latestMessageActivityAt ??
                  new Date(nowMs - candidate.updatedMs).toISOString(),
                reconcileWithoutTargetSessionID: true,
              });
              runningReconcileBackoff.delete(key);
              continue;
            }
            runningReconcileBackoff.set(
              key,
              nextBackoffState({
                cache,
                nowMs,
                initialBackoffMs: RUNNING_RECONCILE_INITIAL_BACKOFF_MS,
                maxBackoffMs: RUNNING_RECONCILE_MAX_BACKOFF_MS,
              }),
            );
            continue;
          }

          mutations.push({
            childID: candidate.childID,
            targetSessionID: evidence.targetSessionID ?? candidate.childID,
            status: evidence.status,
            endedAt: evidence.endedAt,
            reconcileWithoutTargetSessionID: true,
          });
          runningReconcileBackoff.delete(key);
          continue;
        }

        const evidence = await probeRunningEvidence({
          ctx,
          targetSessionID: candidate.targetSessionID,
          candidateAgeMs: Math.max(candidate.startedMs, candidate.updatedMs),
          nowMs,
        });

        if (evidence.status === "done" || evidence.status === "error") {
          mutations.push({
            childID: candidate.childID,
            targetSessionID: candidate.targetSessionID,
            status: evidence.status,
            endedAt: evidence.endedAt,
          });
          runningReconcileBackoff.delete(key);
          continue;
        }

        if (evidence.sawRunningEvidence) {
          runningReconcileBackoff.set(key, {
            backoffMs: RUNNING_RECONCILE_INITIAL_BACKOFF_MS,
            nextAllowedAtMs: nowMs + RUNNING_RECONCILE_INITIAL_BACKOFF_MS,
          });
          continue;
        }

        const shouldApplyFallback = shouldApplyStaleRunningFallback({
          staleThresholdMs: STALE_RUNNING_THRESHOLD_MS,
          evidence,
          startedMs: candidate.startedMs,
          updatedMs: candidate.updatedMs,
        });

        if (shouldApplyFallback) {
          mutations.push({
            childID: candidate.childID,
            targetSessionID: candidate.targetSessionID,
            status: "done",
            endedAt: new Date(nowMs - candidate.updatedMs).toISOString(),
          });
          runningReconcileBackoff.delete(key);
          continue;
        }

        runningReconcileBackoff.set(
          key,
          nextBackoffState({
            cache,
            nowMs,
            initialBackoffMs: RUNNING_RECONCILE_INITIAL_BACKOFF_MS,
            maxBackoffMs: RUNNING_RECONCILE_MAX_BACKOFF_MS,
          }),
        );
      }

      if (mutations.length === 0) return;

      snapshotSidebarScrollOffsets();
      setState((current: StatuslineState) => {
        const next = cloneState(current);
        let changed = false;

        for (const mutation of mutations) {
          if (
            mutation.reconcileWithoutTargetSessionID &&
            mutation.targetSessionID.startsWith("ses_")
          ) {
            changed =
              upsertChildDetails(next, mutation.childID, {
                targetSessionID: mutation.targetSessionID,
                updatedAt: mutation.endedAt,
              }) || changed;
          }
          if (
            markChildStatus(
              next,
              mutation.reconcileWithoutTargetSessionID
                ? mutation.childID
                : mutation.targetSessionID,
              mutation.status,
              mutation.endedAt,
            )
          ) {
            changed = true;
          }
        }

        const refreshed = refreshLiveState(next);
        if (!changed && !refreshed) return current;
        persistStateSnapshot(statePath, textPath, next);
        return next;
      });
    } finally {
      reconcileInFlight = false;
    }
  };

  const timers = createTuiMaintenanceTimers({
    onElapsedTick: () => {
      snapshotSidebarScrollOffsets();
      setNowMs(Date.now());
    },
    onMaintenanceTick: () => {
      const currentNowMs = Date.now();
      if (
        currentNowMs - lastRunningReconcileAtMs >=
        RUNNING_RECONCILE_MAINTENANCE_INTERVAL_MS
      ) {
        void reconcileRunningChildren();
      }

      void (async () => {
        // Hydrate on a probe of a state snapshot; the data-store reads are
        // async (sync-before-list). Merge only the token deltas into the live
        // state, never replacing it: events that land during the reads must
        // survive. Derived fields are recomputed from the live state.
        const probe = await runTuiStateMaintenance(ctx, state());
        setState((current: StatuslineState) => {
          const next = cloneState(current);
          const tokenChanged = mergeHydratedTokens(next, probe);
          const refreshed = refreshLiveState(next);
          if (!tokenChanged && !refreshed) return current;
          snapshotSidebarScrollOffsets();
          persistStateSnapshot(statePath, textPath, next);
          return next;
        });
      })();
    },
  });

  createEffect(() => {
    timers.syncElapsedTimer(
      Object.values(state().children).some(
        (child) => child.status === "running",
      ),
    );
  });

  // v2 events are adapted to the v1-shaped internal events the core
  // reduces; one raw event can produce several internal events.
  const adapter = createV2EventAdapter();

  const applyRawEvent = (raw: unknown): void => {
    debugEvent(raw);
    const events = adapter.adapt(raw);
    if (events.length === 0) return;

    void (async () => {
      // Hydrate on a probe of a state snapshot first: the data-store reads are
      // async (sync-before-list). The event application stays synchronous
      // inside setState; hydrating a probe keeps events that arrive during the
      // reads from being lost, and only the token deltas are merged back.
      const probe = cloneState(state());
      await hydrateStateTokensFromData(ctx, probe);

      snapshotSidebarScrollOffsets();
      setState((current: StatuslineState) => {
        const next = cloneState(current);
        let changed = false;
        for (const event of events) {
          changed = applySubagentEvent(next, event) || changed;
        }
        const tokenChanged = mergeHydratedTokens(next, probe);
        const refreshed = refreshLiveState(next);
        if (!changed && !tokenChanged && !refreshed) return current;
        persistStateSnapshot(statePath, textPath, next);
        return next;
      });
    })();
  };

  const disposers: Array<() => void> = [];
  for (const type of V2_SUBSCRIBED_EVENTS) {
    try {
      const unregister = ctx.data.on(type, applyRawEvent);
      if (typeof unregister === "function") disposers.push(unregister);
    } catch {
      // A host that rejects one event type must not lose the others.
    }
  }

  const unregisterSlots: Array<() => void> = [];
  const registerSlot = (claim: unknown): void => {
    try {
      const unregister = ctx.ui.slot(claim);
      if (typeof unregister === "function") unregisterSlots.push(unregister);
    } catch {
      // A host without this slot degrades additively; never crash setup.
    }
  };

  registerSlot({
    append: "sidebar.content",
    render: (input: { sessionID?: string }) => {
      const routeSessionID = resolveRouteSessionID(ctx);
      const sessionID = input?.sessionID ?? routeSessionID ?? "";
      const restoreFromChild = (() => {
        const pending = consumePendingSidebarRefocus();
        if (pending?.parentSessionID !== sessionID) return undefined;
        return {
          childRowID: pending.childRowID,
          showCompletedHistory: pending.showCompletedHistory ?? false,
        };
      })();
      return (
        <Show when={subagentsSectionEnabled()}>
          <SidebarSubagents
            sessionID={sessionID}
            state={state}
            nowMs={nowMs}
            models={models}
            expanded={subagentsExpanded}
            onToggleExpanded={() =>
              setSubagentsExpandedPreference(!subagentsExpanded())
            }
            onSetExpanded={setSubagentsExpandedSilently}
            onReturnFocus={focusActivePrompt}
            onToggleListFocus={toggleSidebarListFocus}
            onOpenSession={(target) =>
              navigateToSessionTarget(ctx, target)
            }
            onNavigateToChild={rememberSidebarChildNavigation}
            sidebarWidth={() =>
              resolveSidebarWidth(ctx.renderer) ?? resolveSidebarWidth(input)
            }
            theme={theme()}
            keymap={ctx.keymap}
            restoreFromChild={restoreFromChild}
          />
        </Show>
      );
    },
  });

  registerSlot({
    append: "home.footer",
    render: () => (
      <HomeBottomStatus state={state} theme={theme()} />
    ),
  });

  // Additive compact summary: `after` keeps the host's own status content.
  registerSlot({
    after: "prompt.footer.status",
    render: () => <CompactSummary state={state} theme={theme()} />,
  });

  return () => {
    disposed = true;
    timers.dispose();
    for (const timeout of hydrateRetryTimeouts.values()) {
      clearTimeout(timeout);
    }
    hydrateRetryTimeouts.clear();
    commandDispose();
    for (const dispose of disposers) {
      try {
        dispose();
      } catch {
        // Cleanup is best-effort.
      }
    }
    for (const unregister of unregisterSlots) {
      try {
        unregister();
      } catch {
        // Cleanup is best-effort.
      }
    }
  };
}

/**
 * v2 TUI entry. Reactive work lives inside a solid-js root; the returned
 * cleanup disposes that root and every host registration.
 */
export const subagentStatuslineTuiPlugin = {
  id: TUI_PLUGIN_ID,
  setup(ctx: V2TuiContext): () => void {
    let cleanup = (): void => {};
    createRoot((disposeRoot) => {
      const innerCleanup = initializeTui(ctx);
      cleanup = () => {
        innerCleanup();
        disposeRoot();
      };
    });
    return cleanup;
  },
};

export default subagentStatuslineTuiPlugin;
