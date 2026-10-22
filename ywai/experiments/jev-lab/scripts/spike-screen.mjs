/**
 * Fase 0 spike A — screen one fixture patch against Jev.
 *
 * This is the extraction PLAN.md §15 asked for: the screen questions and call
 * shape come from devagrawal09/jev-review src/review/judgments.ts (MIT), not
 * from memory. It answers PLAN §6.7.6: the SDK is @typesafe-ai/sdk; a screen
 * call is client.systemOne({ state, questions }); probabilities come back as
 * response.answers[dimension].noul.
 *
 * Usage:
 *   bun install                                                  (once)
 *   TYPESAFE_API_KEY=... bun scripts/spike-screen.mjs fixtures/02-removed-auth-check.patch
 */
import { noul, TypeSafeClient } from "@typesafe-ai/sdk"
import { readFileSync } from "node:fs"

const PATCH_PATH = process.argv[2]
if (!PATCH_PATH) {
  console.error("usage: bun scripts/spike-screen.mjs <fixture.patch>")
  process.exit(1)
}

// TypeSafeClient() with no args reads TYPESAFE_API_KEY from the env (jev-review
// .env.example). Without a key, review refuses to run — fail closed (PLAN §3.1).
const client = new TypeSafeClient()

// ChangedFile = { path, patch } (jev-review src/domain/types.ts). Fixtures are
// single-file diffs, so changedTests is empty and testGap scores against nothing.
const patch = readFileSync(PATCH_PATH, "utf8")
const file = {
  path: PATCH_PATH.replace(/\\/g, "/").replace(/^fixtures\//, "src/").replace(/\.patch$/, ".ts"),
  patch,
}

// The five screen questions, verbatim from jev-review src/review/judgments.ts.
const questions = {
  correctness: noul(
    {
      question: "Does file.patch directly support that this change likely introduces incorrect runtime behavior?",
      inspect: "file.patch",
      focus: "Concrete behavior, state, data-flow, or async errors introduced by added or modified lines",
      ignore: ["Style preferences", "Naming concerns", "Unsupported speculation"],
    },
    {
      true: {
        what: "The patch contains a realistic path to a wrong runtime result",
        examples: ["A condition now handles the opposite case", "A value is written to the wrong field"],
      },
      false: {
        what: "The patch is correct, non-behavioral, or lacks direct evidence of a bug",
        examples: ["Formatting only", "A refactor that preserves data flow"],
      },
    },
  ),
  security: noul(
    {
      question: "Does file.patch directly support that this change introduces or weakens a security boundary?",
      inspect: "file.patch",
      focus: "Authorization, injection, secret exposure, trust boundaries, and unsafe defaults",
    },
    {
      true: {
        what: "The patch creates a concrete path around a security control or into an unsafe sink",
        examples: ["An authorization check is removed", "Untrusted input reaches command execution"],
      },
      false: {
        what: "No security boundary is weakened by the patch",
        not_for: "Code that merely uses security-related names",
      },
    },
  ),
  reliability: noul(
    {
      question: "Does file.patch directly support that this change can crash, race, leak, deadlock, or recover poorly?",
      inspect: "file.patch",
      focus: "Realistic resource, concurrency, cancellation, and failure paths",
    },
    {
      true: {
        what: "A changed path can lose work, leak resources, hang, crash, or leave inconsistent state",
        examples: ["Cleanup is skipped after failure", "Concurrent work updates shared state unsafely"],
      },
      false: { what: "The patch preserves safe lifecycle and failure handling" },
    },
  ),
  compatibility: noul(
    {
      question: "Does file.patch directly support that this change can break an existing caller, format, protocol, or public behavior?",
      inspect: "file.patch",
      focus: "Externally observed contracts rather than internal implementation details",
    },
    {
      true: {
        what: "An existing consumer can fail because a contract changed without a safe migration",
        examples: ["A required field is removed", "A persisted value changes meaning"],
      },
      false: { what: "The changed contract remains compatible or is entirely internal" },
    },
  ),
  testGap: noul(
    {
      question: "Does file.patch change important behavior without adequate targeted evidence in changedTests?",
      compare: ["file.patch", "changedTests"],
      focus: "New branches, boundaries, failure paths, and component interactions",
    },
    {
      true: {
        what: "Important changed behavior has no targeted changed test",
        examples: ["A new failure branch has no assertion", "A protocol change lacks a compatibility test"],
      },
      false: {
        what: "Changed tests exercise the important behavior, or the patch is non-behavioral",
        examples: ["A focused regression test covers the branch", "Documentation-only change"],
      },
    },
  ),
}

const started = Date.now()
const response = await client.systemOne({ state: { file, changedTests: [] }, questions })
const latencyMs = Date.now() - started

const answers = response.answers
console.log("probabilities:", JSON.stringify({
  correctness: answers.correctness.noul,
  security: answers.security.noul,
  reliability: answers.reliability.noul,
  compatibility: answers.compatibility.noul,
  testGap: answers.testGap.noul,
}, null, 2))
console.log(`latencyMs: ${latencyMs}`)
// Full raw response so FINDINGS.md records the real shape (usage, confidence, ...).
console.log("raw:", JSON.stringify(response, null, 2))
