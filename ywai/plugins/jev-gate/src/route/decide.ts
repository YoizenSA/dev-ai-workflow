/**
 * Route: who should execute this task (PLAN 4.4).
 *
 * Jev picks one option from the catalog. The code - never Jev - derives the
 * reason codes and decides whether the answer is close enough to escalate.
 *
 * Spike A finding applied: noul answers carry no `confidence`, so a Choice is
 * not assumed to carry one either. Confidence here is read from the answer's
 * own probabilities when they come back, and is otherwise undefined; the
 * close-call rule then rests on the margin alone rather than on an invented
 * number.
 */
import type { JevClient } from "../jev/client"
import { buildCatalog, defaultCatalog, type AgentLike, type RouteOption } from "./catalog"

export const ROUTE_MARGIN = 0.12
/** PROVISIONAL until Fase 6: only applied when probabilities come back. */
export const ROUTE_MIN_CONFIDENCE = 0.6

export interface RouteInput {
	task: string
	wantsWrite?: boolean
	filesHint?: string[]
	failingTests?: boolean
	touchesDeniedPath?: boolean
	/** The agents the host reports; empty falls back to the known roster. */
	agents?: AgentLike[]
}

export interface RouteDecision {
	choice: string
	/** The agent to switch to, when the choice maps to one. */
	agent?: string
	writes: boolean
	probabilities?: Record<string, number>
	confidence?: number
	closeCall: boolean
	reasonCodes: string[]
	source: "jev" | "llm"
}

/** Ordered so the message reads the same way every time. */
function deriveReasonCodes(
	input: RouteInput,
	closeCall: boolean,
	lowConfidence: boolean,
	source: "jev" | "llm",
): string[] {
	const codes: string[] = []
	if (closeCall) codes.push("close_call")
	if (lowConfidence) codes.push("low_confidence")
	if (input.wantsWrite) codes.push("write_requested")
	if (input.failingTests) codes.push("failing_tests")
	if (input.touchesDeniedPath) codes.push("touches_denied_path")
	if (source === "llm") codes.push("llm_fallback")
	return codes
}

/** Gap between the top two probabilities, if the answer carried any. */
export function topMargin(probabilities: Record<string, number> | undefined): number | undefined {
	if (!probabilities) return undefined
	const sorted = Object.values(probabilities).sort((a, b) => b - a)
	if (sorted.length < 2) return undefined
	return sorted[0] - sorted[1]
}

/** Criteria as Jev consumes them: the gate fields stay on our side. */
function toCriteria(catalog: Record<string, RouteOption>): Record<string, unknown> {
	return Object.fromEntries(
		Object.entries(catalog).map(([name, option]) => [
			name,
			{ what: option.what, not_for: option.not_for, examples: option.examples },
		]),
	)
}

export async function routeTask(client: JevClient, input: RouteInput): Promise<RouteDecision> {
	const catalog = input.agents?.length ? buildCatalog(input.agents) : defaultCatalog()

	const response = await client.systemOne({
		state: {
			task: input.task,
			wantsWrite: input.wantsWrite ?? false,
			files: input.filesHint ?? [],
			failingTests: input.failingTests ?? false,
		},
		questions: {
			route: {
				type: "choice",
				instructions: {
					question: "Who should execute this task? Choose who SHOULD do it, not who could.",
					inspect: "task",
				},
				criteria: toCriteria(catalog),
			},
		},
	})

	const answer = response.answers?.route
	const raw = answer?.choice
	const known = raw !== undefined && catalog[raw] !== undefined
	// An unrecognised label is itself a close call: escalate to a person
	// rather than guess which agent Jev meant.
	const choice = known ? (raw as string) : "human"
	const option = catalog[choice] ?? catalog.human

	const probabilities = answer?.probabilities
	const confidence = probabilities?.[choice]
	const gap = topMargin(probabilities)

	const closeCall = !known || (gap !== undefined && gap < ROUTE_MARGIN)
	const lowConfidence = confidence !== undefined && confidence < ROUTE_MIN_CONFIDENCE

	return {
		choice,
		agent: option?.agent,
		writes: option?.writes ?? false,
		probabilities,
		confidence,
		closeCall: closeCall || lowConfidence,
		reasonCodes: deriveReasonCodes(input, closeCall, lowConfidence, "jev"),
		source: "jev",
	}
}
