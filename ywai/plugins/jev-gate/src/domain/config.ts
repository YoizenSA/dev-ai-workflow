/**
 * Thresholds and policy constants (PLAN 3.3).
 *
 * Values marked PROVISIONAL are guesses until the Fase 6 eval calibrates them;
 * every other value comes from jev-review or tsg.
 */
import type { Dimension } from "./types"

export const SCREEN_THRESHOLD = 0.7
export const MIN_LOCATION_CONFIDENCE = 0.55
export const OWNER_SEVERITY = 1.5
export const BLOCKING_SEVERITY = 2.0
export const MAX_FOLLOW_UPS = 8
export const MAX_PROFILES = 5
/** Requests in flight, not questions - Jev parallelizes questions itself. */
export const CONCURRENCY = 3
export const JEV_TIMEOUT_MS = 5000
/** Over this many files, jev_review_path asks before spending requests. */
export const CONFIRM_FILES_CODEBASE = 30
/** Hard stop, confirmation or not. */
export const MAX_FILES_CODEBASE = 80

/** Report order, and the tie-break order when two signals share a probability. */
export const DIMENSIONS: Dimension[] = [
	"security",
	"correctness",
	"reliability",
	"compatibility",
	"testGap",
]

/**
 * Dimensions allowed to open a follow-up.
 *
 * `compatibility` is screened and reported but never promoted on its own:
 * Spike A (2026-09-17) scored it 0.77-0.86 on all four fixtures, highest of all
 * on the *clean* rename, so at SCREEN_THRESHOLD it would open a follow-up on
 * essentially every patch. Fase 6 either rewrites its question or drops it.
 */
export const LOCATABLE_DIMENSIONS: Dimension[] = [
	"security",
	"correctness",
	"reliability",
	"testGap",
]

/** Never sent to Jev, whatever the diff says (PLAN 3.2). */
export const DENY_GLOBS = [
	"**/.env*",
	"**/*.pem",
	"**/*.key",
	"**/secrets/**",
	// Our own key file. A live Fase 2 run put `.opencode/jev-gate.json` in the
	// diff: it was skipped for not being JS/TS, which is luck, not policy.
	"**/jev-gate.json",
]

/** v1 reviews JS/TS only; py/go land in v1.5 (PLAN 4.1). */
export const REVIEWABLE_EXTENSIONS = [".ts", ".tsx", ".js", ".jsx", ".mts", ".cts"]

/**
 * Minimal glob matcher for DENY_GLOBS: `**` spans separators, `*` does not.
 * Deliberately tiny - a deny list is not the place for a glob dependency.
 */
export function matchesGlob(path: string, glob: string): boolean {
	const normalized = path.replace(/\\/g, "/")
	let pattern = ""
	for (let i = 0; i < glob.length; i++) {
		const char = glob[i]
		if (char === "*") {
			if (glob[i + 1] === "*") {
				pattern += ".*"
				i++
				// `**/` must also match zero directories: `**/x.ts` covers `x.ts`.
				if (glob[i + 1] === "/") i++
			} else {
				pattern += "[^/]*"
			}
			continue
		}
		pattern += char.replace(/[.+^${}()|[\]\\?]/g, "\\$&")
	}
	return new RegExp(`^${pattern}$`).test(normalized)
}

export function isDenied(path: string, globs: string[] = DENY_GLOBS): boolean {
	return globs.some((glob) => matchesGlob(path, glob))
}

export function isReviewable(path: string): boolean {
	return REVIEWABLE_EXTENSIONS.some((ext) => path.toLowerCase().endsWith(ext))
}

/** Tests are context for the review, never its target (PLAN 3.2). */
export function isTestFile(path: string): boolean {
	return /(^|\/)(__tests__|tests?)\/|\.(test|spec)\.[cm]?[jt]sx?$/.test(
		path.replace(/\\/g, "/"),
	)
}
