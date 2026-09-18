import { describe, expect, test } from "bun:test"
import { checkPage, summarizePageCheck } from "../src/review/page"
import type { JevClient, SystemOneRequest, SystemOneResponse } from "../src/jev/client"

class FakeClient implements JevClient {
	readonly calls: SystemOneRequest[] = []
	constructor(private readonly noul: number) {}
	async systemOne(request: SystemOneRequest): Promise<SystemOneResponse> {
		this.calls.push(request)
		return {
			model: "fake",
			answers: { thenHolds: { type: "noul", noul: this.noul } },
			usage: { input_tokens: 80, output_tokens: 8 },
		}
	}
}

describe("checkPage", () => {
	test("a Then Jev scores at 0.94 is PASS", async () => {
		const client = new FakeClient(0.94)
		const result = await checkPage(client, {
			snapshot: "- heading: Workflows\n- button: New workflow",
			then: "the New workflow action is available",
		})
		expect(result.verdict).toBe("PASS")
		expect(result.probability).toBe(0.94)
		expect(result.then).toBe("the New workflow action is available")
		expect(client.calls[0]?.state).toEqual({
			page: {
				snapshot: "- heading: Workflows\n- button: New workflow",
				then: "the New workflow action is available",
			},
		})
	})

	test("a Then Jev scores at 0.11 is FAIL", async () => {
		const client = new FakeClient(0.11)
		const result = await checkPage(client, {
			snapshot: "- alert: Failed to load workflows",
			then: "the Workflows page lists existing workflows",
		})
		expect(result.verdict).toBe("FAIL")
		expect(result.probability).toBe(0.11)
	})
})

describe("summarizePageCheck", () => {
	test("PASS keeps Jev's probability on the line", () => {
		const md = summarizePageCheck({
			verdict: "PASS",
			probability: 0.94,
			then: "the New workflow action is available",
			questionsVersion: "page-then-2026-09-17.1",
			latencyMs: 310,
			source: "jev",
		})
		expect(md).toContain("**Jev Then - PASS**")
		expect(md).toContain("94%")
		expect(md).toContain("the New workflow action is available")
		expect(md).toContain("Do not add your own")
	})
})
