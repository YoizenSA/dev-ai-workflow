/**
 * One Jev call per judgment (PLAN 4.1 / 5).
 *
 * Screen is one request per file carrying all five questions; locate is four
 * separate requests per signal, because jev-review split dependent steps on
 * purpose and fusing them is how a "which hunk" answer starts contaminating a
 * severity answer.
 */
import { SCREEN_THRESHOLD, LOCATABLE_DIMENSIONS, DIMENSIONS } from "../domain/config"
import type { Dimension, Hunk, Signal } from "../domain/types"
import { noulOf, type Answer, type JevClient } from "../jev/client"
import {
	hunkQuestion,
	locateState,
	mechanismQuestion,
	ownerQuestion,
	screenQuestions,
	screenState,
	severityQuestion,
} from "../jev/questions"

export interface Usage {
	inputTokens: number
	outputTokens: number
	requests: number
}

export function emptyUsage(): Usage {
	return { inputTokens: 0, outputTokens: 0, requests: 0 }
}

export function addUsage(into: Usage, from: { input_tokens?: number; output_tokens?: number } | undefined) {
	into.requests++
	into.inputTokens += from?.input_tokens ?? 0
	into.outputTokens += from?.output_tokens ?? 0
}

/** Screen one file: five probabilities, one request. */
export async function screenFile(
	client: JevClient,
	path: string,
	patch: string,
	usage: Usage,
): Promise<Partial<Record<Dimension, number>>> {
	const response = await client.systemOne({
		state: screenState(path, patch),
		questions: screenQuestions(),
	})
	addUsage(usage, response.usage)

	const scores: Partial<Record<Dimension, number>> = {}
	for (const dimension of DIMENSIONS) {
		const value = noulOf(response.answers ?? {}, dimension)
		if (value !== undefined) scores[dimension] = value
	}
	return scores
}

/**
 * Rank screen results into follow-ups.
 *
 * Highest probability first; ties break by DIMENSIONS order, which puts
 * security ahead of testGap. Only LOCATABLE_DIMENSIONS can open a follow-up -
 * see the compatibility note in config.
 */
export function rankSignals(
	matrix: Record<string, Partial<Record<Dimension, number>>>,
	limit: number,
	threshold = SCREEN_THRESHOLD,
): Signal[] {
	const signals: Signal[] = []
	for (const [file, scores] of Object.entries(matrix)) {
		for (const dimension of LOCATABLE_DIMENSIONS) {
			const probability = scores[dimension]
			if (probability !== undefined && probability >= threshold) {
				signals.push({ file, dimension, probability })
			}
		}
	}
	signals.sort((a, b) => {
		if (b.probability !== a.probability) return b.probability - a.probability
		const byDimension =
			DIMENSIONS.indexOf(a.dimension) - DIMENSIONS.indexOf(b.dimension)
		if (byDimension !== 0) return byDimension
		return a.file.localeCompare(b.file)
	})
	return signals.slice(0, limit)
}

export interface Located {
	hunk: Hunk
	mechanism: string
	severity: number
	owner?: string
}

function choiceOf(answers: Record<string, Answer>, name: string): string | undefined {
	const value = answers[name]?.choice
	return typeof value === "string" ? value : undefined
}

function scoreOf(answers: Record<string, Answer>, name: string): number | undefined {
	const value = answers[name]?.score
	return typeof value === "number" ? value : undefined
}

/**
 * Locate one signal: hunk, then mechanism, then severity, then owner.
 *
 * Returns undefined when Jev answers `noMatch` - a signal we cannot place is
 * not a finding, and inventing a line number would be exactly the "presentar
 * como finding de Jev algo que Jev no marco" the plan forbids.
 */
export async function locateSignal(
	client: JevClient,
	signal: Signal,
	hunks: Hunk[],
	usage: Usage,
	ownerSeverity: number,
): Promise<Located | undefined> {
	if (hunks.length === 0) return undefined

	const hunkResponse = await client.systemOne({
		state: { file: { path: signal.file, patch: hunks.map((h) => h.text).join("\n") } },
		questions: { hunk: hunkQuestion(signal.dimension, hunks) },
	})
	addUsage(usage, hunkResponse.usage)

	const label = choiceOf(hunkResponse.answers ?? {}, "hunk")
	if (!label || label === "noMatch") return undefined
	const index = Number(/^hunk_(\d+)$/.exec(label)?.[1])
	const hunk = Number.isInteger(index) ? hunks[index] : undefined
	if (!hunk) return undefined

	const state = locateState(signal.file, hunk)

	const mechanismResponse = await client.systemOne({
		state,
		questions: { mechanism: mechanismQuestion(signal.dimension) },
	})
	addUsage(usage, mechanismResponse.usage)
	const mechanism = choiceOf(mechanismResponse.answers ?? {}, "mechanism")
	if (!mechanism) return undefined

	const severityResponse = await client.systemOne({
		state,
		questions: { severity: severityQuestion(signal.dimension) },
	})
	addUsage(usage, severityResponse.usage)
	const severity = scoreOf(severityResponse.answers ?? {}, "severity") ?? 0

	let owner: string | undefined
	if (severity >= ownerSeverity) {
		const ownerResponse = await client.systemOne({
			state,
			questions: { owner: ownerQuestion() },
		})
		addUsage(usage, ownerResponse.usage)
		owner = choiceOf(ownerResponse.answers ?? {}, "owner")
	}

	return { hunk, mechanism, severity, owner }
}
