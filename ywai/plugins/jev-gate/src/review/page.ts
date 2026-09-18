/**
 * Score a Gherkin Then against an accessibility snapshot.
 *
 * Playwright or chrome-devtools describes the page. Jev answers one noul.
 * Jev never clicks. Missing probability is not PASS.
 */
import { PAGE_THEN_THRESHOLD } from "../domain/config"
import { noulOf, type JevClient } from "../jev/client"
import { PAGE_THEN_VERSION, thenQuestion, thenState } from "../jev/questions"

export interface PageCheckInput {
	snapshot: string
	then: string
	url?: string
	title?: string
	threshold?: number
	now?: () => number
}

export interface PageCheckResult {
	verdict: "PASS" | "FAIL"
	probability: number
	then: string
	questionsVersion: string
	latencyMs: number
	source: "jev"
}

export async function checkPage(client: JevClient, input: PageCheckInput): Promise<PageCheckResult> {
	const now = input.now ?? Date.now
	const startedAt = now()
	const threshold = input.threshold ?? PAGE_THEN_THRESHOLD
	const response = await client.systemOne({
		state: thenState(input.snapshot, input.then, input.url, input.title),
		questions: { thenHolds: thenQuestion() },
	})
	const probability = noulOf(response.answers, "thenHolds")
	if (typeof probability !== "number") {
		throw new Error("Jev did not return a Then probability. This is not a passing check.")
	}
	return {
		verdict: probability >= threshold ? "PASS" : "FAIL",
		probability,
		then: input.then,
		questionsVersion: PAGE_THEN_VERSION,
		latencyMs: now() - startedAt,
		source: "jev",
	}
}

export function summarizePageCheck(result: PageCheckResult): string {
	const pct = `${Math.round(result.probability * 100)}%`
	return [
		`**Jev Then - ${result.verdict}** (${pct}, ${(result.latencyMs / 1000).toFixed(1)}s)`,
		`Then: ${result.then}`,
		`questions ${result.questionsVersion} - source: ${result.source}`,
		"These findings are Jev's. Do not add your own and attribute them to Jev, and do not restate one as more certain than its probability.",
	].join("\n")
}
