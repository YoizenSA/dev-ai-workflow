# QA Playwright Code Review Rules

The reviewer must verify **each** point before approving the PR.
Any violation of points marked as 🛑 **BLOCKING** requires changes before merge.

## 1. Reliability & Determinism (🛑 BLOCKING)
- [ ] No `waitForTimeout()` / arbitrary sleeps used as synchronization.
- [ ] Assertions rely on Playwright auto-waiting or explicit state transitions.
- [ ] Tests do not depend on execution order or leaked shared state.
- [ ] Retries are not used to hide known failures.

## 2. Locator Quality (🛑 BLOCKING)
- [ ] Locators prefer `getByRole`, `getByLabel`, `getByPlaceholder`, or `getByTestId`.
- [ ] Brittle CSS/XPath selectors are avoided or justified in code review.
- [ ] Test names describe business behavior, not internal implementation.

## 3. Test Architecture
- [ ] Repeated setup is extracted into fixtures, helpers, or `storageState` when appropriate.
- [ ] Page Objects are introduced only when they reduce real duplication.
- [ ] Test data setup/cleanup is deterministic and worker-safe.
- [ ] Tags/projects (`@smoke`, `@critical`, browsers, devices) are used intentionally.

## 4. Coverage & Risk
- [ ] Critical user flows and negative/error paths affected by the change are covered.
- [ ] Accessibility, console errors, or network/error handling are tested when relevant.
- [ ] Cross-browser/device implications were reviewed when configuration or UI changed.

## 5. Diagnostics & CI
- [ ] Failures produce actionable artifacts (trace, screenshot, video, report) when needed.
- [ ] CI configuration changes preserve reproducibility and reasonable runtime.
- [ ] New environment variables or secrets are documented and never hardcoded.

## 6. Validation Commands
Run the equivalent commands for the repository package manager before requesting review:

```bash
npm run lint
npx tsc --noEmit
npx playwright test --list
npx playwright test
```
