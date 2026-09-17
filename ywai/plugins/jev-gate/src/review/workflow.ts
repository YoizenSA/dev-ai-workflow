/**
 * The change-review pipeline (PLAN 4.1). Not negotiable, in this order:
 * discover -> screen -> filter -> locate -> action.
 *
 * Pure orchestration: it takes files and a client, and knows nothing about git
 * or OpenCode. That is what makes it testable without a network.
 */
import {
	BLOCKING_SEVERITY,
	CONCURRENCY,
	MAX_FOLLOW_UPS,
	OWNER_SEVERITY,
	SCREEN_THRESHOLD,
	isDenied,
	isReviewable,
	isTestFile,
} from "../domain/config"
import { parseHunks } from "../domain/patch"
import type {
	ChangedFile,
	Finding,
	ReviewAction,
	ReviewReport,
	ScreenMatrix,
} from "../domain/types"
import type { JevClient } from "../jev/client"
import { QUESTIONS_VERSION } from "../jev/questions"
import { emptyUsage, locateSignal, rankSignals, screenFile } from "./judgments"

export interface ReviewOptions {
	runId?: string
	threshold?: number
	maxFollowUps?: number
	ownerSeverity?: number
	blockingSeverity?: number
	concurrency?: number
	now?: () => number
}

/** Run `worker` over `items`, at most `limit` requests in flight. */
async function mapWithLimit<T, R>(
	items: T[],
	limit: number,
	worker: (item: T) => Promise<R>,
): Promise<R[]> {
	const results = new Array<R>(items.length)
	let next = 0
	const runners = Array.from({ length: Math.min(limit, items.length) }, async () => {
		while (true) {
			const index = next++
			if (index >= items.length) return
			results[index] = await worker(items[index])
		}
	})
	await Promise.all(runners)
	return results
}

/** Which files get sent, and why the others do not. */
export function selectFiles(files: ChangedFile[]): {
	targets: ChangedFile[]
	skipped: Array<{ path: string; reason: string }>
} {
	const targets: ChangedFile[] = []
	const skipped: Array<{ path: string; reason: string }> = []
	for (const file of files) {
		if (isDenied(file.path)) {
			skipped.push({ path: file.path, reason: "deny glob: never sent to Jev" })
		} else if (!isReviewable(file.path)) {
			skipped.push({ path: file.path, reason: "not JS/TS (v1 scope)" })
		} else if (isTestFile(file.path)) {
			skipped.push({ path: file.path, reason: "test file: context, not target" })
		} else if (!file.patch.trim()) {
			skipped.push({ path: file.path, reason: "empty patch" })
		} else {
			targets.push(file)
		}
	}
	return { targets, skipped }
}

export function decideAction(findings: Finding[], blockingSeverity: number): ReviewAction {
	if (findings.some((finding) => finding.severity >= blockingSeverity)) {
		return "request_changes"
	}
	return findings.length > 0 ? "comment" : "clean"
}

export async function runChangeReview(
	client: JevClient,
	files: ChangedFile[],
	options: ReviewOptions = {},
): Promise<ReviewReport> {
	const now = options.now ?? Date.now
	const startedAt = now()
	const threshold = options.threshold ?? SCREEN_THRESHOLD
	const ownerSeverity = options.ownerSeverity ?? OWNER_SEVERITY
	const blockingSeverity = options.blockingSeverity ?? BLOCKING_SEVERITY
	const usage = emptyUsage()

	const { targets, skipped } = selectFiles(files)

	const matrix: ScreenMatrix = {}
	const screened = await mapWithLimit(
		targets,
		options.concurrency ?? CONCURRENCY,
		async (file) => ({
			path: file.path,
			scores: await screenFile(client, file.path, file.patch, usage),
		}),
	)
	for (const { path, scores } of screened) matrix[path] = scores

	const signals = rankSignals(matrix, options.maxFollowUps ?? MAX_FOLLOW_UPS, threshold)
	const byPath = new Map(targets.map((file) => [file.path, file]))

	const findings: Finding[] = []
	for (const signal of signals) {
		const file = byPath.get(signal.file)
		if (!file) continue
		const located = await locateSignal(
			client,
			signal,
			parseHunks(file.patch),
			usage,
			ownerSeverity,
		)
		// No hunk, no finding: a signal we cannot place stays in the matrix.
		if (!located) continue
		findings.push({
			file: signal.file,
			dimension: signal.dimension,
			probability: signal.probability,
			startLine: located.hunk.startLine,
			endLine: located.hunk.endLine,
			mechanism: located.mechanism,
			severity: located.severity,
			owner: located.owner,
		})
	}

	findings.sort((a, b) => b.severity - a.severity || b.probability - a.probability)

	return {
		runId: options.runId ?? `run_${startedAt.toString(36)}`,
		action: decideAction(findings, blockingSeverity),
		matrix,
		findings,
		skipped,
		source: "jev",
		questionsVersion: QUESTIONS_VERSION,
		usage,
		latencyMs: now() - startedAt,
	}
}
