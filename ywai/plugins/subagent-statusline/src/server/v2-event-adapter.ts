/**
 * OpenCode v2 → v1-shaped event adapter for the statusline core.
 *
 * OpenCode v2 delivers server events as an async stream of envelopes:
 *
 *   { id, created, type, data }        (payload lives in `data`)
 *
 * The state machine (`src/events.ts`, `applySubagentEvent`) consumes
 * the v1 wire shape instead:
 *
 *   { type, properties: { info?, part?, status?, ... } }
 *
 * This module is the only place that knows either shape. It is a pure
 * translation layer: for every recognized v2 event it emits zero or more
 * v1-shaped `EventLike` objects that `applySubagentEvent` already knows how
 * to reduce. It never throws on unknown or malformed input and it keeps no
 * state on disk; the only memory is a bounded map of tool-call ids → names
 * (v2 `session.tool.success`/`failed` no longer carry the tool name, so the
 * name seen on `session.tool.input.started` is remembered to rebuild the
 * v1 `part.tool` field).
 *
 * Payload shapes below were verified against
 * @opencode/protocol@0.0.0-beta (dist/groups/event.d.ts).
 */

import type { EventLike } from "../events.js";
import { lookupParentSessionID } from "./parent-lookup";

/** Structural slice of the v2 event envelope. Never imported at runtime. */
export type V2ServerEvent = {
  type?: unknown;
  data?: unknown;
  [key: string]: unknown;
};

export type V2EventAdapter = {
  /** Translate one raw stream item into 0..n v1-shaped events. Never throws. */
  adapt(raw: unknown): EventLike[];
};

type ToolCallMemory = {
  name: string;
  sessionID: string;
  assistantMessageID?: string;
};

const MAX_REMEMBERED_TOOL_CALLS = 512;

function isRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === "object" && !Array.isArray(value);
}

function asString(value: unknown): string | undefined {
  return typeof value === "string" && value.length > 0 ? value : undefined;
}

/** Wrap already-validated fields into an internal event (no fresh literal). */
function internalEvent(
  type: string,
  properties: Record<string, unknown>,
): EventLike {
  return { type, properties };
}

/** v2 payload: `data` first; tolerate a v1-style `properties` envelope too. */
function payloadOf(event: V2ServerEvent): Record<string, unknown> {
  const data = event.data ?? event.properties;
  return isRecord(data) ? data : {};
}

/** Join the text entries of a v2 content array into one output string. */
function contentText(value: unknown): string | undefined {
  if (!Array.isArray(value)) return undefined;
  const texts: string[] = [];
  for (const entry of value) {
    if (isRecord(entry) && entry.type === "text") {
      const text = asString(entry.text);
      if (text) texts.push(text);
    }
  }
  return texts.length > 0 ? texts.join("\n") : undefined;
}

/** Map a v2 tool state status onto the v1 vocabulary the core understands. */
function v1ToolStatus(value: unknown): "running" | "completed" | "error" {
  if (value === "completed" || value === "error") return value;
  // "streaming" (input still arriving) and anything unknown count as running.
  return "running";
}

function sessionProperties(
  data: Record<string, unknown>,
): { sessionID: string; base: Record<string, unknown> } | undefined {
  const sessionID = asString(data.sessionID);
  if (!sessionID) return undefined;
  return {
    sessionID,
    base: { id: sessionID, sessionID },
  };
}

/**
 * Build a v1 `message.part.updated` around a v1-shaped tool part.
 * `info` deliberately carries no `role`/model fields: the detail-target path
 * (`extractSessionID`) is the only consumer that should fire for it.
 */
function partUpdatedEvent(input: {
  sessionID: string;
  messageID?: string;
  part: Record<string, unknown>;
}): EventLike {
  const info: Record<string, unknown> = { sessionID: input.sessionID };
  if (input.messageID) info.id = input.messageID;
  return internalEvent("message.part.updated", {
    id: input.sessionID,
    sessionID: input.sessionID,
    info,
    part: input.part,
  });
}

/** Translate a v2 tool part (from a content snapshot) into a v1 tool part. */
function v1PartFromV2ToolPart(input: {
  sessionID: string;
  messageID?: string;
  partID: string;
  name: string;
  state: Record<string, unknown>;
  partTime?: Record<string, unknown>;
}): Record<string, unknown> {
  const status = v1ToolStatus(input.state.status);
  const stateInput = isRecord(input.state.input) ? input.state.input : {};
  const metadata = isRecord(input.state.metadata) ? input.state.metadata : {};
  // v1 `part.state.output` is how task-tool target sessions get recovered
  // (the core parses `ses_…` ids out of it); v2 puts tool output in
  // `state.content` for completed/failed calls.
  const output =
    contentText(input.state.content) ?? asString(input.state.output);
  const time = input.partTime;

  const state: Record<string, unknown> = { status, input: stateInput };
  if (Object.keys(metadata).length > 0) state.metadata = metadata;
  if (output !== undefined) state.output = output;
  if (isRecord(input.state.error)) state.error = input.state.error;
  if (time) state.time = time;

  const part: Record<string, unknown> = {
    type: "tool",
    tool: input.name,
    id: input.partID,
    sessionID: input.sessionID,
    messageID: input.messageID,
    state,
  };
  if (time) part.time = time;
  return part;
}

function createAdapter(): V2EventAdapter {
  const toolCalls = new Map<string, ToolCallMemory>();

  function rememberToolCall(id: string, memory: ToolCallMemory): void {
    if (toolCalls.size >= MAX_REMEMBERED_TOOL_CALLS && !toolCalls.has(id)) {
      const oldest = toolCalls.keys().next().value;
      if (oldest !== undefined) toolCalls.delete(oldest);
    }
    toolCalls.set(id, memory);
  }

  function adaptSessionCreated(
    data: Record<string, unknown>,
    created: unknown,
  ): EventLike[] {
    // Only sessions with a parent are tracked (subagent sessions);
    // root sessions are ignored by `extractCreatedChild` anyway.
    const sessionID = asString(data.sessionID);
    if (!sessionID) return [];

    // v2 publishes session.created before anything sets a parent, so a
    // subagent arrives here looking exactly like a root session. Remember it
    // instead of dropping it: the edge is written moments later, and the next
    // event for this session is the chance to notice.
    const parentID = asString(data.parentID);
    if (!parentID) {
      rememberUnparented(sessionID, data, created);
      return [];
    }

    const info: Record<string, unknown> = {
      id: sessionID,
      sessionID,
      parentID,
    };
    const title = asString(data.title);
    if (title) info.title = title;
    const agent = asString(data.agent);
    if (agent) info.agent = agent;
    if (typeof created === "number" && Number.isFinite(created)) {
      info.time = { created };
    }

    return [
      internalEvent("session.created", {
        id: sessionID,
        sessionID,
        parentID,
        info,
      }),
    ];
  }

  function adaptUsageUpdated(data: Record<string, unknown>): EventLike[] {
    const session = sessionProperties(data);
    if (!session) return [];
    const tokens = isRecord(data.tokens) ? data.tokens : {};
    const input = typeof tokens.input === "number" ? tokens.input : undefined;
    const output = typeof tokens.output === "number" ? tokens.output : undefined;
    const reasoning =
      typeof tokens.reasoning === "number" ? tokens.reasoning : undefined;
    const parts = [input, output, reasoning].filter(
      (value): value is number => value !== undefined,
    );
    const usage: Record<string, unknown> = {};
    if (input !== undefined) usage.inputTokens = input;
    if (output !== undefined) usage.outputTokens = output;
    if (reasoning !== undefined) usage.reasoningTokens = reasoning;
    if (parts.length > 0) {
      usage.totalTokens = parts.reduce((sum, value) => sum + value, 0);
    }
    if (Object.keys(usage).length === 0) return [];

    // Routed through the internal `message.updated` detail path so token
    // hints land on the tracked child with this sessionID (the walk in
    // `extractChildDetails` recognizes `*Tokens` keys).
    const properties: Record<string, unknown> = {
      ...session.base,
      info: { ...session.base },
      usage,
    };
    return [internalEvent("message.updated", properties)];
  }

  function adaptRenamed(data: Record<string, unknown>): EventLike[] {
    const session = sessionProperties(data);
    const title = asString(data.title);
    if (!session || !title) return [];
    // Also routed through the internal detail path: `info.title` is picked
    // up by `extractChildDetails` and applied to the tracked child.
    return [
      internalEvent("message.updated", {
        ...session.base,
        info: { ...session.base, title },
      }),
    ];
  }

  function adaptStepStarted(
    data: Record<string, unknown>,
    created: unknown,
  ): EventLike[] {
    const sessionID = asString(data.sessionID);
    const model = isRecord(data.model) ? data.model : {};
    const modelID = asString(model.id);
    const providerID = asString(model.providerID);
    if (!sessionID || !modelID || !providerID) return [];

    const info: Record<string, unknown> = {
      id: asString(data.assistantMessageID) ?? sessionID,
      sessionID,
      role: "assistant",
      modelID,
      providerID,
    };
    const variant = asString(model.variant);
    if (variant) info.variant = variant;
    if (typeof created === "number" && Number.isFinite(created)) {
      info.time = { created };
    }

    // Internal `message.updated` + assistant info drives
    // `extractLatestAssistantModel` → `setChildModel` for this session.
    return [
      internalEvent("message.updated", {
        id: sessionID,
        sessionID,
        info,
      }),
    ];
  }

  function adaptContentUpdated(data: Record<string, unknown>): EventLike[] {
    const sessionID = asString(data.sessionID);
    if (!sessionID) return [];
    const messageID = asString(data.messageID);
    const content = Array.isArray(data.content) ? data.content : [];

    const events: EventLike[] = [];
    for (const entry of content) {
      if (!isRecord(entry) || entry.type !== "tool") continue;
      const partID = asString(entry.id);
      const name = asString(entry.name);
      if (!partID || !name) continue;

      rememberToolCall(partID, { name, sessionID, assistantMessageID: messageID });
      events.push(
        partUpdatedEvent({
          sessionID,
          messageID,
          part: v1PartFromV2ToolPart({
            sessionID,
            messageID,
            partID,
            name,
            state: isRecord(entry.state) ? entry.state : {},
            partTime: isRecord(entry.time) ? entry.time : undefined,
          }),
        }),
      );
    }
    return events;
  }

  function adaptToolInputStarted(data: Record<string, unknown>): EventLike[] {
    const sessionID = asString(data.sessionID);
    const id = asString(data.id);
    const name = asString(data.name);
    const assistantMessageID = asString(data.assistantMessageID);
    if (!sessionID || !id || !name) return [];

    rememberToolCall(id, { name, sessionID, assistantMessageID });
    // Early `running` tool child so a delegation is visible while the input
    // is still streaming; the core ignores tools other than task/delegate.
    return [
      partUpdatedEvent({
        sessionID,
        messageID: assistantMessageID,
        part: {
          type: "tool",
          tool: name,
          id,
          sessionID,
          messageID: assistantMessageID,
          state: { status: "running" },
        },
      }),
    ];
  }

  function adaptToolTerminal(
    data: Record<string, unknown>,
    status: "completed" | "error",
  ): EventLike[] {
    const id = asString(data.id);
    if (!id) return [];
    const memory = toolCalls.get(id);
    if (!memory) return [];

    const sessionID = asString(data.sessionID) ?? memory.sessionID;
    const assistantMessageID =
      asString(data.assistantMessageID) ?? memory.assistantMessageID;
    const metadata = isRecord(data.metadata) ? data.metadata : {};
    const output = contentText(data.content);
    const state: Record<string, unknown> = { status, metadata };
    if (output !== undefined) state.output = output;
    if (isRecord(data.error)) state.error = data.error;

    return [
      partUpdatedEvent({
        sessionID,
        messageID: assistantMessageID,
        part: {
          type: "tool",
          tool: memory.name,
          id,
          sessionID,
          messageID: assistantMessageID,
          state,
        },
      }),
    ];
  }

  /**
   * Sessions seen at creation with no parent yet. Bounded like the tool-call
   * memory: a session that never turns out to be a subagent must not pin
   * memory for the life of the process.
   */
  const unparented = new Map<string, { data: Record<string, unknown>; created: unknown }>();

  function rememberUnparented(
    sessionID: string,
    data: Record<string, unknown>,
    created: unknown,
  ): void {
    if (unparented.size >= MAX_REMEMBERED_TOOL_CALLS && !unparented.has(sessionID)) {
      const oldest = unparented.keys().next().value;
      if (oldest !== undefined) unparented.delete(oldest);
    }
    unparented.set(sessionID, { data, created });
  }

  /**
   * Emit the deferred session.created for a session whose parent has since
   * been recorded. Returns the events to prepend, so a terminal event for a
   * subagent the adapter had written off still reduces against a known child.
   * Each session is resolved at most once, whatever the answer.
   */
  function resolveDeferredCreation(sessionID: string): EventLike[] {
    const pending = unparented.get(sessionID);
    if (!pending) return [];

    const parentID = lookupParentSessionID(sessionID);
    if (!parentID) return [];

    unparented.delete(sessionID);
    return adaptSessionCreated(
      { ...pending.data, parentID },
      pending.created,
    );
  }

  function adapt(raw: unknown): EventLike[] {
    try {
      if (!isRecord(raw)) return [];
      const type = asString(raw.type);
      if (!type) return [];
      const data = payloadOf(raw);
      const created = raw.created;

      // Any event other than the creation itself is a later moment, so it is
      // also the first chance to learn that this session became a child.
      const deferred =
        type === "session.created"
          ? []
          : resolveDeferredCreation(asString(data.sessionID) ?? "");

      const mapped = ((): EventLike[] => {
      switch (type) {
        case "session.created":
          return adaptSessionCreated(data, created);
        case "session.status": {
          const session = sessionProperties(data);
          const status = data.status;
          if (!session) return [];
          if (!isRecord(status) && typeof status !== "string") return [];
          return [
            internalEvent("session.status", {
              ...session.base,
              info: { ...session.base },
              status,
            }),
          ];
        }
        case "session.idle": {
          const session = sessionProperties(data);
          if (!session) return [];
          return [
            internalEvent("session.idle", {
              ...session.base,
              info: { ...session.base },
            }),
          ];
        }
        case "session.execution.failed": {
          const session = sessionProperties(data);
          if (!session) return [];
          return [
            internalEvent("session.error", {
              ...session.base,
              info: { ...session.base },
              error: data.error,
            }),
          ];
        }
        // In opencode2 the durable terminal events are session.execution.*:
        // `session.idle` and `session.status` have no V2 publisher (the app
        // keys "session done" on execution.succeeded / .interrupted). A
        // successful run reduces through the internal session.idle path, which
        // the vendored core already maps to done; an interruption reduces
        // through session.error (aborted/cancelled is an error status).
        case "session.execution.succeeded": {
          const session = sessionProperties(data);
          if (!session) return [];
          return [
            internalEvent("session.idle", {
              ...session.base,
              info: { ...session.base },
            }),
          ];
        }
        case "session.execution.interrupted": {
          const session = sessionProperties(data);
          if (!session) return [];
          // "superseded" means a newer execution replaced this one: the
          // session is still running, so only the other interruption reasons
          // are terminal.
          if (data.reason === "superseded") return [];
          return [
            internalEvent("session.error", {
              ...session.base,
              info: { ...session.base },
            }),
          ];
        }
        case "session.usage.updated":
          return adaptUsageUpdated(data);
        case "session.renamed":
          return adaptRenamed(data);
        case "session.step.started":
          return adaptStepStarted(data, created);
        case "session.message.content.updated":
          return adaptContentUpdated(data);
        case "session.tool.input.started":
          return adaptToolInputStarted(data);
        case "session.tool.called": {
          // Carries the parsed input; only useful when the tool name is
          // already remembered from `session.tool.input.started`.
          const id = asString(data.id);
          const memory = id ? toolCalls.get(id) : undefined;
          if (!id || !memory) return [];
          const sessionID = asString(data.sessionID) ?? memory.sessionID;
          const assistantMessageID =
            asString(data.assistantMessageID) ?? memory.assistantMessageID;
          return [
            partUpdatedEvent({
              sessionID,
              messageID: assistantMessageID,
              part: {
                type: "tool",
                tool: memory.name,
                id,
                sessionID,
                messageID: assistantMessageID,
                state: {
                  status: "running",
                  input: isRecord(data.input) ? data.input : {},
                },
              },
            }),
          ];
        }
        case "session.tool.success":
          return adaptToolTerminal(data, "completed");
        case "session.tool.failed":
          return adaptToolTerminal(data, "error");
        default:
          // Unknown/unmapped v2 types are intentionally ignored.
          return [];
      }
      })();

      return deferred.length > 0 ? [...deferred, ...mapped] : mapped;
    } catch {
      // Defensive by design: a malformed event must never take the plugin down.
      return [];
    }
  }

  return { adapt };
}

export function createV2EventAdapter(): V2EventAdapter {
  return createAdapter();
}
