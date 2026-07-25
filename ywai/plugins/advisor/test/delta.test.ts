import { describe, expect, test } from "bun:test"
import { ADVISORY_MARKER, SessionCursor, isAdvisoryMessage, renderDelta, worthReviewing } from "../src/delta"

const msg = (id: string, role: string, text: string) => ({ id, role, text })

describe("SessionCursor", () => {
  test("returns only what has not been reviewed", () => {
    const cursor = new SessionCursor()
    const history = [msg("1", "user", "add auth"), msg("2", "assistant", "on it")]

    expect(cursor.take(history).messages).toHaveLength(2)

    history.push(msg("3", "assistant", "done"))
    const second = cursor.take(history)
    expect(second.messages).toHaveLength(1)
    expect(second.messages[0]?.id).toBe("3")
  })

  test("an idle turn with nothing new yields an empty delta", () => {
    const cursor = new SessionCursor()
    const history = [msg("1", "user", "hi")]
    cursor.take(history)
    expect(cursor.take(history).messages).toHaveLength(0)
  })

  test("seeding skips history that predates enabling the advisor", () => {
    // Turning the advisor on mid-session must not replay the whole
    // conversation — that is one very large, very pointless first call.
    const history = [msg("1", "user", "old"), msg("2", "assistant", "old reply")]
    const cursor = new SessionCursor(history.length)

    expect(cursor.take(history).messages).toHaveLength(0)

    history.push(msg("3", "user", "new request"))
    expect(cursor.take(history).messages.map((m) => m.id)).toEqual(["3"])
  })
})

describe("own notes are never reviewed", () => {
  test("advisory messages are filtered out of the delta", () => {
    const cursor = new SessionCursor()
    const delta = cursor.take([
      msg("1", "assistant", "refactored the parser"),
      { id: "2", role: "user", text: `${ADVISORY_MARKER} severity="concern">watch the retry loop</advisory>` },
      msg("3", "assistant", "continuing"),
    ])

    expect(delta.messages.map((m) => m.id)).toEqual(["1", "3"])
  })

  test("recognized by marker or by flag", () => {
    expect(isAdvisoryMessage({ text: `${ADVISORY_MARKER} severity="nit">x</advisory>` })).toBe(true)
    expect(isAdvisoryMessage({ advisory: true, text: "anything" })).toBe(true)
    expect(isAdvisoryMessage({ role: "assistant", text: "normal work" })).toBe(false)
  })
})

describe("rewritten transcripts", () => {
  test("compaction rewinds the cursor and reports it", () => {
    const cursor = new SessionCursor()
    cursor.take([msg("1", "user", "a"), msg("2", "assistant", "b"), msg("3", "assistant", "c")])

    // Compaction replaced three messages with one summary.
    const after = cursor.take([msg("s1", "assistant", "summary of earlier work")])
    expect(after.reset).toBe(true)
    expect(after.messages.map((m) => m.id)).toEqual(["s1"])
  })

  test("a fork that keeps the length but changes history is still detected", () => {
    const cursor = new SessionCursor()
    cursor.take([msg("1", "user", "a"), msg("2", "assistant", "b")])

    const forked = cursor.take([msg("1", "user", "a"), msg("2b", "assistant", "different reply")])
    expect(forked.reset).toBe(true)
  })

  test("normal growth is not mistaken for a rewrite", () => {
    const cursor = new SessionCursor()
    const history = [msg("1", "user", "a")]
    cursor.take(history)
    history.push(msg("2", "assistant", "b"))
    expect(cursor.take(history).reset).toBe(false)
  })
})

describe("renderDelta", () => {
  test("labels who said what", () => {
    const out = renderDelta([msg("1", "user", "make it faster"), msg("2", "assistant", "cached the lookup")])
    expect(out).toContain("## user")
    expect(out).toContain("make it faster")
    expect(out).toContain("## assistant")
  })

  test("drops empty messages instead of emitting bare headings", () => {
    expect(renderDelta([msg("1", "assistant", "   ")])).toBe("")
  })
})

describe("worthReviewing", () => {
  test("an empty delta is not worth a model call", () => {
    expect(worthReviewing([])).toBe(false)
    expect(worthReviewing([msg("1", "assistant", "  ")])).toBe(false)
  })

  test("real content is", () => {
    expect(worthReviewing([msg("1", "assistant", "rewrote the scheduler")])).toBe(true)
  })
})
