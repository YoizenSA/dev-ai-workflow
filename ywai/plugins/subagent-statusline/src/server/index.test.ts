import { describe, expect, it } from "vitest";
import plugin from "./index.js";
import {
  createRuntimeHarness,
  readRuntimeState,
  readStatusText,
} from "../../test/helpers/runtime-harness.js";

type FakeSubscribeOptions = { signal?: AbortSignal };

/**
 * v2-shaped event source: yields the given events, then stays open until the
 * plugin aborts (the real server keeps the SSE-style stream open).
 */
function eventStream(
  events: unknown[],
  signal?: AbortSignal,
): AsyncIterable<unknown> {
  return {
    async *[Symbol.asyncIterator]() {
      for (const event of events) yield event;
      if (!signal) return;
      await new Promise<never>((_, reject) => {
        if (signal.aborted) {
          reject(new Error("aborted"));
          return;
        }
        signal.addEventListener(
          "abort",
          () => reject(new Error("aborted")),
          { once: true },
        );
      });
    },
  };
}

function fakeContext(events: unknown[]) {
  return {
    event: {
      subscribe(options?: FakeSubscribeOptions) {
        return eventStream(events, options?.signal);
      },
    },
  };
}

async function waitFor(
  predicate: () => boolean | Promise<boolean>,
  timeoutMs = 5_000,
): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (await predicate()) return;
    await new Promise((resolve) => setTimeout(resolve, 25));
  }
  if (!(await predicate())) {
    throw new Error("condition not met before timeout");
  }
}

const CREATED_MS = 1765030400000;

function subagentCreatedEvent(): unknown {
  return {
    id: "evt_1",
    created: CREATED_MS,
    type: "session.created",
    data: {
      sessionID: "ses_child_1",
      projectID: "prj_1",
      slug: "review",
      title: "Review auth changes",
      agent: "reviewer",
      parentID: "ses_parent_1",
      version: "1",
    },
  };
}

describe("v2 server plugin setup", () => {
  it("exposes the v2 plugin contract shape", () => {
    expect(plugin.id).toBe("subagent-statusline");
    expect(typeof plugin.setup).toBe("function");
  });

  it("writes an empty state and status line on startup", async () => {
    const harness = await createRuntimeHarness();
    const cleanup = await plugin.setup(fakeContext([]));
    try {
      const state = await readRuntimeState(harness.statePath);
      expect(state).toMatchObject({ children: {}, totalExecuted: 0 });
      const text = await readStatusText(harness.textPath);
      expect(typeof text).toBe("string");
      expect(text.length).toBeGreaterThan(0);
    } finally {
      const dispose = cleanup as () => void;
      dispose();
    }
  });

  it("preserves existing state when OPENCODE_SUBAGENT_STATUSLINE_PRESERVE_STATE=1", async () => {
    const harness = await createRuntimeHarness({ preserveState: true });
    const { writeFile } = await import("node:fs/promises");
    const marker = {
      children: {
        ses_previous: {
          id: "ses_previous",
          title: "Old run",
          status: "done",
          source: "session",
          parentID: "ses_parent_1",
        },
      },
      totalExecuted: 1,
      updatedAt: "2026-04-30T10:00:00.000Z",
    };
    await writeFile(harness.statePath, JSON.stringify(marker), "utf8");

    const cleanup = await plugin.setup(fakeContext([]));
    try {
      const state = await readRuntimeState(harness.statePath);
      expect(state).toMatchObject({
        children: { ses_previous: { title: "Old run", status: "done" } },
        totalExecuted: 1,
      });
    } finally {
      const dispose = cleanup as () => void;
      dispose();
    }
  });

  it("translates v2 stream events into state.json and status.txt updates", async () => {
    const harness = await createRuntimeHarness();
    const cleanup = await plugin.setup(
      fakeContext([
        null,
        { type: 42 },
        { type: "session.message.content.updated", data: "not-an-object" },
        subagentCreatedEvent(),
      ]),
    );
    try {
      await waitFor(async () => {
        const state = await readRuntimeState<{
          children?: Record<string, { status?: string }>;
        }>(harness.statePath);
        return state.children?.["ses_child_1"]?.status === "running";
      });
      const text = await readStatusText(harness.textPath);
      expect(text.length).toBeGreaterThan(0);
    } finally {
      const dispose = cleanup as () => void;
      dispose();
    }
  });

  it("never rejects setup when the event stream fails immediately", async () => {
    await createRuntimeHarness();
    const brokenContext = {
      event: {
        subscribe() {
          throw new Error("no stream for you");
        },
      },
    };
    await expect(plugin.setup(brokenContext)).resolves.toBeDefined();
  });

  it("returns an abort-based cleanup that is safe to call", async () => {
    const harness = await createRuntimeHarness();
    const cleanup = await plugin.setup(fakeContext([]));
    expect(typeof cleanup).toBe("function");
    const dispose = cleanup as () => void;
    expect(() => dispose()).not.toThrow();
    // The startup snapshot stays intact after cleanup.
    await expect(readStatusText(harness.textPath)).resolves.toBeDefined();
  });
});
