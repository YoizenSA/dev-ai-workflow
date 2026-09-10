import { describe, expect, it } from "vitest";
import { createV2EventAdapter } from "./v2-event-adapter.js";
import { applySubagentEvent } from "../events.js";
import { createEmptyState, getCounts } from "../state.js";

// Replay of the smoke run's event sequence (VERDICT.md F2): a child session is
// created, the parent runs a task tool against it, the child produces tokens,
// and the child's execution succeeds. Before the fix the v2 adapter ignored
// session.execution.succeeded, so the child stayed "running" forever even
// though the parent received its result. The payloads mirror
// @opencode/protocol@0.0.0-beta (dist/groups/event.d.ts).

const CREATED_MS = 1765030400000;

function v2Event(type: string, data: unknown): unknown {
  return { id: "evt_1", created: CREATED_MS, type, data };
}

function createdChildEvent(): unknown {
  return v2Event("session.created", {
    sessionID: "ses_child_1",
    projectID: "prj_1",
    slug: "run-echo",
    title: "Run echo subagent-ok",
    agent: "general",
    parentID: "ses_parent_1",
    version: "1",
  });
}

describe("v2 adapter: execution terminal events", () => {
  it("maps session.execution.succeeded to the internal session.idle shape", () => {
    const adapter = createV2EventAdapter();
    const events = adapter.adapt(
      v2Event("session.execution.succeeded", { sessionID: "ses_child_1" }),
    );
    expect(events).toEqual([
      {
        type: "session.idle",
        properties: {
          id: "ses_child_1",
          sessionID: "ses_child_1",
          info: { id: "ses_child_1", sessionID: "ses_child_1" },
        },
      },
    ]);
  });

  it("maps session.execution.interrupted to the internal session.error shape", () => {
    const adapter = createV2EventAdapter();
    const events = adapter.adapt(
      v2Event("session.execution.interrupted", {
        sessionID: "ses_child_1",
        reason: "user",
      }),
    );
    expect(events).toEqual([
      {
        type: "session.error",
        properties: {
          id: "ses_child_1",
          sessionID: "ses_child_1",
          info: { id: "ses_child_1", sessionID: "ses_child_1" },
        },
      },
    ]);
  });

  it("ignores session.execution.interrupted when a newer run superseded it", () => {
    const adapter = createV2EventAdapter();
    expect(
      adapter.adapt(
        v2Event("session.execution.interrupted", {
          sessionID: "ses_child_1",
          reason: "superseded",
        }),
      ),
    ).toEqual([]);
  });

  it("ignores execution events without a session id", () => {
    const adapter = createV2EventAdapter();
    expect(
      adapter.adapt(v2Event("session.execution.succeeded", {})),
    ).toEqual([]);
    expect(
      adapter.adapt(
        v2Event("session.execution.interrupted", { reason: "user" }),
      ),
    ).toEqual([]);
  });
});

describe("v2 server half: observed smoke sequence reaches done", () => {
  it("flips the child to done when session.execution.succeeded arrives", () => {
    const adapter = createV2EventAdapter();
    const state = createEmptyState();

    const apply = (raw: unknown): void => {
      for (const event of adapter.adapt(raw)) {
        applySubagentEvent(state, event);
      }
    };

    // 1. Child session created (parent runs a subagent).
    apply(createdChildEvent());
    expect(state.children["ses_child_1"]).toMatchObject({ status: "running" });

    // 2. Parent task tool: input streaming, parsed call, live content part.
    apply(
      v2Event("session.tool.input.started", {
        sessionID: "ses_parent_1",
        id: "too_1",
        name: "task",
        assistantMessageID: "msg_1",
      }),
    );
    apply(
      v2Event("session.tool.called", {
        id: "too_1",
        sessionID: "ses_parent_1",
        input: {
          prompt: "Run echo subagent-ok and report its output.",
          description: "Run echo subagent-ok",
          subagent_type: "general",
        },
        executed: false,
        assistantMessageID: "msg_1",
      }),
    );
    apply(
      v2Event("session.message.content.updated", {
        sessionID: "ses_parent_1",
        messageID: "msg_1",
        content: [
          {
            id: "too_1",
            type: "tool",
            name: "task",
            time: { created: CREATED_MS },
            state: {
              status: "running",
              input: { prompt: "Run echo subagent-ok and report its output." },
              metadata: { sessionId: "ses_child_1" },
            },
          },
        ],
      }),
    );

    // 3. Child turns: model selection and usage.
    apply(
      v2Event("session.step.started", {
        sessionID: "ses_child_1",
        model: { id: "deepseek-v4-flash", providerID: "opencode-admin" },
        agent: "general",
        assistantMessageID: "msg_child_1",
      }),
    );
    apply(
      v2Event("session.usage.updated", {
        sessionID: "ses_child_1",
        cost: 0.01,
        tokens: { input: 8656, output: 48, reasoning: 16 },
      }),
    );

    // 4. Parent task tool completes with the child's session id.
    apply(
      v2Event("session.tool.success", {
        id: "too_1",
        sessionID: "ses_parent_1",
        content: [{ type: "text", text: "task_id: ses_child_1 done" }],
        executed: true,
        assistantMessageID: "msg_1",
      }),
    );

    // Before the terminal execution event the child is still running and the
    // live counts show the smoke symptom: 1 running · 0 done.
    expect(state.children["ses_child_1"]).toMatchObject({
      status: "running",
      model: { modelID: "deepseek-v4-flash", providerID: "opencode-admin" },
    });
    expect(state.children["tool:too_1"]).toMatchObject({
      status: "done",
      targetSessionID: "ses_child_1",
    });
    expect(getCounts(state)).toEqual({ running: 1, done: 0, error: 0 });

    // 5. The child's execution succeeds: this is the event the run actually
    // emits in opencode2, and the fix maps it to the done transition.
    apply(
      v2Event("session.execution.succeeded", { sessionID: "ses_child_1" }),
    );

    const child = state.children["ses_child_1"];
    expect(child).toMatchObject({
      status: "done",
      color: "green",
      title: "Run echo subagent-ok",
    });
    expect(child?.endedAt).toBeDefined();
    expect(state.totalExecuted).toBe(1);
    expect(getCounts(state)).toEqual({ running: 0, done: 1, error: 0 });
  });

  it("marks an interrupted child execution as error", () => {
    const adapter = createV2EventAdapter();
    const state = createEmptyState();
    for (const event of adapter.adapt(createdChildEvent())) {
      applySubagentEvent(state, event);
    }
    for (const event of adapter.adapt(
      v2Event("session.execution.interrupted", {
        sessionID: "ses_child_1",
        reason: "user",
      }),
    )) {
      applySubagentEvent(state, event);
    }
    expect(state.children["ses_child_1"]).toMatchObject({ status: "error" });
    expect(getCounts(state)).toEqual({ running: 0, done: 0, error: 1 });
  });

  it("keeps the child running when the interrupted reason is superseded", () => {
    const adapter = createV2EventAdapter();
    const state = createEmptyState();
    for (const event of adapter.adapt(createdChildEvent())) {
      applySubagentEvent(state, event);
    }
    for (const event of adapter.adapt(
      v2Event("session.execution.interrupted", {
        sessionID: "ses_child_1",
        reason: "superseded",
      }),
    )) {
      applySubagentEvent(state, event);
    }
    expect(state.children["ses_child_1"]).toMatchObject({ status: "running" });
  });
});