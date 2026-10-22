/**
 * Semantic grep (PLAN 4.3).
 *
 * One noul per segment: does this text carry direct evidence of the query?
 * The `false` criteria are the whole trick - a name that merely matches, a
 * comment that mentions the term, or a doc file are the three ways a grep-like
 * tool returns junk, so they are named explicitly rather than left implied.
 */
import { CONCURRENCY } from "../domain/config"
import type { JevClient, Question } from "../jev/client"
import { addUsage, emptyUsage, type Usage } from "../review/judgments"
import { MAX_FIND_SEGMENTS, snippetOf, type Segment } from "./segment"

export const FIND_THRESHOLD = 0.81
/** PROVISIONAL: measured against 1 in the Fase 3 batching check. */
export const FIND_BATCH = 5

export interface Hit {
	file: string
	startLine: number
	endLine: number
	snippet: string
	probability: number
}

export interface FindResult {
	query: string
	hits: Hit[]
	scannedSegments: number
	/** Segments the corpus produced, before the cap. Coverage is scanned/total. */
	totalSegments: number
	truncated: boolean
	usage: Usage
	latencyMs: number
}

/** One question per segment, each pointed at its own key in the state. */
export function segmentQuestion(query: string, key: string): Question {
	return {
		type: "noul",
		instructions: {
			question: `Does ${key}.text contain direct evidence of: ${query}?`,
			inspect: `${key}.text`,
			focus: "Code that actually does it, not code that mentions it",
		},
		criteria: {
			true: {
				what: "The code in this segment does the thing described",
				examples: ["The function that performs it", "The call site that triggers it"],
			},
			false: {
				what: "Only a name, a mention, or documentation matches",
				examples: [
					"An identifier that happens to contain the word",
					"A comment or TODO about it",
					"A README or changelog describing it",
					"An import of something that does it elsewhere",
				],
			},
		},
	}
}

async function scoreBatch(
	client: JevClient,
	query: string,
	batch: Segment[],
	usage: Usage,
): Promise<Array<{ segment: Segment; probability: number }>> {
	const state: Record<string, unknown> = {}
	const questions: Record<string, Question> = {}
	batch.forEach((segment, index) => {
		const key = `segment_${index}`
		state[key] = { file: segment.file, text: segment.text }
		questions[key] = segmentQuestion(query, key)
	})

	const response = await client.systemOne({ state, questions })
	addUsage(usage, response.usage)

	return batch.map((segment, index) => ({
		segment,
		probability: response.answers?.[`segment_${index}`]?.noul ?? 0,
	}))
}

/** Run `worker` over `items`, at most `limit` requests in flight. */
async function mapWithLimit<T, R>(
	items: T[],
	limit: number,
	worker: (item: T) => Promise<R>,
): Promise<R[]> {
	const results = new Array<R>(items.length)
	let next = 0
	await Promise.all(
		Array.from({ length: Math.min(limit, items.length) }, async () => {
			while (true) {
				const index = next++
				if (index >= items.length) return
				results[index] = await worker(items[index])
			}
		}),
	)
	return results
}

export interface FindOptions {
	threshold?: number
	limit?: number
	batchSize?: number
	concurrency?: number
	now?: () => number
}

export async function findInSegments(
	client: JevClient,
	query: string,
	segments: Segment[],
	options: FindOptions = {},
): Promise<FindResult> {
	const now = options.now ?? Date.now
	const startedAt = now()
	const usage = emptyUsage()
	const threshold = options.threshold ?? FIND_THRESHOLD
	const batchSize = Math.max(1, options.batchSize ?? FIND_BATCH)

	const truncated = segments.length > MAX_FIND_SEGMENTS
	const scanned = truncated ? segments.slice(0, MAX_FIND_SEGMENTS) : segments

	const batches: Segment[][] = []
	for (let i = 0; i < scanned.length; i += batchSize) {
		batches.push(scanned.slice(i, i + batchSize))
	}

	const scoredBatches = await mapWithLimit(
		batches,
		options.concurrency ?? CONCURRENCY,
		(batch) => scoreBatch(client, query, batch, usage),
	)

	const hits: Hit[] = []
	for (const scored of scoredBatches) {
		for (const { segment, probability } of scored ?? []) {
			if (probability < threshold) continue
			hits.push({
				file: segment.file,
				startLine: segment.startLine,
				endLine: segment.endLine,
				snippet: snippetOf(segment),
				probability,
			})
		}
	}

	hits.sort((a, b) => b.probability - a.probability || a.file.localeCompare(b.file))

	return {
		query,
		hits: hits.slice(0, options.limit ?? 20),
		scannedSegments: scanned.length,
		totalSegments: segments.length,
		truncated,
		usage,
		latencyMs: now() - startedAt,
	}
}
