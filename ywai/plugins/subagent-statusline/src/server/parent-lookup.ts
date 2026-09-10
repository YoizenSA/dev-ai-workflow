/**
 * Parent lookup against OpenCode's session store.
 *
 * `session.created` on v2 carries no parentID: the host does not set a parent
 * at creation, so the event alone can never identify a subagent. The delegating
 * plugin writes the edge into `session_v2` right after create returns, which is
 * after this event has already been published — so a session that looks like a
 * root here may be a child a moment later. Reading the store is how that later
 * truth is recovered.
 *
 * Kept synchronous (bun:sqlite is a sync API) so it drops straight into the
 * adapter's event mapping, and best-effort: a missing store or an unknown
 * schema reads as "no parent", never as a throw inside the event loop.
 */

import * as os from "node:os";
import * as path from "node:path";

export function resolveOpencodeDbPath(): string {
  const dataHome =
    process.env.XDG_DATA_HOME || path.join(os.homedir(), ".local", "share");
  return path.join(dataHome, "opencode", "opencode.db");
}

type SqliteDatabase = {
  query(sql: string): { get(...params: unknown[]): unknown };
  close(): void;
};

/** Loaded lazily and cached; undefined once we know bun:sqlite is unavailable. */
let openDatabase: ((file: string) => SqliteDatabase) | null | undefined;

function sqliteOpener(): ((file: string) => SqliteDatabase) | null {
  if (openDatabase !== undefined) return openDatabase;
  try {
    // Synchronous require: the adapter maps events synchronously, and this
    // module only ever runs inside the Bun runtime that ships OpenCode.
    const { Database } = require("bun:sqlite");
    openDatabase = (file: string) =>
      new Database(file, { readonly: true }) as SqliteDatabase;
  } catch {
    openDatabase = null;
  }
  return openDatabase;
}

/**
 * Return the parent session id recorded for sessionID, or undefined when the
 * session is unknown, unlinked, or the store cannot be read.
 */
export function lookupParentSessionID(
  sessionID: string,
  dbPath: string = resolveOpencodeDbPath(),
): string | undefined {
  if (!sessionID) return undefined;
  const open = sqliteOpener();
  if (!open) return undefined;

  try {
    const db = open(dbPath);
    try {
      const row = db
        .query("SELECT parent_id FROM session_v2 WHERE id = ?")
        .get(sessionID) as { parent_id?: unknown } | undefined;
      const parent = row?.parent_id;
      return typeof parent === "string" && parent.length > 0
        ? parent
        : undefined;
    } finally {
      db.close();
    }
  } catch {
    return undefined;
  }
}

/** Test seam: forget the cached opener so a test can re-probe. */
export function resetSqliteOpenerForTests(): void {
  openDatabase = undefined;
}
