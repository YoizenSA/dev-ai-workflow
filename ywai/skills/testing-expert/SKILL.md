---
name: testing-expert
description: "Inspect test quality: assertion strength, coverage gaps, tests that pass while the feature is broken."
---

# Testing Expert

A test suite is only worth the failures it would catch. Judge tests by one question: **if the behavior broke, would this test go red?**

That reframes what looks like coverage. A suite at 90% line coverage whose assertions are `assert result` is close to 0% real coverage — every one of those tests passes on garbage data. The number measures which lines ran, not which mistakes are caught.

## The two failure modes

**False green** — the test passes while the feature is broken. Caused by assertions that cannot fail (truthiness on an object that always exists, `assertNotNone`, comparing a value to itself), by mocks so complete that only the mock is under test, and by tests that never exercise an error path. This is the expensive one: it consumes maintenance and returns false confidence, so the team stops looking where the bug actually is.

**Flaky** — the test fails while the feature works. Caused by timing guesses, shared state, ordering dependence, and real clocks or networks. It trains everyone to re-run the suite instead of reading it, which eventually hides a real failure.

## Inspecting

Start from the test's name and ask what behavior it claims to protect. Then check whether the assertions actually pin that behavior, whether removing the feature would make it fail, and whether the error paths and boundaries are covered at all. A test whose name and assertions disagree is a documentation bug and a coverage gap at once.

When you report gaps, name the consequence: what could ship broken and reach production undetected. "No test for the empty-cart case" is a note; "an empty cart throws in checkout and nothing would catch it" gets fixed.

## Reference

- [Inspection checklist](references/inspection-checklist.md) — the 2-minute pass and the deep pass
- [Red flags](references/red-flags.md) — merge-blocking patterns, with why each one is worthless
- [Assertion quality](references/assertion-quality.md) — weak vs. strong assertions
- [Example report](references/example-report.md) — a full inspection worked end to end
