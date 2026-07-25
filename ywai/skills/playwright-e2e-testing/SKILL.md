---
name: playwright-e2e-testing
description: Playwright end-to-end testing — locators, waiting, page objects, auth and network control, parallel runs, and debugging. Use when writing or fixing E2E/browser tests, when a test is flaky, or when another skill or agent needs Playwright conventions.
---

# Playwright E2E Testing

Playwright's defining feature is **auto-waiting**: every action retries until the element is actionable. Most E2E pain comes from fighting that instead of using it — a suite full of `waitForTimeout` is slower, flakier, and no more correct than one that lets Playwright wait.

Two failure modes matter more than everything else below.

**Flaky** — the test fails while the feature works. The cause is almost always a timing guess or shared state. Wait for the condition you actually depend on (`await expect(locator).toBeVisible()`), never for a duration, and give each test its own data: a hardcoded `test@test.com` collides the moment the suite runs in parallel.

**False green** — the test passes while the feature is broken. Locators that match by position (`div > form > input:nth-child(3)`) or by styling class survive changes they should have caught, and assertions loose enough to always hold (`toBeTruthy` on an object that always exists) can never fail.

## Locators

Prefer, in order: role with accessible name → label → `data-testid` → text. Reach for CSS or XPath only when nothing else identifies the element, and treat that as a signal the app is missing a test hook. Role-based locators double as an accessibility check — if you cannot find the button by its role and name, neither can a screen reader.

## Structure

Keep selectors and interactions in page objects rather than in the tests, so a UI change is one edit instead of twenty. Keep tests independent and parallel-safe: no ordering assumptions, no state carried between them, unique data per run.

## Reference

- [Setup, config, and CI](references/setup-and-config.md) — install, `playwright.config.ts`, pipelines, reporters
- [Test structure](references/test-structure.md) — test and describe blocks, hooks, fixtures
- [Locators, interactions, assertions](references/locators-and-actions.md) — locator API, forms, uploads, drag-and-drop, web-first assertions
- [Page object model](references/page-object-model.md) — page and component objects, test organization
- [Auth and network](references/auth-and-network.md) — storage state, session reuse, mocking, interception
- [Visual, parallel, debugging](references/advanced.md) — screenshots, sharding, trace viewer, UI mode
- [Common patterns](references/common-patterns.md) — recurring shapes worth copying

## Resources

[Playwright docs](https://playwright.dev/) · [Best practices](https://playwright.dev/docs/best-practices) · [API reference](https://playwright.dev/docs/api/class-playwright)
