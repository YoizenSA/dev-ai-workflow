# QA Playwright Project Agent Instructions

## Scope

- This template applies to browser E2E, API-assisted setup, accessibility checks, visual checks, and test reliability work.
- Optimize for release confidence: critical user journeys, integrations, and failure paths first.
- Treat flaky tests as defects in the automation or product contract until proven otherwise.

## Operating Workflow

1. Discover package manager, Playwright config, projects/browsers, base URLs, fixtures, and CI artifacts.
2. Identify the user-visible behavior or contract under test.
3. Prefer API/fixture setup over repeated UI setup.
4. Add focused tests with isolated data and stable locators.
5. Run the smallest relevant test first, then the broader suite if needed.

## Test Design Rules

- Test observable behavior, not implementation details.
- Prefer semantic locators: `getByRole`, `getByLabel`, `getByPlaceholder`, `getByText`, then `getByTestId`.
- Avoid CSS/XPath selectors unless no stable semantic locator exists.
- Keep one primary business behavior per test when possible.
- Use tags intentionally (`@smoke`, `@critical`, `@regression`, `@a11y`) and align with CI filters.

## Reliability Rules

- Never commit `waitForTimeout()` or arbitrary sleeps as synchronization.
- Use Playwright auto-waiting assertions and explicit UI/network state checks.
- Keep test data isolated per worker/browser context.
- Use `storageState`, fixtures, or API setup for authentication instead of logging in through UI in every test.
- Mock unstable third-party boundaries when live dependencies reduce reliability.

## Accessibility and Debuggability

- Cover keyboard navigation, labels, focus, and error states for critical flows.
- Preserve useful traces, screenshots, videos, and test steps on failure.
- Fail on unexpected console/runtime errors when the suite contract allows it.
- Make assertions explain the user-visible expectation.

## Security and Environment Rules

- Never hardcode real credentials, tokens, production data, or shared accounts in tests.
- Use environment-scoped test accounts and deterministic cleanup.
- Do not point destructive tests at production or shared persistent environments.

## Verification Commands

Detect package manager from lockfile and use existing scripts. Typical checks:

```bash
<pm> run lint
<pm> run typecheck
<pm> exec playwright test
<pm> exec playwright test --ui
<pm> exec playwright show-trace <trace.zip>
```

For debugging, run a targeted spec/project first before the entire suite.

## Skills

Read `.atl/skill-registry.md` (or `skills/skill-registry.md` in older setups) for the authoritative skill list, trigger patterns, and compact rules. When work matches a skill trigger, invoke that skill first.
