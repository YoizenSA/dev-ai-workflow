import { describe, expect, test } from "bun:test"
import * as fs from "node:fs"
import * as os from "node:os"
import * as path from "node:path"

import { needsInitialBuild, wiringPath } from "../src/decide.js"

function tmpRepo(): string {
	const dir = fs.mkdtempSync(path.join(os.tmpdir(), "graft-build-test-"))
	fs.mkdirSync(path.join(dir, ".git"))
	return dir
}

describe("needsInitialBuild", () => {
	test("builds when the repo has no graph at all", () => {
		expect(needsInitialBuild(tmpRepo())).toBe(true)
	})

	test("builds when graft/ exists but is empty — `graft ask` creates the dir without a graph", () => {
		const dir = tmpRepo()
		fs.mkdirSync(path.join(dir, "graft"))
		expect(needsInitialBuild(dir)).toBe(true)
	})

	test("skips when the wiring graph is already there, however stale", () => {
		const dir = tmpRepo()
		fs.mkdirSync(path.dirname(wiringPath(dir)), { recursive: true })
		fs.writeFileSync(wiringPath(dir), "{}")
		expect(needsInitialBuild(dir)).toBe(false)
	})

	test("skips a non-repo: graft indexes repositories, and cwd may be anywhere", () => {
		const dir = fs.mkdtempSync(path.join(os.tmpdir(), "graft-build-test-"))
		expect(needsInitialBuild(dir)).toBe(false)
	})

	test("skips when a previous build failed here, so a broken repo retries once, not every open", () => {
		const dir = tmpRepo()
		fs.mkdirSync(path.join(dir, "graft", ".cache"), { recursive: true })
		fs.writeFileSync(path.join(dir, "graft", ".cache", ".ywai-build-failed"), "")
		expect(needsInitialBuild(dir)).toBe(false)
	})

	test("honors graft's own kill switch", () => {
		const dir = tmpRepo()
		process.env.GRAFT_NO_REFRESH = "1"
		try {
			expect(needsInitialBuild(dir)).toBe(false)
		} finally {
			delete process.env.GRAFT_NO_REFRESH
		}
	})
})
