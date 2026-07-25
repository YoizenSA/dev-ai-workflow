import { describe, expect, test } from "bun:test"
import { EmissionGuard, normalizeNote } from "../src/emission-guard"

// The guard is what stands between "a second model watches every turn" and a
// transcript buried in "Stop." — oh-my-pi recorded 309 notes covering 92 unique
// texts in one session. Every case here is a way that flood gets in.

describe("normalizeNote", () => {
  test("folds punctuation and casing to one key", () => {
    const key = normalizeNote("stop")
    for (const variant of ["Stop.", "*Stop*", "  STOP  ", "stop!!!", "**stop**"]) {
      expect(normalizeNote(variant)).toBe(key)
    }
  })

  test("keeps distinct notes distinct", () => {
    expect(normalizeNote("missing await on end()")).not.toBe(normalizeNote("missing await on start()"))
  })

  test("collapses runs of separators rather than dropping words", () => {
    expect(normalizeNote("no  issue;   continue.")).toBe("no issue continue")
  })
})

describe("content-free notes", () => {
  test("filler is suppressed", () => {
    const guard = new EmissionGuard()
    for (const filler of ["Stop.", "Done", "No issue; continue.", "LGTM", "nothing to add", "OK"]) {
      guard.beginUpdate()
      const v = guard.admit(filler, "blocker")
      expect(v.accepted).toBe(false)
      if (!v.accepted) expect(v.reason).toBe("content-free")
    }
  })

  test("a real note that merely starts with a filler word survives", () => {
    const guard = new EmissionGuard()
    guard.beginUpdate()
    const v = guard.admit("Stop: the await is missing on end(), buffered writes are lost.", "blocker")
    expect(v.accepted).toBe(true)
  })

  test("empty and whitespace-only notes are rejected", () => {
    const guard = new EmissionGuard()
    guard.beginUpdate()
    expect(guard.admit("").accepted).toBe(false)
    guard.beginUpdate()
    expect(guard.admit("   \n  ").accepted).toBe(false)
    guard.beginUpdate()
    // Punctuation with no letters or digits normalizes to nothing.
    expect(guard.admit("...!!!").accepted).toBe(false)
  })
})

describe("dedupe", () => {
  test("the same advice never lands twice, however it is dressed", () => {
    const guard = new EmissionGuard()
    guard.beginUpdate()
    expect(guard.admit("The retry loop has no backoff.").accepted).toBe(true)

    for (const repeat of ["the retry loop has no backoff", "The Retry Loop Has No Backoff!", "  the retry loop has no backoff.  "]) {
      guard.beginUpdate()
      const v = guard.admit(repeat)
      expect(v.accepted).toBe(false)
      if (!v.accepted) expect(v.reason).toBe("duplicate")
    }
  })

  test("reset lets a re-primed advisor raise the issue again", () => {
    const guard = new EmissionGuard()
    guard.beginUpdate()
    guard.admit("The retry loop has no backoff.")

    guard.reset()
    guard.beginUpdate()
    expect(guard.admit("The retry loop has no backoff.").accepted).toBe(true)
  })
})

describe("per-update budget", () => {
  test("only one note lands per advisor run", () => {
    const guard = new EmissionGuard()
    guard.beginUpdate()
    expect(guard.admit("First real concern.").accepted).toBe(true)

    const second = guard.admit("Second, also real, concern.")
    expect(second.accepted).toBe(false)
    if (!second.accepted) expect(second.reason).toBe("rate-limited")
  })

  test("suppressed noise does not spend the budget", () => {
    // The failure this prevents: the model opens with "Stop.", the budget is
    // burnt, and the genuine concern that follows in the same update is lost.
    const guard = new EmissionGuard()
    guard.beginUpdate()
    expect(guard.admit("Stop.").accepted).toBe(false)
    expect(guard.admit("Done").accepted).toBe(false)
    expect(guard.admit("The migration runs before the column exists.").accepted).toBe(true)
  })

  test("a new update reopens the budget", () => {
    const guard = new EmissionGuard()
    guard.beginUpdate()
    expect(guard.admit("First.").accepted).toBe(true)
    guard.beginUpdate()
    expect(guard.admit("Second.").accepted).toBe(true)
  })

  test("without beginUpdate the first note still lands", () => {
    // A caller that forgets to open a window must not silently mute the advisor.
    const guard = new EmissionGuard()
    expect(guard.admit("Something real.").accepted).toBe(true)
  })
})

describe("bounded history", () => {
  test("stays bounded over a long session", () => {
    const guard = new EmissionGuard()
    for (let i = 0; i < 5000; i++) {
      guard.beginUpdate()
      guard.admit(`distinct note number ${i}`)
    }
    expect(guard.size).toBeLessThanOrEqual(4096)
  })

  test("recent notes are still deduped after eviction", () => {
    const guard = new EmissionGuard()
    for (let i = 0; i < 5000; i++) {
      guard.beginUpdate()
      guard.admit(`distinct note number ${i}`)
    }
    guard.beginUpdate()
    expect(guard.admit("distinct note number 4999").accepted).toBe(false)
  })
})

describe("severity", () => {
  test("rides through on an accepted note", () => {
    const guard = new EmissionGuard()
    guard.beginUpdate()
    const v = guard.admit("Verification is thinner than the risk taken.", "blocker")
    expect(v.accepted).toBe(true)
    if (v.accepted) expect(v.severity).toBe("blocker")
  })

  test("defaults to nit when unspecified", () => {
    const guard = new EmissionGuard()
    guard.beginUpdate()
    const v = guard.admit("Could be simplified with a single map.")
    if (v.accepted) expect(v.severity).toBe("nit")
  })
})
