import { describe, expect, test } from "bun:test"
import { $ } from "bun"
import * as os from "node:os"
import * as path from "node:path"
import { extractChangedFiles, collectChangedFiles } from "../src/adapters/vcs"
import { loadDecision, loadRun, saveDecision, saveRun } from "../src/adapters/store"
import { detail, summarize } from "../src/format"
import type { ReviewReport } from "../src/domain/types"

const report = (over: Partial<ReviewReport> = {}): ReviewReport => ({
	runId: "run_test",
	action: "request_changes",
	matrix: { "src/api/router.ts": { security: 0.96 } },
	findings: [
		{
			file: "src/api/router.ts",
			dimension: "security",
			probability: 0.96,
			startLine: 8,
			endLine: 8,
			mechanism: "missing_authz",
			// Real runs return fractional scores, not the 0-3 integers of the rubric.
			severity: 2.97,
			owner: "security",
		},
	],
	unplaced: 0,
	skipped: [],
	source: "jev",
	questionsVersion: "2026-09-17.1",
	usage: { inputTokens: 8118, outputTokens: 585, requests: 13 },
	latencyMs: 5088,
	...over,
})

describe("bundle shape", () => {
	test("exports one plugin definition v2 can load", async () => {
		const out = path.join(os.tmpdir(), `jev-gate-bundle-${Date.now()}.js`)
		await $`bun build ${import.meta.dir}/../src/index.ts --outfile ${out} --target node`.quiet()
		const mod = await import(out)
		// v2 reads only the default export and rejects a callable one; extra
		// named exports are ignored, so pin the default's shape, not the list.
		expect(Object.keys(mod)).toContain("default")
		expect(typeof mod.default).toBe("object")
		expect(mod.default.id).toBe("ywai-jev-gate")
		expect(typeof mod.default.setup).toBe("function")
	})

	test("registers every tool the plugin ships", async () => {
		const added: string[] = []
		const entry = await import("../src/index")
		await entry.default.setup({
			tool: {
				transform: async (cb: (editor: { add: (def: { name: string }) => void }) => void) => {
					cb({ add: (def) => added.push(def.name) })
				},
			},
		} as never)
		expect(added).toEqual([
			"jev_review_diff",
			"jev_review_path",
			"jev_find",
			"jev_report",
			"jev_route",
		])
	})
})

describe("vcs adapter", () => {
	test("reads the v2.0.6 envelope: { data: [{ file, patch }] }", () => {
		const files = extractChangedFiles({
			location: { directory: "/x" },
			data: [
				{ file: "src/a.ts", patch: "@@ -1 +1 @@\n-a\n+b" },
				{ file: "src/b.ts", patch: "@@ -1 +1 @@\n-c\n+d" },
			],
		})
		expect(files.map((f) => f.path)).toEqual(["src/a.ts", "src/b.ts"])
		expect(files[0].patch).toContain("+b")
	})

	test("a plain diff string still works", () => {
		const diff = "diff --git a/src/a.ts b/src/a.ts\n--- a/src/a.ts\n+++ b/src/a.ts\n@@ -1 +1 @@\n-a\n+b"
		expect(extractChangedFiles(diff).map((f) => f.path)).toEqual(["src/a.ts"])
	})

	test("an unknown envelope yields nothing rather than a wrong review", () => {
		expect(extractChangedFiles({ data: [{ nope: 1 }] })).toEqual([])
		expect(extractChangedFiles(null)).toEqual([])
	})

	test("only the accepted mode is used; a rejecting host falls back to git", async () => {
		const tried: string[] = []
		const vcs = {
			diff: async ({ mode }: { mode: string }) => {
				tried.push(mode)
				// v2.0.6 rejects everything but "working" with a SchemaError.
				if (mode !== "working") throw new Error("SchemaError(Expected Vcs.Mode)")
				return { data: [{ file: "src/a.ts", patch: "@@ -1 +1 @@\n+b" }] }
			},
		}
		const { files, source } = await collectChangedFiles(vcs, process.cwd())
		expect(tried).toContain("working")
		expect(source).toBe("vcs")
		expect(files.map((f) => f.path)).toEqual(["src/a.ts"])
	})
})

describe("store", () => {
	test("round-trips a run and a session decision through ctx.storage", async () => {
		const backing = new Map<string, unknown>()
		const storage = {
			get: async (key: unknown) => backing.get(String(key)),
			set: async (key: unknown, value: unknown) => void backing.set(String(key), value),
		}
		const saved = report()
		await saveRun(storage, saved)
		await saveDecision(storage, "ses_1", saved)

		expect((await loadRun(storage, "run_test"))?.action).toBe("request_changes")
		const decision = await loadDecision(storage, "ses_1")
		expect(decision?.blockingFindings).toBe(1)
		// Gates must be able to ignore a non-Jev decision, so source travels.
		expect(decision?.source).toBe("jev")
		expect([...backing.keys()]).toEqual(["jev/runs/run_test", "jev/sessions/ses_1"])
	})

	test("a storage that throws does not fail the run", async () => {
		const storage = {
			get: async () => {
				throw new Error("storage down")
			},
			set: async () => {
				throw new Error("storage down")
			},
		}
		await saveRun(storage, report())
		// Memory still answers, so the report is not lost mid-session.
		expect((await loadRun(storage, "run_test"))?.runId).toBe("run_test")
	})
})

describe("summarize", () => {
	test("carries file:line, mechanism, severity and the attribution rule", () => {
		const md = summarize(report())
		expect(md).toContain("REQUEST CHANGES")
		expect(md).toContain("src/api/router.ts:8")
		expect(md).toContain("missing_authz")
		expect(md).toContain("severity 3.0")
		expect(md).toContain("96%")
		expect(md).toContain("run_test")
		expect(md.toLowerCase()).toContain("do not add your own")
	})

	test("clean says no signal, not approved", () => {
		const md = summarize(report({ action: "clean", findings: [], matrix: { "a.ts": { security: 0.1 } } }))
		expect(md).toContain("CLEAN")
		expect(md).toContain("No signal reached the screen threshold")
		expect(md.toLowerCase()).not.toContain("approve")
	})

	test("a signal that could not be placed is said out loud, not dropped", () => {
		const md = summarize(
			report({ action: "clean", findings: [], unplaced: 1, matrix: { "a.ts": { security: 0.95 } } }),
		)
		expect(md).toContain("could not be placed")
		// The headline is what a human skims, so the word has to carry it too.
		expect(md).toContain("INCONCLUSIVE")
		expect(md).not.toContain("CLEAN")
	})

	test("compatibility noise on a clean run does not manufacture a caveat", () => {
		// Both dimensions score high on almost everything and are never
		// promoted, so a matrix full of them is a clean run, not a lost signal.
		const md = summarize(
			report({
				action: "clean",
				findings: [],
				unplaced: 0,
				matrix: { "a.ts": { compatibility: 0.86, testGap: 0.91, security: 0.12 } },
			}),
		)
		expect(md).toContain("CLEAN")
		expect(md).toContain("No signal reached the screen threshold")
		expect(md).not.toContain("could not be placed")
	})
})

describe("detail", () => {
	test("shows every dimension's score, not just the ones that opened a finding", () => {
		const md = detail(
			report({
				action: "clean",
				findings: [],
				unplaced: 1,
				matrix: { "a.ts": { security: 0.83, correctness: 0.12, compatibility: 0.86 } },
			}),
		)
		expect(md).toContain("83%")
		// The score that went nowhere is the whole point of this tool.
		expect(md).toContain("12%")
		expect(md).toContain("86%")
		expect(md).toContain("1 unplaced")
	})

	test("bolds only a score that could actually open a finding", () => {
		const md = detail(
			report({
				action: "clean",
				findings: [],
				matrix: { "a.ts": { security: 0.83, compatibility: 0.86 } },
			}),
		)
		// compatibility is over threshold too, and means nothing.
		expect(md).toContain("**83%**")
		expect(md).not.toContain("**86%**")
	})
})

describe("setup never fails silently", () => {
	test("a host without ctx.tool.transform records why, instead of vanishing", async () => {
		const entry = await import("../src/index")
		const errors: string[] = []
		const original = console.error
		console.error = (line: unknown) => void errors.push(String(line))
		try {
			await entry.default.setup({} as never)
		} finally {
			console.error = original
		}
		// The field failure was a plugin that logged "loading plugin" and then
		// registered nothing, so the tools read as Unknown tool with no clue.
		expect(errors.join("\n")).toContain("jev-gate")
		expect(errors.join("\n")).toContain("ctx.tool.transform")
	})

	test("a transform that throws does not reject setup", async () => {
		const entry = await import("../src/index")
		const original = console.error
		console.error = () => {}
		try {
			await entry.default.setup({
				tool: {
					transform: async () => {
						throw new Error("host API changed")
					},
				},
			} as never)
		} finally {
			console.error = original
		}
	})
})
