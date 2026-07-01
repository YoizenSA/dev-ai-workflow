---
name: tdd
description: >
  Test-driven development loop — drive features through tests one vertical slice
  at a time (red → green → refactor). Use when the user asks for test-first
  development, mentions TDD or red-green-refactor, wants to build a feature
  through tests, or the orchestrator runs the TDD flow.
---

# Test-Driven Development

A discipline for writing code test-first. Two roles share it: `@qa` writes failing tests, `@dev` makes them pass. Both follow the same loop.

A test is a spec: `it('user can checkout with a valid cart')` tells you a capability exists. Test the behavior the public interface promises, not how it's built. The warning sign is coupling — if renaming a private function breaks a test without any behavior changing, that test was wired to the implementation. Rewrite it to go through the public interface.

## Red → Green → Refactor

One loop, three beats. Each beat earns the next.

```
RED      → Write ONE failing test for the next behavior.
GREEN    → Write the minimal code that makes it pass.
REFACTOR → Clean up with tests green — never while red.
```

Why these constraints, not habit:

- **One test at a time** keeps each test pinned to real behavior you can name, not behavior you imagined.
- **Minimal code** stops the test from accidentally passing for the wrong reason. The less code, the fewer ways it can be coincidentally green.
- **Refactor only while green** — the green suite is your only signal that a change preserved behavior. Refactoring red means a broken test and a broken change layered together, and you can't tell which failure is which.

## Vertical slices, not horizontal

```
WRONG (horizontal):  test1, test2, test3  →  impl1, impl2, impl3
RIGHT (vertical):    test1→impl1  →  test2→impl2  →  test3→impl3
```

Writing all tests first produces tests for *imagined* behavior. They describe the shape you expect — data structures, function signatures — rather than anything a user does, so they barely react when real behavior changes. They feel like coverage and give none.

Go one **vertical slice** at a time: one behavior, test then code, end to end. Each slice responds to what the previous cycle taught you — because you just wrote the code, you know exactly what matters and how to verify it.

## Workflow

1. **Plan.** Confirm the public interface and which behaviors matter most. You can't test everything — prioritize critical paths and complex logic. Respect existing ADRs (from `@architect`) and the project's domain vocabulary.

   Done when you can name the interface the tests will call and list the behaviors in priority order.

2. **Tracer bullet.** Write ONE test that proves the whole path works end to end — input in, observable result out. Take it red → green. This is the thin spike that proves the path exists before you flesh it out.

   Done when one test has gone red, then green, against the real public interface.

3. **Incremental loop.** For each remaining behavior: red → green, one test at a time, only enough code to pass. Do not anticipate future tests — the next slice decides what the next test should be.

   Done when every listed behavior is covered by a green test.

4. **Refactor.** With the full suite green, extract duplication and move complexity behind simple interfaces. Run the tests after each step. If a refactor leaves you red, revert it and try a smaller one — red during refactor means you changed behavior, not just structure.

   Done when the code is clean and every test is still green.

## Per-cycle checklist

Run this against each test before moving to the next slice.

```
[ ] Test describes behavior, not implementation
[ ] Test uses the public interface only
[ ] Test would survive an internal refactor
[ ] Code is minimal for this test
[ ] No speculative features added
```
