import { afterEach, beforeEach, describe, expect, test } from "bun:test"
import { Database } from "bun:sqlite"
import { mkdtempSync, rmSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"

import { linkChildSession, resolveOpencodeDbPath } from "../../shared/v2/link-parent"

// The host accepts parentID on session.create and drops it, leaving the row in
// session_v2 with parent_id NULL. Everything that identifies a subagent by its
// parent then loses the session, so these tests pin the repair: it writes the
// edge, it does not overwrite one that already exists, and every degraded path
// reports false instead of throwing — an unlinked delegation must still run.
describe("linkChildSession", () => {
	let dir: string
	let dbPath: string

	beforeEach(() => {
		dir = mkdtempSync(join(tmpdir(), "ywai-link-parent-"))
		dbPath = join(dir, "opencode.db")
		const db = new Database(dbPath, { create: true })
		db.run("CREATE TABLE session_v2 (id TEXT PRIMARY KEY, parent_id TEXT, title TEXT)")
		db.run("INSERT INTO session_v2 (id, parent_id, title) VALUES ('ses_child', NULL, 'dev · x')")
		db.close()
	})

	afterEach(() => rmSync(dir, { recursive: true, force: true }))

	function parentOf(id: string): string | null {
		const db = new Database(dbPath, { readonly: true })
		try {
			return (db.query("SELECT parent_id FROM session_v2 WHERE id = ?").get(id) as any)?.parent_id ?? null
		} finally {
			db.close()
		}
	}

	test("links an unlinked child to its parent", async () => {
		expect(await linkChildSession("ses_child", "ses_parent", dbPath)).toBe(true)
		expect(parentOf("ses_child")).toBe("ses_parent")
	})

	test("leaves an existing parent alone", async () => {
		await linkChildSession("ses_child", "ses_parent", dbPath)
		expect(await linkChildSession("ses_child", "ses_other", dbPath)).toBe(false)
		expect(parentOf("ses_child")).toBe("ses_parent")
	})

	test("reports false for an unknown session instead of throwing", async () => {
		expect(await linkChildSession("ses_missing", "ses_parent", dbPath)).toBe(false)
	})

	test("reports false when the store is missing instead of throwing", async () => {
		expect(await linkChildSession("ses_child", "ses_parent", join(dir, "nope.db"))).toBe(false)
	})

	test("ignores empty ids", async () => {
		expect(await linkChildSession("", "ses_parent", dbPath)).toBe(false)
		expect(await linkChildSession("ses_child", "", dbPath)).toBe(false)
	})

	test("resolves the store under XDG_DATA_HOME", () => {
		const prev = process.env.XDG_DATA_HOME
		process.env.XDG_DATA_HOME = "/tmp/xdg"
		try {
			expect(resolveOpencodeDbPath()).toBe("/tmp/xdg/opencode/opencode.db")
		} finally {
			if (prev === undefined) delete process.env.XDG_DATA_HOME
			else process.env.XDG_DATA_HOME = prev
		}
	})
})
