import { describe, expect, it } from "vitest";
import { createV2EventAdapter } from "./v2-event-adapter.js";
import { applySubagentEvent } from "../events.js";
import { createEmptyState } from "../state.js";

// Payloads mirror @opencode/protocol@0.0.0-beta (dist/groups/event.d.ts):
// envelope is { id, created, type, data } with the payload in `data`.

const CREATED_MS = 1765030400000;

function v2Event(type: string, data: unknown): unknown {
  return { id: "evt_1", created: CREATED_MS, type, data };
}

describe("v2 adapter: session lifecycle", () => {
  it("maps session.created with parentID to the internal session.created shape", () => {
    const adapter = createV2EventAdapter();
    const [event] = adapter.adapt(
      v2Event("session.created", {
        sessionID: "ses_child_1",
        projectID: "prj_1",
        slug: "review-auth-changes",
        title: "Review auth changes",
        agent: "reviewer",
        parentID: "ses_parent_1",
        version: "1",
        location: { directory: "/repo" },
      }),
    );
    expect(event).toBeDefined();
    expect(event?.type).toBe("session.created");
    expect(event?.properties).toMatchObject({
      id: "ses_child_1",
      sessionID: "ses_child_1",
      parentID: "ses_parent_1",
      info: {
        id: "ses_child_1",
        sessionID: "ses_child_1",
        parentID: "ses_parent_1",
        title: "Review auth changes",
        agent: "reviewer",
        time: { created: CREATED_MS },
      },
    });
  });

  it("ignores session.created without parentID (root sessions were never tracked)", () => {
    const adapter = createV2EventAdapter();
    expect(
      adapter.adapt(
        v2Event("session.created", {
          sessionID: "ses_root",
          projectID: "prj_1",
          slug: "root",
          version: "1",
        }),
      ),
    ).toEqual([]);
  });

  it("maps session.status by passing the status object through", () => {
    const adapter = createV2EventAdapter();
    const [event] = adapter.adapt(
      v2Event("session.status", {
        sessionID: "ses_child_1",
        status: { type: "idle" },
      }),
    );
    expect(event?.type).toBe("session.status");
    expect(event?.properties).toMatchObject({
      id: "ses_child_1",
      sessionID: "ses_child_1",
      status: { type: "idle" },
    });
  });

  it("maps session.idle to the internal session.idle shape", () => {
    const adapter = createV2EventAdapter();
    const [event] = adapter.adapt(
      v2Event("session.idle", { sessionID: "ses_child_1" }),
    );
    expect(event).toEqual({
      type: "session.idle",
      properties: {
        id: "ses_child_1",
        sessionID: "ses_child_1",
        info: { id: "ses_child_1", sessionID: "ses_child_1" },
      },
    });
  });

  it("maps session.execution.failed to the internal session.error shape", () => {
    const adapter = createV2EventAdapter();
    const [event] = adapter.adapt(
      v2Event("session.execution.failed", {
        sessionID: "ses_child_1",
        error: { type: "api_error", message: "boom", status: 500 },
      }),
    );
    expect(event?.type).toBe("session.error");
    expect(event?.properties).toMatchObject({
      id: "ses_child_1",
      sessionID: "ses_child_1",
      error: { type: "api_error", message: "boom", status: 500 },
    });
  });
});

describe("v2 adapter: session detail updates", () => {
  it("reshapes session.usage.updated tokens into walkable *Tokens hints", () => {
    const adapter = createV2EventAdapter();
    const [event] = adapter.adapt(
      v2Event("session.usage.updated", {
        sessionID: "ses_child_1",
        cost: 0.01,
        tokens: {
          input: 1200,
          output: 300,
          reasoning: 50,
          cache: { read: 100, write: 20 },
        },
      }),
    );
    expect(event?.type).toBe("message.updated");
    expect(event?.properties).toMatchObject({
      sessionID: "ses_child_1",
      usage: {
        inputTokens: 1200,
        outputTokens: 300,
        reasoningTokens: 50,
        totalTokens: 1550,
      },
    });
  });

  it("maps session.renamed onto the detail path carrying the new title", () => {
    const adapter = createV2EventAdapter();
    const [event] = adapter.adapt(
      v2Event("session.renamed", {
        sessionID: "ses_child_1",
        title: "New focus: audit login flow",
      }),
    );
    expect(event?.type).toBe("message.updated");
    expect(event?.properties).toMatchObject({
      sessionID: "ses_child_1",
      info: {
        id: "ses_child_1",
        sessionID: "ses_child_1",
        title: "New focus: audit login flow",
      },
    });
  });

  it("maps session.step.started model selection onto the assistant-message shape", () => {
    const adapter = createV2EventAdapter();
    const [event] = adapter.adapt(
      v2Event("session.step.started", {
        sessionID: "ses_child_1",
        model: { id: "claude-sonnet-4", providerID: "anthropic" },
        agent: "build",
        assistantMessageID: "msg_7",
      }),
    );
    expect(event?.type).toBe("message.updated");
    expect(event?.properties).toMatchObject({
      sessionID: "ses_child_1",
      info: {
        id: "msg_7",
        sessionID: "ses_child_1",
        role: "assistant",
        modelID: "claude-sonnet-4",
        providerID: "anthropic",
        time: { created: CREATED_MS },
      },
    });
  });
});

describe("v2 adapter: tool events", () => {
  it("maps session.message.content.updated tool parts to v1-shaped message.part.updated", () => {
    const adapter = createV2EventAdapter();
    const events = adapter.adapt(
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
              input: {
                prompt: "Investigate why tests are flaky.",
                description: "Investigate flaky tests",
                subagent_type: "tester",
              },
              metadata: { sessionId: "ses_child_1" },
            },
          },
          { type: "text", text: "meanwhile, thinking…" },
        ],
      }),
    );
    expect(events).toHaveLength(1);
    expect(events[0]?.type).toBe("message.part.updated");
    expect(events[0]?.properties).toMatchObject({
      sessionID: "ses_parent_1",
      info: { sessionID: "ses_parent_1", id: "msg_1" },
      part: {
        type: "tool",
        tool: "task",
        id: "too_1",
        sessionID: "ses_parent_1",
        messageID: "msg_1",
        state: {
          status: "running",
          input: {
            prompt: "Investigate why tests are flaky.",
            description: "Investigate flaky tests",
            subagent_type: "tester",
          },
          metadata: { sessionId: "ses_child_1" },
          time: { created: CREATED_MS },
        },
      },
    });
  });

  it("maps completed tool states and their content text into output", () => {
    const adapter = createV2EventAdapter();
    const events = adapter.adapt(
      v2Event("session.message.content.updated", {
        sessionID: "ses_parent_1",
        messageID: "msg_1",
        content: [
          {
            id: "too_1",
            type: "tool",
            name: "task",
            time: { created: CREATED_MS, completed: CREATED_MS + 5_000 },
            state: {
              status: "completed",
              input: { prompt: "Investigate why tests are flaky." },
              metadata: {},
              content: [{ type: "text", text: "task_id: ses_child_1 done" }],
            },
          },
        ],
      }),
    );
    expect(events[0]?.properties).toMatchObject({
      part: {
        tool: "task",
        state: {
          status: "completed",
          output: "task_id: ses_child_1 done",
        },
      },
    });
  });

  it("maps session.tool.input.started to an early running tool part", () => {
    const adapter = createV2EventAdapter();
    const [event] = adapter.adapt(
      v2Event("session.tool.input.started", {
        sessionID: "ses_parent_1",
        id: "too_1",
        name: "delegate",
        assistantMessageID: "msg_1",
      }),
    );
    expect(event?.type).toBe("message.part.updated");
    expect(event?.properties).toMatchObject({
      part: {
        type: "tool",
        tool: "delegate",
        id: "too_1",
        sessionID: "ses_parent_1",
        messageID: "msg_1",
        state: { status: "running" },
      },
    });
  });

  it("maps session.tool.called with the remembered name and parsed input", () => {
    const adapter = createV2EventAdapter();
    adapter.adapt(
      v2Event("session.tool.input.started", {
        sessionID: "ses_parent_1",
        id: "too_1",
        name: "task",
        assistantMessageID: "msg_1",
      }),
    );
    const [event] = adapter.adapt(
      v2Event("session.tool.called", {
        id: "too_1",
        sessionID: "ses_parent_1",
        input: { prompt: "Investigate why tests are flaky." },
        executed: false,
        assistantMessageID: "msg_1",
      }),
    );
    expect(event?.properties).toMatchObject({
      part: {
        tool: "task",
        id: "too_1",
        state: {
          status: "running",
          input: { prompt: "Investigate why tests are flaky." },
        },
      },
    });
  });

  it("maps session.tool.success to a completed part with output text", () => {
    const adapter = createV2EventAdapter();
    adapter.adapt(
      v2Event("session.tool.input.started", {
        sessionID: "ses_parent_1",
        id: "too_1",
        name: "task",
        assistantMessageID: "msg_1",
      }),
    );
    const [event] = adapter.adapt(
      v2Event("session.tool.success", {
        id: "too_1",
        sessionID: "ses_parent_1",
        content: [{ type: "text", text: "task_id: ses_child_1 done" }],
        executed: true,
        assistantMessageID: "msg_1",
      }),
    );
    expect(event?.properties).toMatchObject({
      part: {
        tool: "task",
        state: { status: "completed", output: "task_id: ses_child_1 done" },
      },
    });
  });

  it("maps session.tool.failed to an error part with the error payload", () => {
    const adapter = createV2EventAdapter();
    adapter.adapt(
      v2Event("session.tool.input.started", {
        sessionID: "ses_parent_1",
        id: "too_1",
        name: "delegate",
        assistantMessageID: "msg_1",
      }),
    );
    const [event] = adapter.adapt(
      v2Event("session.tool.failed", {
        id: "too_1",
        sessionID: "ses_parent_1",
        error: { type: "timeout", message: "agent timed out" },
        executed: true,
        assistantMessageID: "msg_1",
      }),
    );
    expect(event?.properties).toMatchObject({
      part: {
        tool: "delegate",
        state: {
          status: "error",
          error: { type: "timeout", message: "agent timed out" },
        },
      },
    });
  });

  it("ignores terminal tool events whose call was never seen", () => {
    const adapter = createV2EventAdapter();
    expect(
      adapter.adapt(
        v2Event("session.tool.success", {
          id: "too_unknown",
          sessionID: "ses_parent_1",
          content: [],
          executed: true,
        }),
      ),
    ).toEqual([]);
  });
});

describe("v2 adapter: defensive behavior", () => {
  it("ignores unknown v2 event types", () => {
    const adapter = createV2EventAdapter();
    expect(adapter.adapt(v2Event("server.connected", {}))).toEqual([]);
    expect(
      adapter.adapt(v2Event("tui.prompt.append", { text: "hi" })),
    ).toEqual([]);
    expect(
      adapter.adapt(v2Event("session.compaction.started", { sessionID: "ses_x" })),
    ).toEqual([]);
    expect(
      adapter.adapt(v2Event("some.future.event", { weird: true })),
    ).toEqual([]);
  });

  it("never throws on malformed envelopes", () => {
    const adapter = createV2EventAdapter();
    const malformed: unknown[] = [
      null,
      undefined,
      42,
      "string",
      {},
      { type: 42 },
      { type: "session.created" },
      { type: "session.created", data: null },
      { type: "session.created", data: "not-an-object" },
      { type: "session.idle", data: {} },
      { type: "session.status", data: { sessionID: "ses_1", status: null } },
      { type: "session.message.content.updated", data: { content: "nope" } },
      {
        type: "session.message.content.updated",
        data: { sessionID: "ses_1", content: [{ type: "tool" }] },
      },
      { type: "session.tool.input.started", data: { id: "too_1" } },
      { type: "session.usage.updated", data: { tokens: "x" } },
      { type: "session.step.started", data: { model: 7 } },
    ];
    for (const raw of malformed) {
      expect(() => adapter.adapt(raw)).not.toThrow();
      expect(adapter.adapt(raw)).toEqual([]);
    }
  });

  it("tolerates a v1-style properties envelope for mapped types", () => {
    const adapter = createV2EventAdapter();
    const [event] = adapter.adapt({
      type: "session.idle",
      properties: { sessionID: "ses_child_1" },
    });
    expect(event?.type).toBe("session.idle");
  });
});

describe("v2 adapter drives the vendored state machine", () => {
  it("tracks a subagent session from creation to idle", () => {
    const adapter = createV2EventAdapter();
    const state = createEmptyState();

    const created = adapter.adapt(
      v2Event("session.created", {
        sessionID: "ses_child_1",
        projectID: "prj_1",
        slug: "review",
        title: "Review auth changes",
        agent: "reviewer",
        parentID: "ses_parent_1",
        version: "1",
      }),
    );
    for (const event of created) applySubagentEvent(state, event);
    expect(state.children["ses_child_1"]).toMatchObject({
      id: "ses_child_1",
      title: "Review auth changes",
      parentID: "ses_parent_1",
      status: "running",
      source: "session",
    });

    const idle = adapter.adapt(
      v2Event("session.idle", { sessionID: "ses_child_1" }),
    );
    for (const event of idle) applySubagentEvent(state, event);
    expect(state.children["ses_child_1"]).toMatchObject({ status: "done" });
  });

  it("applies usage tokens and model to the tracked child", () => {
    const adapter = createV2EventAdapter();
    const state = createEmptyState();
    for (const event of adapter.adapt(
      v2Event("session.created", {
        sessionID: "ses_child_1",
        projectID: "prj_1",
        slug: "review",
        title: "Review auth changes",
        agent: "reviewer",
        parentID: "ses_parent_1",
        version: "1",
      }),
    ))
      applySubagentEvent(state, event);

    for (const event of adapter.adapt(
      v2Event("session.usage.updated", {
        sessionID: "ses_child_1",
        cost: 0.01,
        tokens: { input: 1200, output: 300, reasoning: 50 },
      }),
    ))
      applySubagentEvent(state, event);
    expect(state.children["ses_child_1"]?.tokens).toMatchObject({
      input: 1200,
      output: 300,
      total: 1550,
    });

    for (const event of adapter.adapt(
      v2Event("session.step.started", {
        sessionID: "ses_child_1",
        model: { id: "claude-sonnet-4", providerID: "anthropic" },
        agent: "build",
        assistantMessageID: "msg_7",
      }),
    ))
      applySubagentEvent(state, event);
    expect(state.children["ses_child_1"]?.model).toMatchObject({
      modelID: "claude-sonnet-4",
      providerID: "anthropic",
    });
  });

  it("runs a task tool child from content snapshot to success", () => {
    const adapter = createV2EventAdapter();
    const state = createEmptyState();

    for (const event of adapter.adapt(
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
              input: {
                prompt: "Investigate why tests are flaky.",
                description: "Investigate flaky tests",
                subagent_type: "tester",
              },
            },
          },
        ],
      }),
    ))
      applySubagentEvent(state, event);
    expect(state.children["tool:too_1"]).toMatchObject({
      id: "tool:too_1",
      toolName: "task",
      title: "Investigate flaky tests",
      status: "running",
      source: "tool",
      parentID: "ses_parent_1",
    });

    for (const event of adapter.adapt(
      v2Event("session.tool.success", {
        id: "too_1",
        sessionID: "ses_parent_1",
        content: [{ type: "text", text: "task_id: ses_child_1 done" }],
        executed: true,
        assistantMessageID: "msg_1",
      }),
    ))
      applySubagentEvent(state, event);
    expect(state.children["tool:too_1"]).toMatchObject({
      status: "done",
      targetSessionID: "ses_child_1",
    });
  });

  it("marks a tracked child as error on session.execution.failed", () => {
    const adapter = createV2EventAdapter();
    const state = createEmptyState();
    for (const event of adapter.adapt(
      v2Event("session.created", {
        sessionID: "ses_child_1",
        projectID: "prj_1",
        slug: "review",
        title: "Review auth changes",
        agent: "reviewer",
        parentID: "ses_parent_1",
        version: "1",
      }),
    ))
      applySubagentEvent(state, event);

    for (const event of adapter.adapt(
      v2Event("session.execution.failed", {
        sessionID: "ses_child_1",
        error: { type: "api_error", message: "boom" },
      }),
    ))
      applySubagentEvent(state, event);
    expect(state.children["ses_child_1"]).toMatchObject({ status: "error" });
  });
});
