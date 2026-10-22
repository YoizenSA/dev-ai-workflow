/** Shared vocabulary for the Jev review pipeline. No SDK, no OpenCode, no git. */

/** The five screen dimensions (PLAN 3.4). */
export type Dimension =
	| "correctness"
	| "security"
	| "reliability"
	| "compatibility"
	| "testGap"

/** One changed file: the unit every screen question inspects. */
export interface ChangedFile {
	path: string
	patch: string
}

/** One added-line region of a unified diff. */
export interface Hunk {
	/** Line number in the new file where the hunk's added lines start. */
	startLine: number
	endLine: number
	/** The hunk body as it appears in the patch, markers included. */
	text: string
}

/** A dimension that screened over threshold on one file. */
export interface Signal {
	file: string
	dimension: Dimension
	probability: number
}

/** A signal that survived locate: a defensible place plus what goes wrong. */
export interface Finding {
	file: string
	dimension: Dimension
	probability: number
	startLine: number
	endLine: number
	/** How it breaks, in Jev's words - never the agent's. */
	mechanism: string
	/** 0-3 rubric; at or over BLOCKING_SEVERITY it blocks. */
	severity: number
	/** Only assigned at or over OWNER_SEVERITY. */
	owner?: string
}

/** There is no `approve` - Jev never approves (PLAN 0). */
export type ReviewAction = "request_changes" | "comment" | "clean"

/** Per-file, per-dimension screen probabilities. */
export type ScreenMatrix = Record<string, Partial<Record<Dimension, number>>>

export interface ReviewReport {
	runId: string
	action: ReviewAction
	matrix: ScreenMatrix
	findings: Finding[]
	/**
	 * Locatable signals that crossed the screen threshold but could not be
	 * placed on a changed hunk, so they never became findings.
	 *
	 * The count travels because the summary cannot re-derive it: the matrix
	 * also holds `compatibility` and `testGap`, which score high on almost
	 * everything and are never promoted. Counting rows in the matrix instead
	 * reads that noise as a lost signal and prints a caveat on a clean run.
	 */
	unplaced: number
	/** Files seen but deliberately not sent, with the reason. */
	skipped: Array<{ path: string; reason: string }>
	/** Always "jev" here; "llm" marks a fallback that gates must ignore. */
	source: "jev" | "llm"
	questionsVersion: string
	usage: { inputTokens: number; outputTokens: number; requests: number }
	latencyMs: number
}
