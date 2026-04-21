# Strict TDD Module — Apply Phase

> Load this file **only** when `sdd-apply` resolves TDD mode as active. When
> TDD is off, none of the rules below apply and no tokens should be spent on
> them.

## TDD Philosophy

TDD is not testing. TDD is **software design driven by tests**. You write a
test that describes what the code SHOULD do, then write the minimum code to
make it real. The tests design the API, the contracts, the behavior. Code is
a side effect of tests.

### The Three Laws

1. **Do NOT write production code** until you have a failing test.
2. **Do NOT write more test** than is necessary to fail.
3. **Do NOT write more code** than is necessary to pass the test.

## TDD Implementation Cycle

For EVERY task assigned to you, follow this cycle strictly:

```
FOR EACH TASK:
├── 0. SAFETY NET (only if modifying existing files)
│   ├── Run existing tests for files being modified
│   ├── Capture baseline: "{N} tests passing"
│   ├── If any FAIL → STOP, report as "pre-existing failure"
│   │   (do NOT fix pre-existing failures — report to orchestrator)
│   └── This baseline proves you did not break what already worked
│
├── 1. UNDERSTAND
│   ├── Read the task description
│   ├── Read relevant spec scenarios (these ARE your acceptance criteria)
│   ├── Read the design decisions (these CONSTRAIN your approach)
│   ├── Read existing code and test patterns (match the style)
│   └── Determine test layer (see "Choosing Test Layer" below)
│
├── 2. RED — Write a failing test FIRST
│   ├── Write test(s) that describe the expected behavior from the spec
│   ├── Prefer pure functions where possible (no side effects = easy to test)
│   ├── The test MUST reference production code that does NOT exist yet
│   │   (this guarantees failure — no need to execute to confirm)
│   ├── If the production code already exists:
│   │   └── Write a test for the NEW behavior that is NOT yet implemented
│   └── GATE: Do NOT proceed to GREEN until the test is written
│
├── 3. GREEN — Write the MINIMUM code to pass
│   ├── Implement ONLY what the failing test needs
│   ├── Fake It is VALID here (hardcoded return values are OK)
│   ├── EXECUTE tests → must PASS
│   │   ├── ✅ Passed → proceed to TRIANGULATE or REFACTOR
│   │   └── ❌ Failed → fix the implementation, NOT the test
│   └── GATE: Do NOT proceed until GREEN is confirmed by execution
│
├── 4. TRIANGULATE (MANDATORY for most tasks)
│   ├── DEFAULT: triangulation is REQUIRED. You need a compelling reason to skip
│   ├── Add a second test case with DIFFERENT inputs/expected outputs
│   ├── EXECUTE tests → if Fake It breaks (hardcoded no longer works):
│   │   └── Generalize to real logic (this is the whole point)
│   ├── Repeat until ALL spec scenarios for this task are covered
│   ├── MINIMUM: at least 2 test cases per behavior (happy path + one edge)
│   ├── WATCH OUT for GREEN that passes trivially:
│   │   ├── Test passes because the component/element isn't rendered → NOT GREEN
│   │   ├── Test passes because a loop iterates 0 times → NOT GREEN
│   │   ├── Test passes because setup doesn't trigger the code path → NOT GREEN
│   │   └── A real GREEN means: production code RAN and produced expected output
│   ├── Skip triangulation ONLY when ALL of these are true:
│   │   ├── The task is purely structural (config, constant, type export)
│   │   ├── There is literally ONE possible output (no branching, no logic)
│   │   └── You explicitly note "Triangulation skipped: {reason}" in the evidence table
│   └── GATE: All spec scenarios for this task must have tests before REFACTOR
│
├── 5. REFACTOR — Improve without changing behavior
│   ├── Extract constants (eliminate magic numbers)
│   ├── Extract functions (reduce cyclomatic complexity)
│   ├── Improve naming, remove duplication
│   ├── Push toward pure functions where feasible
│   ├── Apply Boy Scout Rule: leave code cleaner than you found it
│   ├── EXECUTE tests after EACH refactoring step → must STILL PASS
│   │   ├── ✅ Still passing → refactoring is safe, continue
│   │   └── ❌ Failed → REVERT that refactoring step, try smaller
│   └── GATE: Tests green after EVERY refactoring change
│
├── 6. Mark task complete [x]
└── 7. Note any deviations or issues discovered
```

## Choosing Test Layer

Determine test layer by WHAT the task does:

```
├── Pure logic, utility, calculation, data transformation
│   └── Unit test (always available if test runner exists)
│
├── Component rendering, user interaction, state changes
│   ├── IF integration tools available → Integration test
│   └── IF NOT → Unit test with mocks (degrade gracefully)
│
├── Multi-component flow, API interaction, context/provider behavior
│   ├── IF integration tools available → Integration test
│   └── IF NOT → Unit test with mocks
│
├── Critical business flow, full user journey, cross-page navigation
│   ├── IF E2E tools available → E2E test
│   ├── IF NOT but integration available → Integration test
│   └── IF neither → Unit test (degrade gracefully)
│
└── Default: Unit test (always the fallback)
```

**Key rule:** Use the HIGHEST available layer that fits the task. NEVER skip a
task because a layer is unavailable — degrade to the next available layer.

## Test Execution

Detect the test runner once and cache the command:

```
Read test command from, in order:
├── Cached testing capabilities (if available)
├── sdd/config.yaml → rules.apply.test_command (override)
└── Fallback: detect from package.json / pyproject.toml / go.mod / Makefile

When executing tests during TDD:
├── Run ONLY the relevant test file, not the entire suite
│   ├── JS/TS: {runner} {test-file} (e.g., pnpm vitest run src/utils/tax.test.ts)
│   ├── Python: pytest {test-file}
│   ├── Go: go test ./{package}/... -run {TestName}
│   └── Adapt to the runner's CLI
├── This keeps the cycle FAST
└── Full suite runs happen in sdd-verify, not here
```

## Pure Function Preference

When writing production code in GREEN/TRIANGULATE, prefer pure functions:

```ts
// ✅ PREFER (pure — easy to test)
function calculateDiscount(price: number, quantity: number): number {
  return quantity >= 5 ? price * quantity * 0.1 : 0;
}

// ❌ AVOID (impure — hard to test)
function calculateDiscount(item: Item) {
  globalState.lastDiscount = item.price * 0.1; // side effect
  updateDOM();                                  // side effect
  return globalState.lastDiscount;
}
```

Pure functions are deterministic (same input → same output), have no side
effects, and are trivially testable. TDD naturally pushes you toward pure
functions — embrace it.

## Approval Testing (for refactoring existing code)

When a task involves REFACTORING existing code (not writing new behavior):

```
BEFORE touching production code:
├── 1. Identify existing behavior to preserve
├── 2. Write "approval tests" that capture current behavior:
│   ├── Call the function with known inputs
│   ├── Assert the CURRENT outputs (even if ugly)
│   └── These tests document what the code does NOW
├── 3. Run approval tests → must PASS (they describe current reality)
├── 4. NOW refactor the production code
├── 5. Run approval tests again → must STILL PASS
│   ├── ✅ Passing → refactoring preserved behavior
│   └── ❌ Failing → refactoring broke something, revert
└── 6. If the spec says behavior should CHANGE:
    ├── Update the approval test to reflect NEW expected behavior
    ├── Run → test FAILS (RED — new behavior not implemented yet)
    └── Implement new behavior → GREEN
```

## Return Summary Extension

When Strict TDD Mode is active, your return summary MUST include:

```markdown
### TDD Cycle Evidence
| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| 1.1 | `path/test.ext` | Unit | ✅ 5/5 | ✅ Written | ✅ Passed | ✅ 3 cases | ✅ Clean |
| 1.2 | `path/test.ext` | Integration | N/A (new) | ✅ Written | ✅ Passed | ➖ Single | ✅ Clean |
| 1.3 | `path/test.ext` | Unit | ✅ 2/2 | ✅ Written | ✅ Passed | ✅ 2 cases | ➖ None |

### Test Summary
- Total tests written: {N}
- Total tests passing: {N}
- Layers used: Unit ({N}), Integration ({N}), E2E ({N})
- Approval tests (refactoring): {N} or "None — no refactoring tasks"
- Pure functions created: {N}
```

Column meanings:

- **Safety Net**: pre-existing tests run before modifying files. `N/A (new)` for new files.
- **RED**: test written first, referencing code that doesn't exist yet. Always `✅ Written`.
- **GREEN**: tests executed and passing after minimal implementation. Must show execution result.
- **TRIANGULATE**: additional test cases added to force real logic. `➖ Single` if spec has one scenario only.
- **REFACTOR**: code improved with tests still passing. `➖ None` if code was already clean.

## Assertion Quality Rules (MANDATORY)

**Every assertion must verify REAL behavior.** A test that passes without
exercising production logic is worse than no test — it gives false confidence.

### Banned Assertion Patterns (NEVER write these)

```
# TRIVIAL ASSERTIONS — test proves nothing
expect(true).toBe(true)              ❌ Tautology
expect(1).toBe(1)                    ❌ Tautology — no production code involved
assert True                          ❌ Always passes

# EMPTY COLLECTION ASSERTIONS without setup context
expect(result).toEqual([])           ❌ ONLY valid if you set up conditions for empty
expect(result).toHaveLength(0)       ❌ Same — why is it empty? Did production run?

# TYPE-ONLY ASSERTIONS — proves existence, not behavior
expect(result).toBeDefined()         ❌ Alone is useless — WHAT is the value?
expect(result).not.toBeNull()        ❌ Alone is useless — assert the actual value

# GHOST LOOP — assertion inside a loop that iterates 0 times
const items = screen.queryAllByTestId("item");  // returns []
for (const item of items) {
  expect(item).toHaveTextContent("value");       ❌ NEVER EXECUTES — dead code
}
# FIX: assert non-empty FIRST, then iterate:
expect(items).toHaveLength(3);                   ✅ Proves items exist
for (const item of items) { ... }                ✅ Now the loop actually runs
```

### What Makes a REAL Assertion

Every assertion must satisfy ALL of these:

1. **Calls production code** — the test invokes a function, method, or component from the implementation.
2. **Asserts a specific output** — compares against a concrete expected value derived from the spec.
3. **Would FAIL if the production code were wrong** — changing the implementation logic must break THIS test.

```ts
// ✅ REAL assertions — production code determines the result
expect(calculateDiscount(100, 10)).toBe(10);
expect(screen.getByText("Welcome, John")).toBeInTheDocument();
assert response.status_code === 403;
expect(result).toHaveLength(3);   // AND you set up exactly 3 items
```

### Empty Collection Rule

`expect(result).toEqual([])` is ONLY valid when:

1. You set up a specific precondition that SHOULD produce an empty result.
2. The production code actually ran and filtered/processed data to arrive at empty.
3. A companion test with different setup produces a NON-EMPTY result (triangulation).

If you cannot explain WHY the result is empty based on setup → the assertion
is trivial and banned.

### Smoke Test Rule

A test that only renders a component without asserting any output is NOT a
valid test:

```ts
// ❌ SMOKE TEST ONLY — proves nothing
render(<MyComponent data={mockData} />);
expect(screen.getByTestId("wrapper")).toBeInTheDocument();

// ✅ BEHAVIORAL TEST — proves what the component DOES
render(<MyComponent data={mockData} />);
expect(screen.getByText("Expected Title")).toBeInTheDocument();
expect(screen.getByRole("button")).toHaveTextContent("Submit");
```

"Renders without crash" is a smoke test. It does NOT count toward TDD
coverage.

### Mock Hygiene Rules

**If you need more mocks than assertions, you are testing at the WRONG level.**

```
Mock/assertion ratio guide:
├── ≤ 3 mocks for a test file → ✅ Healthy — focused test
├── 4–6 mocks → ⚠️ Consider extracting logic to a pure function
└── 7+ mocks → ❌ STOP — test at the wrong layer
    ├── Extract logic to a PURE FUNCTION and test it without mocks
    ├── OR move the test to integration/E2E where real dependencies exist
    └── NEVER write 10+ mocks to verify a one-line transformation
```

**Extract-Before-Mock Rule:** if the behavior is a transformation, mapping,
filtering, or conditional logic, EXTRACT it to a pure function FIRST, then
test the pure function directly.

```ts
// ❌ BAD: 15 mocks to test a one-line status conversion
vi.mock("next/navigation", ...);
vi.mock("next/link", ...);
// ... 12 more mocks ...
render(<StatusCell row={mutedRow} />);
expect(screen.getByText("FAIL")).toBeInTheDocument();

// ✅ GOOD: extract and test the logic directly
export function resolveDisplayStatus(status: string, isMuted: boolean): string {
  return status === "MUTED" ? "FAIL" : status;
}
// Test — ZERO mocks:
expect(resolveDisplayStatus("MUTED", true)).toBe("FAIL");
expect(resolveDisplayStatus("PASS", false)).toBe("PASS");
```

### Implementation Detail Coupling Rule

Tests must assert **behavior visible to the user**, not internal details:

```ts
// ❌ COUPLED TO IMPLEMENTATION
expect(element.className).toContain("text-xs");
expect(element.style.color).toBe("red");
expect(mockService.mock.calls.length).toBe(3);   // Why 3? Brittle.
expect(component.state.isLoading).toBe(true);    // Internal state.

// ✅ BEHAVIORAL — survives refactors
expect(screen.getByText("Error: Payment failed")).toBeInTheDocument();
expect(screen.getByRole("alert")).toHaveTextContent("Risk:");
expect(screen.getByRole("button")).toBeDisabled();
```

**CSS class assertions are NEVER valid test assertions.** If you need to
verify styling:

1. Test the **semantic outcome** (role, visible text, aria state, disabled,
   etc.).
2. OR use a visual regression tool / E2E screenshot comparison.
3. NEVER assert specific Tailwind/CSS class names — they are implementation
   details.

## Hard Gate

If Strict TDD Mode is active:

- You MUST produce the TDD Cycle Evidence table in your return summary.
- Each task row MUST have RED → GREEN → (TRIANGULATE) → REFACTOR columns.
- If you complete a task WITHOUT writing tests first, mark it FAILED in the
  evidence table.
- The verify phase WILL reject your work if the TDD Evidence table is missing
  or incomplete.

**There is no silent fallback.** If you resolved Strict TDD as active, you
follow it or you report failure — you do NOT quietly switch to Standard Mode.

## Rules (Strict TDD specific)

- NEVER write production code before writing its test — the ONE rule that cannot be broken.
- NEVER skip the GREEN execution gate — you MUST run tests and confirm they pass.
- NEVER skip triangulation when the spec defines multiple scenarios — hardcoded Fake It must be forced out.
- NEVER write trivial assertions (see Banned Patterns) — they are WORSE than no test.
- ALWAYS verify every assertion CALLS production code and asserts a SPECIFIC value.
- ALWAYS run the Safety Net before modifying existing files — protect what already works.
- ALWAYS include the TDD Cycle Evidence table in the return — verify will check it.
- If a test runner execution fails for infrastructure reasons (not test failures), report as "Blocked" and continue to the next task.
- Prefer pure functions — but do not force it where it does not fit (e.g., React components with state).
- For refactoring tasks, ALWAYS write approval tests before touching code.
- Run ONLY the relevant test file during the cycle, not the full suite.
