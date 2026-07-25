/**
 * Tracks how much of a primary session the advisor has already reviewed.
 *
 * The advisor sees deltas, never the whole transcript. Re-sending everything
 * each turn would cost more every turn and make the advisor re-raise issues the
 * agent already acted on.
 *
 * Two cases need explicit handling, and both are silent corruption if missed:
 *
 *   - **The transcript was rewritten.** Compaction, a session switch, or a fork
 *     replaces history the cursor was pointing into. Continuing from the old
 *     offset would slice at a meaningless boundary.
 *   - **The advisor's own notes.** They land in the primary transcript, so
 *     without filtering the advisor reviews its own advice and comments on it.
 */

/** The subset of an OpenCode message this module needs. */
export type TrackedMessage = {
  id?: string
  role?: string
  text?: string
  /** Tool calls, already rendered one per line. */
  tools?: string
  /** Whether this message changed anything (drives the review gate). */
  mutating?: boolean
  /** Whether any of its tool calls failed. */
  failed?: boolean
  /** Marks a message this plugin injected. */
  advisory?: boolean
}

export type Delta = {
  messages: TrackedMessage[]
  /** True when the cursor was rewound because history changed underneath it. */
  reset: boolean
}

/** Text marker carried by injected notes, used to recognize them on the way back. */
export const ADVISORY_MARKER = "<advisory"

/** Recognizes a message this plugin injected into the primary session. */
export function isAdvisoryMessage(m: TrackedMessage): boolean {
  if (m.advisory) return true
  return typeof m.text === "string" && m.text.includes(ADVISORY_MARKER)
}

/**
 * Per-session review cursor.
 *
 * Seeded at the transcript length that existed when the advisor was enabled, so
 * turning it on mid-conversation reviews what happens next instead of replaying
 * an hour of history in one very expensive first call.
 */
export class SessionCursor {
  #reviewed = 0
  #lastSeenID: string | undefined

  constructor(seedLength = 0) {
    this.#reviewed = seedLength
  }

  get position(): number {
    return this.#reviewed
  }

  /**
   * Returns what the advisor has not seen yet and advances past it.
   *
   * Detects a rewritten transcript by watching for history that shrank or whose
   * anchor message disappeared; either way the cursor rewinds and the caller is
   * told, so it can also clear the guard's dedupe history — advice given about
   * a transcript that no longer exists should be raisable again.
   */
  take(messages: TrackedMessage[]): Delta {
    const rewritten = this.#wasRewritten(messages)
    if (rewritten) this.#reviewed = 0

    const fresh = messages.slice(this.#reviewed)
    this.#reviewed = messages.length
    this.#lastSeenID = messages.length ? messages[messages.length - 1]?.id : undefined

    return { messages: fresh.filter((m) => !isAdvisoryMessage(m)), reset: rewritten }
  }

  /** Rewinds so the next take() replays the current transcript from the start. */
  reset(): void {
    this.#reviewed = 0
    this.#lastSeenID = undefined
  }

  #wasRewritten(messages: TrackedMessage[]): boolean {
    if (messages.length < this.#reviewed) return true
    if (this.#lastSeenID === undefined) return false
    // The message the cursor was anchored to must still be where it was.
    return messages[this.#reviewed - 1]?.id !== this.#lastSeenID
  }
}

/**
 * Renders a delta for the advisor.
 *
 * Roles are labelled because the advisor's job depends on who did what — the
 * same sentence is a request from the user and a claim from the agent.
 */
export function renderDelta(messages: TrackedMessage[]): string {
  return messages
    .map((m) => {
      const role = m.role ?? "unknown"
      const text = (m.text ?? "").trim()
      const tools = (m.tools ?? "").trim()
      if (!text && !tools) return ""
      // Tools come first: what ran frames what the agent then claims about it.
      const body = [tools, text].filter(Boolean).join("\n\n")
      return `## ${role}\n\n${body}`
    })
    .filter(Boolean)
    .join("\n\n")
}

/**
 * Whether a delta has anything in it to review.
 *
 * Only emptiness skips the call. Guessing which turns "deserve" review — no
 * edits, short answer, therefore skip — trades a certain saving for an
 * uncertain miss, and the misses are the cases that matter: an agent talking
 * itself into the wrong approach touches nothing while it does so.
 *
 * Cost is controlled where it can be without losing signal: the delta is small
 * by construction, tool calls render as one line each, and the verdict is a
 * classification rather than prose. Noise is stopped at emission, by the guard,
 * not by refusing to look. (Same division as oh-my-pi, whose advisor reviews
 * every delta and gates only what comes back.)
 */
export function worthReviewing(messages: TrackedMessage[]): boolean {
  return renderDelta(messages).trim().length > 0
}
