## Test-Driven Development Discipline

When the orchestrator runs the TDD flow (or the user asks for test-first development), follow this shared discipline. `@qa` writes failing tests; `@dev` makes them pass. Both roles share the principles below.

### Philosophy

- **Test behavior, not implementation**: Tests exercise public interfaces and describe *what* the system does, not *how*. They should survive internal refactors.
- **A test reads like a spec**: `it('user can checkout with a valid cart')` tells you exactly what capability exists.
- **Warning sign**: if renaming an internal function breaks a test (without behavior changing), that test was coupled to implementation — rewrite it to go through the public interface.

### Red → Green → Refactor

```
RED      → Write ONE failing test for the next behavior
GREEN    → Write the minimal code to make it pass
REFACTOR → Clean up with tests green (never while red)
```

### Anti-Pattern: Horizontal Slices

**Do NOT write all tests first, then all implementation.** Writing tests in bulk produces tests for *imagined* behavior — they verify the shape of things (data structures, signatures) rather than real user-facing behavior, and become insensitive to actual changes.

```
WRONG (horizontal):  test1, test2, test3  →  impl1, impl2, impl3
RIGHT (vertical):    test1→impl1  →  test2→impl2  →  test3→impl3
```

Go one **vertical slice** at a time (tracer bullets). Each test responds to what you learned from the previous cycle — because you just wrote the code, you know exactly what behavior matters and how to verify it.

### Workflow

1. **Plan**: Confirm the public interface and which behaviors matter most. You can't test everything — prioritize critical paths and complex logic. Respect existing ADRs (from `@architect`) and the project's domain vocabulary.
2. **Tracer bullet**: Write ONE test that proves the path works end-to-end (red → green).
3. **Incremental loop**: For each remaining behavior, red → green. One test at a time, only enough code to pass. Don't anticipate future tests.
4. **Refactor**: With all tests green, extract duplication and move complexity behind simple interfaces. Run tests after each step. **Never refactor while red — get to green first.**

### Per-Cycle Checklist

```
[ ] Test describes behavior, not implementation
[ ] Test uses the public interface only
[ ] Test would survive an internal refactor
[ ] Code is minimal for this test
[ ] No speculative features added
```
