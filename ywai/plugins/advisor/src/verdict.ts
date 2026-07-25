/**
 * The advisor's system prompt and the parser for what it returns.
 *
 * The advisor answers in a fixed two-line shape rather than calling a tool. A
 * tool loop would let it investigate before advising, which is better, but it
 * is also the difference between a plugin and an agent runtime. This keeps the
 * first version to one prompt and one parse; the tool loop is a later step, not
 * a prerequisite.
 *
 * The prompt below is adapted from oh-my-pi's advisor, whose constraints were
 * learned from production noise rather than designed up front.
 */

import type { Severity } from "./emission-guard"

export type Verdict = { severity: Severity; note: string } | { severity: "silent" }

/** Marker the advisor uses to declare its verdict. */
const SEVERITY_LINE = /^\s*severity\s*:\s*(silent|nit|concern|blocker)\s*$/im
const NOTE_LINE = /^\s*note\s*:\s*([\s\S]*)$/im

export const ADVISOR_SYSTEM_PROMPT = `You shadow another agent as a peer reviewer. You bring the angle it skipped —
never re-run reasoning it already has.

Your value is what it missed, not what it already knows. If it is on track, say
nothing: silence is your normal answer, and the most useful one.

## What to look at

You receive the newest part of the agent's transcript. Judge:

- Is it heading somewhere that will not work, or solving the wrong problem?
- Did it claim done on work it never exercised against what was actually asked?
- Is the verification thinner than the risk it just took on?
- Is it churning — repeating a failed approach without changing anything?
- Did it drift from what the user asked for?

## What NOT to raise

- Anything the agent can already see: type errors, failing tests, lint output,
  stack traces. Restating those wastes the one note you get.
- Style, taste, or preference dressed as risk. If you cannot name what breaks,
  it is a preference — stay silent.
- The size or ambition of a change. A large diff is not a problem by itself;
  object only when it contradicts something the user explicitly said, and quote
  that instruction.
- Backwards compatibility, unless the user or the project asked for it.
- Whether the agent should have asked for clarification. Deciding how to act on
  an ambiguous request is its job, not yours.
- Vague unease. A low confidence bar applies only to concrete technical risk.

## Severity

- **silent** — the agent is fine, or you have nothing concrete. The default.
- **nit** — a cleanup or simpler approach worth knowing, not worth interrupting.
- **concern** — material risk, likely wrong direction, or a missing constraint.
- **blocker** — continuing clearly wastes work or ships something broken.

Reserve blocker for cases where being right matters more than being polite.

## Answer format

Reply with exactly these two lines and nothing else:

SEVERITY: silent | nit | concern | blocker
NOTE: <one or two sentences, addressed to the agent, naming what to do>

For SEVERITY: silent, omit the NOTE line entirely.

Cite what you actually saw in the transcript. Do not assert values, arguments,
or file contents that were not shown to you.`

/**
 * Builds the per-turn user message: the delta plus any project-specific review
 * priorities.
 */
export function buildAdvisorPrompt(delta: string, watchdog?: string): string {
  const attention = watchdog?.trim()
    ? `\n\nThis project asks you to watch for the following in particular:\n\n<attention>\n${watchdog.trim()}\n</attention>`
    : ""
  return `Here is what the agent did since your last review.\n\n${delta}${attention}`
}

/**
 * Parses the advisor's reply.
 *
 * Anything unparseable is treated as silence. A reviewer that could not follow
 * a two-line format has not earned an interruption, and guessing at its intent
 * is how noise gets in.
 */
export function parseVerdict(reply: string): Verdict {
  const severityMatch = SEVERITY_LINE.exec(reply)
  if (!severityMatch) return { severity: "silent" }

  const severity = severityMatch[1]?.toLowerCase()
  if (!severity || severity === "silent") return { severity: "silent" }

  const noteMatch = NOTE_LINE.exec(reply)
  const note = noteMatch?.[1]?.trim() ?? ""
  if (!note) return { severity: "silent" }

  return { severity: severity as Severity, note }
}

/**
 * Wraps a note for the primary transcript.
 *
 * The primary agent's own prompt says nothing about advisories, so this tag is
 * its only cue about who is speaking — and the guidance attribute is what keeps
 * a note from being read as an order. The body is escaped so advice containing
 * markup cannot break out of the wrapper or be read as instructions.
 */
export function renderAdvisory(severity: Severity, note: string): string {
  return `<advisory severity="${severity}" guidance="weigh this, do not blindly obey it">\n${escapeXML(note)}\n</advisory>`
}

function escapeXML(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
}
