/**
 * Policy gate for advisor notes, applied per session.
 *
 * The advisor's system prompt tells the model to send at most one note per
 * update and never to repeat itself. Advisor models do not reliably obey that.
 * oh-my-pi documented a session where the advisor emitted 309 notes covering 92
 * unique texts — 114 of them literally "Stop." — burying the primary transcript
 * after the task was already finished.
 *
 * So the rules live here, in code, rather than only in the prompt: a rule the
 * model can ignore is not a rule.
 *
 * Suppression is deliberately invisible to the advisor — a dropped note still
 * reports success. Telling the model it was suppressed invites it to rephrase
 * the same empty note until something slips past the dedupe ("Stop." → "Halt."
 * → "Stop now.").
 */

/** Severity ordering used for delivery decisions. */
export type Severity = "nit" | "concern" | "blocker"

export type GuardVerdict =
  | { accepted: true; note: string; severity: Severity }
  | { accepted: false; reason: "empty" | "content-free" | "duplicate" | "rate-limited" }

/**
 * Case-insensitive, punctuation-folded normalization: every run of non-letter,
 * non-digit characters collapses to one space.
 *
 * This is what makes the dedupe hold — "Stop.", "*Stop*" and "  STOP  " all key
 * to `stop`, so a model cycling through cosmetic variants of the same empty
 * note gets caught by the first one.
 */
export function normalizeNote(note: string): string {
  return note
    .toLowerCase()
    .normalize("NFKC")
    .replace(/[^\p{L}\p{N}]+/gu, " ")
    .trim()
}

/**
 * Notes that carry no actionable content. Each entry is already normalized, so
 * one lookup covers every casing and punctuation variant.
 *
 * The list stays short on purpose: it only holds filler observed to flood a
 * transcript. A real note that happens to start with "stop" — "Stop: the await
 * is missing on end(), buffered writes are lost" — normalizes to something
 * longer and is not matched.
 */
const CONTENT_FREE_PHRASES: ReadonlySet<string> = new Set([
  // Telling the agent to stop without saying why helps nobody.
  "stop",
  "stop here",
  "stop now",
  "halt",
  "abort",
  // Completion self-talk: the agent knows it finished.
  "done",
  "complete",
  "completed",
  "task done",
  "task complete",
  "finished",
  "ok",
  "okay",
  "ok done",
  // "Nothing to flag" — silence already says this, and says it for free.
  "no issue",
  "no issues",
  "no issue continue",
  "no issues continue",
  "no concerns",
  "nothing to add",
  "nothing to flag",
  "no further input",
  "lgtm",
  "looks good",
  "looks good to me",
  "continue",
  "proceed",
  "agreed",
])

/** Bound on remembered notes, so a long session cannot grow the guard forever. */
const DEDUPE_CAPACITY = 4096

/**
 * Per-session note gate. One instance per primary session; reset it whenever
 * the transcript is rewritten, since advice already given no longer applies to
 * a history the agent can no longer see.
 */
export class EmissionGuard {
  #seen = new Set<string>()
  #order: string[] = []
  #spentThisUpdate = false

  /**
   * Opens a new budget window. Called once before each advisor model run, so a
   * note suppressed as noise does not consume the slot a real one needs.
   */
  beginUpdate(): void {
    this.#spentThisUpdate = false
  }

  /** Clears history and budget — used when the primary transcript is rewritten. */
  reset(): void {
    this.#seen.clear()
    this.#order = []
    this.#spentThisUpdate = false
  }

  /** Number of distinct notes remembered. Exposed for `/advisor status`. */
  get size(): number {
    return this.#seen.size
  }

  /**
   * Decides whether a note reaches the primary session.
   *
   * Order matters: cheap rejections run before the rate limit so that noise
   * never spends the budget. A model that emits "Stop." and then a real concern
   * in the same update still gets the real one through.
   */
  admit(rawNote: string, severity: Severity = "nit"): GuardVerdict {
    const note = rawNote.trim()
    if (!note) return { accepted: false, reason: "empty" }

    const key = normalizeNote(note)
    if (!key) return { accepted: false, reason: "empty" }

    if (CONTENT_FREE_PHRASES.has(key)) {
      return { accepted: false, reason: "content-free" }
    }

    if (this.#seen.has(key)) {
      return { accepted: false, reason: "duplicate" }
    }

    if (this.#spentThisUpdate) {
      return { accepted: false, reason: "rate-limited" }
    }

    this.#remember(key)
    this.#spentThisUpdate = true
    return { accepted: true, note, severity }
  }

  #remember(key: string): void {
    this.#seen.add(key)
    this.#order.push(key)
    if (this.#order.length > DEDUPE_CAPACITY) {
      const evicted = this.#order.shift()
      if (evicted !== undefined) this.#seen.delete(evicted)
    }
  }
}
