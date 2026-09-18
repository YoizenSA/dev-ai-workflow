/**
 * Every Jev question lives here, behind a version (PLAN 5).
 *
 * QUESTIONS_VERSION exists so eval runs stay comparable: change a word in a
 * rubric and the number has to change too, or Fase 6 is comparing two things
 * that were never the same question.
 *
 * The five screen questions are the ones Spike A actually ran (they came from
 * devagrawal09/jev-review, MIT). Locate questions follow PLAN 5 and are not
 * yet validated against the API - Fase 2 is where they first run.
 */
import type { Dimension, Hunk } from "../domain/types"
import type { Question } from "./client"

export const QUESTIONS_VERSION = "2026-09-17.1"
export const PAGE_THEN_VERSION = "page-then-2026-09-17.1"

const noul = (instructions: unknown, criteria?: unknown): Question => ({
	type: "noul",
	instructions,
	criteria,
})

const choice = (instructions: unknown, criteria: Record<string, unknown>): Question => ({
	type: "choice",
	instructions,
	criteria,
})

const score = (instructions: unknown, criteria: unknown[]): Question => ({
	type: "score",
	instructions,
	criteria,
})

const IGNORE = ["Style preferences", "Naming concerns", "Unsupported speculation"]

/**
 * The five screen questions, asked together in one request per file.
 * Verbatim from the Spike A run that produced the numbers in FINDINGS.md.
 */
export function screenQuestions(): Record<Dimension, Question> {
	return {
		correctness: noul(
			{
				question:
					"Does file.patch directly support that this change likely introduces incorrect runtime behavior?",
				inspect: "file.patch",
				focus:
					"Concrete behavior, state, data-flow, or async errors introduced by added or modified lines",
				ignore: IGNORE,
			},
			{
				true: {
					what: "The patch contains a realistic path to a wrong runtime result",
					examples: [
						"A condition now handles the opposite case",
						"A value is written to the wrong field",
					],
				},
				false: {
					what: "The patch is correct, non-behavioral, or lacks direct evidence of a bug",
					examples: ["Formatting only", "A refactor that preserves data flow"],
				},
			},
		),
		security: noul(
			{
				question:
					"Does file.patch directly support that this change introduces or weakens a security boundary?",
				inspect: "file.patch",
				focus:
					"Authorization, injection, secret exposure, trust boundaries, and unsafe defaults",
				ignore: IGNORE,
			},
			{
				true: {
					what: "The patch removes, bypasses, or loosens a protection",
					examples: [
						"An authorization check is deleted",
						"User input reaches a query unescaped",
					],
				},
				false: {
					what: "No security-relevant change, or the change tightens a boundary",
					examples: ["A check is added", "Only test data changed"],
				},
			},
		),
		reliability: noul(
			{
				question:
					"Does file.patch directly support that this change introduces a crash, race, leak, or bad failure recovery?",
				inspect: "file.patch",
				focus: "Unhandled errors, concurrency, resource lifetime, retry and timeout behavior",
				ignore: IGNORE,
			},
			{
				true: {
					what: "The patch creates a realistic failure mode at runtime",
					examples: ["An await is dropped", "A handle is never released"],
				},
				false: {
					what: "No new failure mode, or the change makes failure handling stricter",
					examples: ["An error path is added", "Pure data change"],
				},
			},
		),
		compatibility: noul(
			{
				question:
					"Does file.patch directly support that this change breaks an existing caller, protocol, or data format?",
				inspect: "file.patch",
				focus: "Exported signatures, wire and storage formats, defaults callers depend on",
				ignore: IGNORE,
			},
			{
				true: {
					what: "An existing caller or stored value stops working as written",
					examples: [
						"A required parameter is added to an exported function",
						"A serialized field is renamed",
					],
				},
				false: {
					what: "The change is additive, internal, or preserves the old contract",
					examples: ["A new optional field", "A rename confined to one module"],
				},
			},
		),
		testGap: noul(
			{
				question:
					"Does file.patch change important behavior without a test that targets the new behavior?",
				inspect: "file.patch",
				focus: "Behavior that a targeted test would have caught",
				ignore: IGNORE,
			},
			{
				true: {
					what: "Behavior changed and nothing in the change exercises it",
					examples: ["A new branch with no test", "A bug fix with no regression test"],
				},
				false: {
					what: "The change is non-behavioral, or it carries a test for the new behavior",
					examples: ["Formatting", "A new branch plus a test that covers it"],
				},
			},
		),
	}
}

/**
 * One noul: does the accessibility snapshot support that the Gherkin Then is
 * observed? Jev does not click. The snapshot is data in `state`.
 */
export function thenQuestion(): Question {
	return noul(
		{
			question: "Does page.snapshot directly support that page.then is observed on the page?",
			inspect: "page.snapshot",
			compare: ["page.snapshot", "page.then"],
			focus: "Only what the snapshot shows, not what the page might do next",
			ignore: IGNORE,
		},
		{
			true: {
				what: "The snapshot contains direct evidence of the Then",
				examples: [
					"Then names a New workflow action and a button with that name is in the snapshot",
					"Then names an error and an alert with that text is in the snapshot",
				],
			},
			false: {
				what: "The snapshot does not show the Then, or only mentions it in a comment",
				examples: [
					"Then requires a list of workflows and the snapshot is an error alert",
					"The Then text appears only inside an HTML comment",
				],
			},
		},
	)
}

export function thenState(snapshot: string, then: string, url?: string, title?: string): unknown {
	return { page: { snapshot, then, ...(url ? { url } : {}), ...(title ? { title } : {}) } }
}

/** State for one screen request. Code always travels inside `state` (PLAN 5). */
export function screenState(path: string, patch: string): unknown {
	return { file: { path, patch } }
}

/**
 * Locate step 1: which hunk, with an explicit escape.
 *
 * `noMatch` is a real label, not a fallback we invent afterwards: without it
 * Choice has to pick some hunk, and a forced pick reads as a finding.
 */
export function hunkQuestion(dimension: Dimension, hunks: Hunk[]): Question {
	const criteria: Record<string, unknown> = {}
	hunks.forEach((hunk, index) => {
		criteria[`hunk_${index}`] = {
			what: `Lines ${hunk.startLine}-${hunk.endLine} of the new file`,
		}
	})
	criteria.noMatch = {
		what: "No hunk in this patch carries the problem",
		examples: ["The signal came from context, not from a changed line"],
	}
	return choice(
		{
			question: `Which hunk of file.patch carries the ${dimension} problem?`,
			inspect: "file.patch",
			focus: "Only the changed lines, not the surrounding context",
		},
		criteria,
	)
}

/** Locate step 2: how it breaks. Asked on its own - never fused with step 1. */
export function mechanismQuestion(dimension: Dimension): Question {
	const byDimension: Record<Dimension, Record<string, unknown>> = {
		correctness: {
			inverted_condition: { what: "A condition now selects the opposite branch" },
			wrong_value: { what: "A wrong value or field is read or written" },
			missing_case: { what: "A case that used to be handled no longer is" },
			bad_async: { what: "Ordering or awaiting of async work is wrong" },
		},
		security: {
			missing_authz: { what: "An authorization or role check is gone or bypassed" },
			injection: { what: "Untrusted input reaches an interpreter unescaped" },
			secret_exposure: { what: "A secret is logged, returned, or stored in the clear" },
			unsafe_default: { what: "A default now permits what it used to deny" },
		},
		reliability: {
			unhandled_error: { what: "An error path is dropped or swallowed" },
			race: { what: "Concurrent access without the ordering it needs" },
			leak: { what: "A resource is acquired and never released" },
			bad_recovery: { what: "Retry, timeout, or fallback behavior is now wrong" },
		},
		compatibility: {
			signature_change: { what: "An exported signature changed on callers" },
			format_change: { what: "A wire or storage format changed" },
			default_change: { what: "A default callers depend on changed" },
		},
		testGap: {
			untested_branch: { what: "A new branch nothing exercises" },
			untested_fix: { what: "A bug fix with no regression test" },
			removed_coverage: { what: "A test that covered this behavior was removed" },
		},
	}
	return choice(
		{
			question: `How does the change in file.hunk break ${dimension}?`,
			inspect: "file.hunk",
		},
		byDimension[dimension],
	)
}

/** Locate step 3: severity, 0-3. Thresholds in config decide what it means. */
export function severityQuestion(dimension: Dimension): Question {
	return score(
		{
			question: `How severe is this ${dimension} problem in production?`,
			inspect: "file.hunk",
		},
		[
			{ what: "Not a real problem" },
			{ what: "Minor: annoying, contained, easy to notice" },
			{ what: "Serious: wrong results or a failure users hit" },
			{ what: "Critical: data loss, a breached boundary, or a broken contract" },
		],
	)
}

/** Locate step 4: only asked at or over OWNER_SEVERITY. */
export function ownerQuestion(): Question {
	return choice(
		{ question: "Who should own fixing this?", inspect: "file.hunk" },
		{
			security: { what: "Owns boundaries, authz, secrets" },
			api: { what: "Owns exported contracts and formats" },
			runtime: { what: "Owns behavior, concurrency, resources" },
			testing: { what: "Owns coverage for the changed behavior" },
			maintainer: { what: "No specialist needed" },
		},
	)
}

/** State for every locate step: the file, the hunk, and nothing else. */
export function locateState(path: string, hunk: Hunk): unknown {
	return { file: { path, hunk: hunk.text, startLine: hunk.startLine, endLine: hunk.endLine } }
}
