/**
 * Parent linking for sessions created through the v2 plugin context.
 *
 * `ctx.session.create({ title, parentID })` accepts a parentID and drops it:
 * the returned session carries no parentID and the row lands in `session_v2`
 * with `parent_id` NULL. Everything downstream that identifies a subagent by
 * its parent then stops seeing it — the sub-agent statusline drops such
 * sessions outright — so a delegation runs invisibly.
 *
 * Until the host honors the field, write the edge ourselves. This is a
 * deliberate reach into the host's store, so it is narrow on purpose: one
 * UPDATE, by primary key, only when the row is still unlinked, and every
 * failure degrades to "not linked" instead of breaking the delegation.
 */

import * as os from "node:os"
import * as path from "node:path"

/** Resolve OpenCode's SQLite store the same way the host does (XDG). */
export function resolveOpencodeDbPath(): string {
	const dataHome = process.env.XDG_DATA_HOME || path.join(os.homedir(), ".local", "share")
	return path.join(dataHome, "opencode", "opencode.db")
}

/**
 * Link childID to parentID in the host's session store. Returns whether the
 * edge was written: false covers every degraded path (no bun:sqlite, missing
 * db, unknown schema, row already linked or absent).
 *
 * `session_v2` is v2's table; the legacy `session` table is left alone.
 */
export async function linkChildSession(
	childID: string,
	parentID: string,
	dbPath: string = resolveOpencodeDbPath(),
): Promise<boolean> {
	if (!childID || !parentID) return false

	try {
		// bun:sqlite only exists under the Bun runtime that ships OpenCode.
		// A dynamic import keeps this module loadable everywhere else.
		const { Database } = await import("bun:sqlite")
		const db = new Database(dbPath, { readwrite: true })
		try {
			const changed = db
				.query("UPDATE session_v2 SET parent_id = ? WHERE id = ? AND parent_id IS NULL")
				.run(parentID, childID)
			return (changed?.changes ?? 0) > 0
		} finally {
			db.close()
		}
	} catch {
		return false
	}
}
