import { beforeEach, describe, expect, test } from "bun:test";
import { Database } from "bun:sqlite";
import { mkdirSync, mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { createV2EventAdapter } from "./v2-event-adapter";

// v2 publishes session.created before any parent is set, so a subagent looks
// exactly like a root session at creation and the adapter used to drop it for
// good — the delegation then ran invisibly, which is the whole bug. The
// adapter must instead hold the session and emit its creation once the parent
// edge shows up in the store.
describe("deferred parent resolution", () => {
  let dbPath: string;

  function seedSession(id: string, parentID: string | null): void {
    const db = new Database(dbPath, { create: true });
    db.run("CREATE TABLE IF NOT EXISTS session_v2 (id TEXT PRIMARY KEY, parent_id TEXT)");
    db.run("INSERT OR REPLACE INTO session_v2 (id, parent_id) VALUES (?, ?)", [id, parentID]);
    db.close();
  }

  beforeEach(() => {
    // The lookup resolves the store under XDG_DATA_HOME, so redirecting the
    // variable points it at a throwaway db without stubbing the module.
    const dataHome = mkdtempSync(join(tmpdir(), "ywai-deferred-"));
    process.env.XDG_DATA_HOME = dataHome;
    mkdirSync(join(dataHome, "opencode"), { recursive: true });
    dbPath = join(dataHome, "opencode", "opencode.db");
  });

  function created(sessionID: string) {
    return { type: "session.created", properties: { sessionID, title: "ask · x" } };
  }
  function succeeded(sessionID: string) {
    return { type: "session.execution.succeeded", properties: { sessionID } };
  }

  test("holds an unparented creation and emits it once the edge exists", () => {
    const adapter = createV2EventAdapter();

    // Creation arrives before anything links the session: nothing to report.
    expect(adapter.adapt(created("ses_child"))).toEqual([]);

    // The delegating plugin writes the edge moments later.
    seedSession("ses_child", "ses_parent");

    const events = adapter.adapt(succeeded("ses_child"));
    const kinds = events.map((e: any) => e.type);
    expect(kinds).toContain("session.created");
    expect(kinds.indexOf("session.created")).toBe(0);

    const creation = events.find((e: any) => e.type === "session.created") as any;
    expect(creation.properties.parentID).toBe("ses_parent");
  });

  test("keeps ignoring a session that never gains a parent", () => {
    const adapter = createV2EventAdapter();
    adapter.adapt(created("ses_root"));
    seedSession("ses_root", null);

    const kinds = adapter.adapt(succeeded("ses_root")).map((e: any) => e.type);
    expect(kinds).not.toContain("session.created");
  });

  test("emits the deferred creation only once", () => {
    const adapter = createV2EventAdapter();
    adapter.adapt(created("ses_child"));
    seedSession("ses_child", "ses_parent");

    const first = adapter.adapt(succeeded("ses_child")).map((e: any) => e.type);
    const second = adapter.adapt(succeeded("ses_child")).map((e: any) => e.type);
    expect(first).toContain("session.created");
    expect(second).not.toContain("session.created");
  });

  test("passes a creation that already carries its parent straight through", () => {
    const adapter = createV2EventAdapter();
    const events = adapter.adapt({
      type: "session.created",
      properties: { sessionID: "ses_child", parentID: "ses_parent" },
    });
    expect(events.map((e: any) => e.type)).toEqual(["session.created"]);
  });
});
