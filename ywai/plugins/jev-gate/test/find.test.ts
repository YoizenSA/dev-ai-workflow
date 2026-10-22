import { describe, expect, test } from "bun:test"
import { MAX_SEGMENT_CHARS, SEGMENT_LINES, segmentFile, snippetOf } from "../src/find/segment"
import { findInSegments, segmentQuestion } from "../src/find/score"
import type { JevClient, SystemOneRequest, SystemOneResponse } from "../src/jev/client"

const lines = (count: number) => Array.from({ length: count }, (_, i) => `line ${i + 1}`).join("\n")

describe("segmentFile", () => {
	test("a short file is one segment, numbered from 1", () => {
		const segments = segmentFile("src/a.ts", lines(10))
		expect(segments.length).toBe(1)
		expect(segments[0]).toMatchObject({ file: "src/a.ts", startLine: 1, endLine: 10 })
	})

	test("windows overlap, so a straddling function is whole somewhere", () => {
		const segments = segmentFile("src/a.ts", lines(120))
		expect(segments.length).toBeGreaterThan(1)
		const [first, second] = segments
		expect(first.endLine).toBe(SEGMENT_LINES)
		// 60-line windows with 10 of overlap advance by 50.
		expect(second.startLine).toBe(51)
		expect(second.startLine).toBeLessThan(first.endLine)
	})

	test("the last window does not repeat the tail", () => {
		const segments = segmentFile("src/a.ts", lines(120))
		const last = segments[segments.length - 1]
		expect(last.endLine).toBe(120)
		expect(segments.filter((s) => s.endLine === 120).length).toBe(1)
	})

	test("an oversized segment is trimmed, not sent whole", () => {
		const huge = Array.from({ length: 40 }, () => "x".repeat(500)).join("\n")
		const [segment] = segmentFile("src/big.ts", huge)
		expect(segment.text.length).toBe(MAX_SEGMENT_CHARS)
	})

	test("blank stretches produce no segment to pay for", () => {
		expect(segmentFile("src/a.ts", "\n\n\n")).toEqual([])
	})

	test("the snippet is short enough to read in a tool result", () => {
		const [segment] = segmentFile("src/a.ts", lines(30))
		expect(snippetOf(segment).split("\n").length).toBe(6)
	})
})

describe("segmentQuestion", () => {
	test("names the three ways a semantic grep returns junk", () => {
		const question = segmentQuestion("where do we validate the token", "segment_0")
		const criteria = JSON.stringify(question.criteria).toLowerCase()
		expect(criteria).toContain("comment")
		expect(criteria).toContain("readme")
		expect(criteria).toContain("identifier")
		// Each question inspects only its own segment.
		expect(JSON.stringify(question.instructions)).toContain("segment_0.text")
	})
})

/** Answers every segment question with a score keyed by the segment's file. */
const scriptedClient = (byFile: Record<string, number>, calls: SystemOneRequest[] = []): JevClient => ({
	async systemOne(request: SystemOneRequest) {
		calls.push(request)
		const state = request.state as Record<string, { file: string }>
		const answers: SystemOneResponse["answers"] = {}
		for (const key of Object.keys(request.questions)) {
			answers[key] = { type: "noul", noul: byFile[state[key]?.file] ?? 0 }
		}
		return { model: "fake", answers, usage: { input_tokens: 50, output_tokens: 5 } }
	},
})

describe("findInSegments", () => {
	const segments = [
		...segmentFile("src/auth.ts", lines(10)),
		...segmentFile("src/util.ts", lines(10)),
		...segmentFile("docs/readme-ish.ts", lines(10)),
	]

	test("keeps only what clears the threshold, ordered by probability", async () => {
		const result = await findInSegments(
			scriptedClient({ "src/auth.ts": 0.95, "src/util.ts": 0.4, "docs/readme-ish.ts": 0.82 }),
			"where do we validate the token",
			segments,
		)
		expect(result.hits.map((h) => h.file)).toEqual(["src/auth.ts", "docs/readme-ish.ts"])
		expect(result.hits[0].probability).toBe(0.95)
		expect(result.hits[0].snippet.length).toBeGreaterThan(0)
	})

	test("a custom threshold is honoured", async () => {
		const result = await findInSegments(
			scriptedClient({ "src/auth.ts": 0.95, "src/util.ts": 0.4, "docs/readme-ish.ts": 0.82 }),
			"q",
			segments,
			{ threshold: 0.9 },
		)
		expect(result.hits.map((h) => h.file)).toEqual(["src/auth.ts"])
	})

	test("batching changes the request count, not the answers", async () => {
		const scores = { "src/auth.ts": 0.95, "src/util.ts": 0.4, "docs/readme-ish.ts": 0.82 }
		const oneByOne: SystemOneRequest[] = []
		const batched: SystemOneRequest[] = []

		const single = await findInSegments(scriptedClient(scores, oneByOne), "q", segments, {
			batchSize: 1,
		})
		const five = await findInSegments(scriptedClient(scores, batched), "q", segments, {
			batchSize: 5,
		})

		expect(oneByOne.length).toBe(3)
		expect(batched.length).toBe(1)
		expect(single.hits.map((h) => h.file)).toEqual(five.hits.map((h) => h.file))
		// Fase 3 measures whether precision survives batching against a real
		// Jev; this only pins that the plumbing is equivalent.
		expect(five.usage.requests).toBeLessThan(single.usage.requests)
	})

	test("a missing answer scores zero rather than inventing a hit", async () => {
		const silent: JevClient = {
			async systemOne() {
				return { model: "fake", answers: {} }
			},
		}
		const result = await findInSegments(silent, "q", segments)
		expect(result.hits).toEqual([])
	})

	test("over the cap it stops and says so", async () => {
		const many = Array.from({ length: 205 }, (_, i) => segmentFile(`src/f${i}.ts`, lines(5))[0])
		const result = await findInSegments(scriptedClient({}), "q", many, { batchSize: 50 })
		expect(result.truncated).toBe(true)
		expect(result.scannedSegments).toBe(200)
		// Coverage is only legible against the whole corpus: 200 of 200 and
		// 200 of 205 print the same number and mean different things.
		expect(result.totalSegments).toBe(205)
	})

	test("a full pass reports total equal to scanned, so coverage reads as complete", async () => {
		const few = Array.from({ length: 12 }, (_, i) => segmentFile(`src/f${i}.ts`, lines(5))[0])
		const result = await findInSegments(scriptedClient({}), "q", few)
		expect(result.truncated).toBe(false)
		expect(result.scannedSegments).toBe(result.totalSegments)
	})

	test("limit caps what comes back", async () => {
		const many = Array.from({ length: 30 }, (_, i) => segmentFile(`src/f${i}.ts`, lines(5))[0])
		const scores = Object.fromEntries(many.map((s) => [s.file, 0.9]))
		const result = await findInSegments(scriptedClient(scores), "q", many, { limit: 5 })
		expect(result.hits.length).toBe(5)
	})
})
