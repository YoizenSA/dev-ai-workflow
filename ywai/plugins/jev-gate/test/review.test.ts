import { describe, expect, test } from "bun:test"
import { readFileSync } from "node:fs"
import { join } from "node:path"
import { isDenied, isReviewable, isTestFile, matchesGlob } from "../src/domain/config"
import { parseHunks } from "../src/domain/patch"
import { splitDiff } from "../src/adapters/git"
import { rankSignals } from "../src/review/judgments"
import { decideAction, runChangeReview, selectFiles } from "../src/review/workflow"
import type { JevClient, SystemOneRequest, SystemOneResponse } from "../src/jev/client"
import { HttpJevClient, MissingKeyError } from "../src/jev/client"

const FIXTURES = join(import.meta.dir, "../../../experiments/jev-lab/fixtures")

/** A client that answers from a script, and records what it was asked. */
class FakeClient implements JevClient {
	readonly calls: SystemOneRequest[] = []
	constructor(private readonly reply: (req: SystemOneRequest) => SystemOneResponse) {}
	async systemOne(request: SystemOneRequest): Promise<SystemOneResponse> {
		this.calls.push(request)
		return this.reply(request)
	}
}

const noulAnswers = (scores: Record<string, number>): SystemOneResponse => ({
	model: "fake",
	answers: Object.fromEntries(
		Object.entries(scores).map(([k, v]) => [k, { type: "noul" as const, noul: v }]),
	),
	usage: { input_tokens: 100, output_tokens: 10 },
})

describe("deny globs", () => {
	test("** spans separators, * does not", () => {
		expect(matchesGlob("src/a/b.ts", "src/**/b.ts")).toBe(true)
		expect(matchesGlob("src/a/b.ts", "src/*/b.ts")).toBe(true)
		expect(matchesGlob("src/a/b/c.ts", "src/*/c.ts")).toBe(false)
	})

	test("secrets never reach Jev, at any depth", () => {
		expect(isDenied(".env")).toBe(true)
		expect(isDenied("apps/api/.env.local")).toBe(true)
		expect(isDenied("certs/server.pem")).toBe(true)
		expect(isDenied("config/secrets/db.ts")).toBe(true)
		expect(isDenied("src/keyboard.ts")).toBe(false)
		// Our own key file, at any depth: never sent, deny-listed not merely
		// skipped for its extension.
		expect(isDenied(".opencode/jev-gate.json")).toBe(true)
	})

	test("reviewable and test detection", () => {
		expect(isReviewable("src/a.ts")).toBe(true)
		expect(isReviewable("README.md")).toBe(false)
		expect(isTestFile("src/a.test.ts")).toBe(true)
		expect(isTestFile("test/helpers.ts")).toBe(true)
		expect(isTestFile("src/latest.ts")).toBe(false)
	})
})

describe("parseHunks", () => {
	test("numbers added lines against the new file", () => {
		const hunks = parseHunks(readFileSync(join(FIXTURES, "01-inverted-if.patch"), "utf8"))
		expect(hunks.length).toBe(1)
		expect(hunks[0].startLine).toBeGreaterThan(0)
		expect(hunks[0].endLine).toBeGreaterThanOrEqual(hunks[0].startLine)
		expect(hunks[0].text).toContain("@@")
	})

	test("a pure deletion still yields a hunk to point at", () => {
		const patch = [
			"@@ -10,4 +10,2 @@",
			" keep",
			"-app.use(requireRole('admin'))",
			"-app.use(auditLog())",
			" keep",
		].join("\n")
		const hunks = parseHunks(patch)
		expect(hunks.length).toBe(1)
		// The deleted lines sat right after the first context line.
		expect(hunks[0].startLine).toBe(11)
	})

	test("multiple hunks stay separate", () => {
		const patch = ["@@ -1,2 +1,3 @@", " a", "+b", "@@ -20,2 +21,3 @@", " c", "+d"].join("\n")
		expect(parseHunks(patch).map((h) => h.startLine)).toEqual([2, 22])
	})
})

describe("splitDiff", () => {
	test("one patch per file, path from the +++ line", () => {
		const diff = [
			"diff --git a/src/a.ts b/src/a.ts",
			"--- a/src/a.ts",
			"+++ b/src/a.ts",
			"@@ -1 +1 @@",
			"-a",
			"+b",
			"diff --git a/src/b.ts b/src/b.ts",
			"--- a/src/b.ts",
			"+++ b/src/b.ts",
			"@@ -1 +1 @@",
			"-c",
			"+d",
		].join("\n")
		expect(splitDiff(diff).map((f) => f.path)).toEqual(["src/a.ts", "src/b.ts"])
	})
})

describe("rankSignals", () => {
	const matrix = {
		"src/a.ts": { security: 0.9, correctness: 0.75, compatibility: 0.99, testGap: 0.71 },
		"src/b.ts": { correctness: 0.9, reliability: 0.2 },
	}

	test("orders by probability, then by dimension priority", () => {
		const ranked = rankSignals(matrix, 10)
		expect(ranked[0]).toEqual({ file: "src/a.ts", dimension: "security", probability: 0.9 })
		// Same 0.9: security outranks correctness, so b.ts comes second.
		expect(ranked[1]).toEqual({ file: "src/b.ts", dimension: "correctness", probability: 0.9 })
	})

	test("drops everything under threshold", () => {
		expect(rankSignals(matrix, 10).every((s) => s.probability >= 0.7)).toBe(true)
		expect(rankSignals(matrix, 10, 0.95)).toEqual([])
	})

	test("compatibility never opens a follow-up, however high it scores", () => {
		// Spike A: it scored highest on the clean fixture. Screened, not located.
		expect(rankSignals(matrix, 10).some((s) => s.dimension === "compatibility")).toBe(false)
	})

	test("respects MAX_FOLLOW_UPS", () => {
		expect(rankSignals(matrix, 2).length).toBe(2)
	})
})

describe("selectFiles", () => {
	test("keeps source, explains every skip", () => {
		const { targets, skipped } = selectFiles([
			{ path: "src/a.ts", patch: "@@ -1 +1 @@\n+x" },
			{ path: "src/a.test.ts", patch: "@@ -1 +1 @@\n+x" },
			{ path: ".env", patch: "@@ -1 +1 @@\n+SECRET=1" },
			{ path: "README.md", patch: "@@ -1 +1 @@\n+docs" },
			{ path: "src/empty.ts", patch: "   " },
		])
		expect(targets.map((f) => f.path)).toEqual(["src/a.ts"])
		expect(skipped.map((s) => s.path)).toEqual([
			"src/a.test.ts",
			".env",
			"README.md",
			"src/empty.ts",
		])
		expect(skipped.every((s) => s.reason.length > 0)).toBe(true)
	})
})

describe("decideAction", () => {
	const finding = (severity: number) => ({
		file: "src/a.ts",
		dimension: "security" as const,
		probability: 0.9,
		startLine: 1,
		endLine: 2,
		mechanism: "missing_authz",
		severity,
	})

	test("blocks at or over BLOCKING_SEVERITY, and never approves", () => {
		expect(decideAction([finding(2)], 2)).toBe("request_changes")
		expect(decideAction([finding(1)], 2)).toBe("comment")
		expect(decideAction([], 2)).toBe("clean")
	})
})

describe("runChangeReview", () => {
	const authPatch = readFileSync(join(FIXTURES, "02-removed-auth-check.patch"), "utf8")

	test("screens, locates, and blocks - with the Spike A numbers", async () => {
		const client = new FakeClient((req) => {
			const names = Object.keys(req.questions)
			if (names.includes("correctness")) {
				return noulAnswers({
					correctness: 0.89,
					security: 0.98,
					reliability: 0.28,
					compatibility: 0.83,
					testGap: 0.96,
				})
			}
			if (names[0] === "hunk") {
				return { model: "fake", answers: { hunk: { type: "choice", choice: "hunk_0" } } }
			}
			if (names[0] === "mechanism") {
				return {
					model: "fake",
					answers: { mechanism: { type: "choice", choice: "missing_authz" } },
				}
			}
			if (names[0] === "severity") {
				return { model: "fake", answers: { severity: { type: "score", score: 3 } } }
			}
			return { model: "fake", answers: { owner: { type: "choice", choice: "security" } } }
		})

		const report = await runChangeReview(client, [{ path: "src/api/router.ts", patch: authPatch }])

		expect(report.action).toBe("request_changes")
		expect(report.source).toBe("jev")
		expect(report.matrix["src/api/router.ts"].security).toBe(0.98)
		const security = report.findings.find((f) => f.dimension === "security")
		expect(security?.mechanism).toBe("missing_authz")
		expect(security?.severity).toBe(3)
		expect(security?.owner).toBe("security")
		expect(security!.startLine).toBeGreaterThan(0)
		expect(report.usage.requests).toBeGreaterThan(1)
		expect(report.questionsVersion).toMatch(/^\d{4}-\d{2}-\d{2}/)
	})

	test("a clean patch is clean, and never reaches locate", async () => {
		const client = new FakeClient(() =>
			noulAnswers({
				correctness: 0.17,
				security: 0.03,
				reliability: 0.04,
				compatibility: 0.86,
				testGap: 0.2,
			}),
		)
		const report = await runChangeReview(client, [
			{ path: "src/a.ts", patch: readFileSync(join(FIXTURES, "04-clean-rename.patch"), "utf8") },
		])
		expect(report.action).toBe("clean")
		expect(report.findings).toEqual([])
		// One screen request, no follow-ups: compatibility 0.86 did not promote.
		expect(client.calls.length).toBe(1)
	})

	test("noMatch produces no finding - Jev's silence is not a finding", async () => {
		const client = new FakeClient((req) => {
			const names = Object.keys(req.questions)
			if (names.includes("correctness")) return noulAnswers({ security: 0.95 })
			return { model: "fake", answers: { hunk: { type: "choice", choice: "noMatch" } } }
		})
		const report = await runChangeReview(client, [{ path: "src/api/router.ts", patch: authPatch }])
		expect(report.findings).toEqual([])
		expect(report.action).toBe("clean")
		// The signal still shows in the matrix; it just has nowhere to point.
		expect(report.matrix["src/api/router.ts"].security).toBe(0.95)
	})

	test("denied files never reach the client", async () => {
		const client = new FakeClient(() => noulAnswers({ security: 0.9 }))
		const report = await runChangeReview(client, [
			{ path: "apps/api/.env", patch: "@@ -1 +1 @@\n+TYPESAFE_API_KEY=real" },
		])
		expect(client.calls.length).toBe(0)
		expect(report.skipped[0].reason).toContain("deny glob")
	})
})

describe("client", () => {
	test("no key means refusal, not a silent pass", () => {
		const previous = process.env.TYPESAFE_API_KEY
		delete process.env.TYPESAFE_API_KEY
		try {
			expect(() => new HttpJevClient()).toThrow(MissingKeyError)
		} finally {
			if (previous !== undefined) process.env.TYPESAFE_API_KEY = previous
		}
	})

	test("posts the documented shape with a bearer key", async () => {
		let seen: { url: string; init: RequestInit } | undefined
		const client = new HttpJevClient({
			apiKey: "test-key",
			fetchImpl: (async (url: string, init: RequestInit) => {
				seen = { url, init }
				return new Response(JSON.stringify({ model: "jev", answers: {} }), { status: 200 })
			}) as unknown as typeof fetch,
		})
		await client.systemOne({ state: { a: 1 }, questions: { q: { type: "noul", instructions: "x" } } })

		expect(seen!.url).toBe("https://api.typesafe.ai/v1/systemone")
		expect((seen!.init.headers as Record<string, string>).Authorization).toBe("Bearer test-key")
		expect(JSON.parse(seen!.init.body as string).questions.q.type).toBe("noul")
	})

	test("an HTTP error is an error, not an empty review", async () => {
		const client = new HttpJevClient({
			apiKey: "k",
			fetchImpl: (async () => new Response("nope", { status: 401 })) as unknown as typeof fetch,
		})
		expect(client.systemOne({ state: {}, questions: { q: { type: "noul", instructions: "x" } } })).rejects.toThrow(
			/401/,
		)
	})
})
