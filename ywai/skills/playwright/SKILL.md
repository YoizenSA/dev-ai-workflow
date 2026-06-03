---
name: playwright
description: E2E testing with Playwright. Browser automation, test frameworks, CI/CD integration.

allowed-tools: [Read, Edit, Write, Glob, Grep, Bash]
---

## When to Use It

- When writing end-to-end tests for web applications
- When configuring Playwright for a new project
- When setting up CI/CD pipelines for browser testing
- When debugging flaky E2E tests

## Critical Patterns

- Use page object model for maintainable tests
- Prefer `data-testid` attributes over CSS selectors for element targeting
- Run tests in parallel where possible (shard across workers)
- Use `test.step` to group related actions and improve trace readability
- Always configure `retries` and `trace: 'on-first-retry'` for CI reliability
- Separate test environments (local, staging, prod) via project configs
