import { describe, expect, test } from "bun:test"
import { mkdtempSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { runUltrafast, summarizeUltrafast } from "../src/browser/ultrafast"

/** A fake CLI: echoes the request it got, then prints the given lines. */
function fakeCli(body: string): string[] {
	const file = join(mkdtempSync(join(tmpdir(), "jev-fake-")), "cli.mjs")
	writeFileSync(file, body)
	return [process.execPath, file]
}

const request = { url: "http://x.test", goal: "Log in", values: { Password: "hunter2-secret" } }

describe("runUltrafast", () => {
	test("sends the request on stdin and returns the final result with streamed states", async () => {
		const command = fakeCli(`
			let raw = ""
			process.stdin.on("data", (c) => (raw += c))
			process.stdin.on("end", () => {
				const req = JSON.parse(raw)
				console.log(JSON.stringify({ type: "state", status: "ready", step: 1, action: "Password", kind: "fill" }))
				console.log("not json")
				console.log(JSON.stringify({ type: "result", status: "done", elapsed_ms: 5, final_url: req.url + "/home",
					history: [{ step: 1, action: "Password", kind: "fill", text: "***" }], echo: req.values.Password }))
			})
		`)
		const states: unknown[] = []
		const result = await runUltrafast(request, { command, onState: (s) => states.push(s) })
		expect(result.status).toBe("done")
		expect(result.final_url).toBe("http://x.test/home")
		expect(states).toHaveLength(1)
		expect((result as unknown as { echo: string }).echo).toBe("hunter2-secret")
	})

	test("a hung CLI is killed at the timeout and reported as error", async () => {
		const command = fakeCli(`setInterval(() => {}, 1000)`)
		const result = await runUltrafast(request, { command, timeoutMs: 300 })
		expect(result.status).toBe("error")
		expect(result.error).toContain("timed out")
	})

	test("a CLI that exits without a result line is an error with its stderr", async () => {
		const command = fakeCli(`console.error("boom"); process.exit(3)`)
		const result = await runUltrafast(request, { command })
		expect(result.status).toBe("error")
		expect(result.error).toContain("boom")
	})
})

describe("summarizeUltrafast", () => {
	test("tells the caller DONE is not the Then and never prints values", () => {
		const md = summarizeUltrafast(
			{
				type: "result",
				status: "done",
				elapsed_ms: 1200,
				final_url: "http://x.test/home",
				history: [{ step: 1, action: "Password", kind: "fill", text: "***" }],
			},
			"Log in",
		)
		expect(md).toContain("**Result:** done")
		expect(md).toContain("typed into **Password**")
		expect(md).toContain("jev_check_page")
		expect(md).not.toContain("hunter2")
	})

	test("prints the final page so the Then can be scored on it", () => {
		const md = summarizeUltrafast(
			{
				type: "result",
				status: "done",
				final_url: "http://x.test/login",
				history: [],
				final_page: { url: "http://x.test/login", title: "Login", snapshot: "- alert: Invalid credentials" },
			},
			"Log in",
		)
		expect(md).toContain("```\n- alert: Invalid credentials\n```")
		expect(md).toContain("snapshot")
	})
})
