import { describe, expect, test } from "bun:test"
import { ADVISOR_SYSTEM_PROMPT, buildAdvisorPrompt, parseVerdict, renderAdvisory } from "../src/verdict"

describe("parseVerdict", () => {
  test("reads a real note", () => {
    const v = parseVerdict("SEVERITY: concern\nNOTE: The migration runs before the column exists.")
    expect(v.severity).toBe("concern")
    if (v.severity !== "silent") expect(v.note).toContain("column exists")
  })

  test("silence produces no note", () => {
    expect(parseVerdict("SEVERITY: silent").severity).toBe("silent")
  })

  test("tolerates casing and spacing", () => {
    expect(parseVerdict("severity:BLOCKER\nnote:  ships broken output  ").severity).toBe("blocker")
  })

  test("keeps a multi-sentence note whole", () => {
    const v = parseVerdict("SEVERITY: nit\nNOTE: First sentence. Second sentence.")
    if (v.severity !== "silent") expect(v.note).toBe("First sentence. Second sentence.")
  })

  // Anything the reviewer garbles must fall to silence: a model that could not
  // follow a two-line format has not earned an interruption, and guessing at
  // what it meant is exactly how noise gets in.
  test.each([
    ["prose with no verdict", "I think the agent is doing fine here, maybe watch the retries."],
    ["severity declared but note empty", "SEVERITY: concern\nNOTE:   "],
    ["note with no severity", "NOTE: something is wrong"],
    ["unknown severity word", "SEVERITY: warning\nNOTE: something"],
    ["empty reply", ""],
  ])("falls back to silence: %s", (_label, reply) => {
    expect(parseVerdict(reply).severity).toBe("silent")
  })

  test("a declared silence with a stray note stays silent", () => {
    // The severity is the decision; a note beside it does not override it.
    expect(parseVerdict("SEVERITY: silent\nNOTE: nothing really").severity).toBe("silent")
  })
})

describe("renderAdvisory", () => {
  test("carries severity and the do-not-obey framing", () => {
    const out = renderAdvisory("blocker", "The retry loop never backs off.")
    expect(out).toContain('severity="blocker"')
    expect(out).toContain("do not blindly obey")
    expect(out).toContain("The retry loop never backs off.")
  })

  test("escapes markup so a note cannot break out of the wrapper", () => {
    const out = renderAdvisory("nit", 'Use <div class="x"> & check a > b')
    expect(out).toContain("&lt;div")
    expect(out).toContain("&amp;")
    expect(out).toContain("&gt;")
    // The wrapper itself must remain the only real tag.
    expect(out.match(/<advisory/g)).toHaveLength(1)
    expect(out).toContain("</advisory>")
  })
})

describe("buildAdvisorPrompt", () => {
  test("carries the delta", () => {
    expect(buildAdvisorPrompt("## assistant\n\nrewrote the parser")).toContain("rewrote the parser")
  })

  test("project priorities are wrapped so they read as guidance, not transcript", () => {
    const out = buildAdvisorPrompt("delta", "Watch for writes that bypass the queue.")
    expect(out).toContain("<attention>")
    expect(out).toContain("bypass the queue")
  })

  test("no attention block when the project has no watchdog file", () => {
    expect(buildAdvisorPrompt("delta", "   ")).not.toContain("<attention>")
    expect(buildAdvisorPrompt("delta")).not.toContain("<attention>")
  })
})

describe("system prompt", () => {
  test("states the format it will be parsed against", () => {
    expect(ADVISOR_SYSTEM_PROMPT).toContain("SEVERITY:")
    expect(ADVISOR_SYSTEM_PROMPT).toContain("NOTE:")
  })

  test("makes silence the default rather than an option", () => {
    expect(ADVISOR_SYSTEM_PROMPT.toLowerCase()).toContain("silence is your normal answer")
  })
})
