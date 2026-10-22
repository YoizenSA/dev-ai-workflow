/**
 * Run persistence (PLAN 3 / 4).
 *
 * `ctx.storage` is the real store (get / set / remove / scan, confirmed on
 * opencode v2.0.6). Everything here degrades to an in-memory map when storage
 * is missing or throws, because losing a report must never fail a review that
 * already cost real requests.
 */
import type { ReviewReport } from "../domain/types"

export interface StorageLike {
	get?(key: unknown): Promise<unknown>
	set?(key: unknown, value: unknown): Promise<unknown>
	scan?(prefix: unknown): Promise<unknown>
}

/** The last decision of a session, which the Fase 5 gates read. */
export interface SessionDecision {
	runId: string
	action: ReviewReport["action"]
	blockingFindings: number
	source: ReviewReport["source"]
	at: string
}

/** What the gate reads: the last review and the last route of a session. */
export interface RouteRecord {
	choice: string
	closeCall: boolean
	source: "jev" | "llm"
	at: string
}

const memory = new Map<string, unknown>()

const runKey = (runId: string) => `jev/runs/${runId}`
const decisionKey = (sessionID: string) => `jev/sessions/${sessionID}`
const routeKey = (sessionID: string) => `jev/routes/${sessionID}`

async function write(storage: StorageLike | undefined, key: string, value: unknown) {
	memory.set(key, value)
	try {
		await storage?.set?.(key, value)
	} catch {
		// Memory already holds it; a storage miss is not worth failing a run.
	}
}

async function read(storage: StorageLike | undefined, key: string): Promise<unknown> {
	try {
		const stored = await storage?.get?.(key)
		if (stored !== undefined && stored !== null) return stored
	} catch {
		// Fall through to memory.
	}
	return memory.get(key)
}

export async function saveRun(storage: StorageLike | undefined, report: ReviewReport) {
	await write(storage, runKey(report.runId), report)
}

export async function loadRun(
	storage: StorageLike | undefined,
	runId: string,
): Promise<ReviewReport | undefined> {
	return (await read(storage, runKey(runId))) as ReviewReport | undefined
}

/**
 * Record what this session's review concluded.
 *
 * `source` rides along on purpose: a gate must ignore an `llm` decision, and
 * it can only do that if the decision says where it came from (PLAN 3.1).
 */
export async function saveDecision(
	storage: StorageLike | undefined,
	sessionID: string,
	report: ReviewReport,
) {
	const decision: SessionDecision = {
		runId: report.runId,
		action: report.action,
		blockingFindings: report.findings.filter((f) => f.severity >= 2).length,
		source: report.source,
		at: new Date().toISOString(),
	}
	await write(storage, decisionKey(sessionID), decision)
}

export async function loadDecision(
	storage: StorageLike | undefined,
	sessionID: string,
): Promise<SessionDecision | undefined> {
	return (await read(storage, decisionKey(sessionID))) as SessionDecision | undefined
}

export async function saveRoute(
	storage: StorageLike | undefined,
	sessionID: string,
	route: { choice: string; closeCall: boolean; source: "jev" | "llm" },
) {
	const record: RouteRecord = { ...route, at: new Date().toISOString() }
	await write(storage, routeKey(sessionID), record)
}

export async function loadRoute(
	storage: StorageLike | undefined,
	sessionID: string,
): Promise<RouteRecord | undefined> {
	return (await read(storage, routeKey(sessionID))) as RouteRecord | undefined
}
