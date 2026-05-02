# QA Playwright Constitution & AI Agent Directives

## Part 1: Core Principles (NON-NEGOTIABLE)

### I. Release Confidence
- Prioritize critical user journeys, integrations, and failure paths before edge-case cosmetics.
- Prefer deterministic coverage over a large but noisy suite.
- Treat every flaky test as a defect in the automation until proven otherwise.

### II. Test Design
- Test observable user behavior and system contracts, not implementation details.
- Prefer focused, isolated scenarios over long mega-flows.
- Use stable locators first: `getByRole`, `getByLabel`, `getByPlaceholder`, `getByTestId`.
- Avoid brittle CSS/XPath selectors unless no semantic option exists.

### III. Reliability
- Never commit `waitForTimeout()` or arbitrary sleeps as a synchronization strategy.
- Use Playwright auto-waiting assertions and explicit UI/network state checks.
- Keep data isolated per worker/browser context.
- Prefer fixtures, API setup, or `storageState` over repeated UI login flows in every test.

### IV. Accessibility & Observability
- Include accessibility, keyboard, and error-state coverage when the flow is user-critical.
- Preserve debuggability with traces, screenshots, videos, and clear test steps.
- Fail on unexpected console/runtime errors when the suite contract allows it.

### V. Security & Environments
- Never hardcode secrets, tokens, or shared credentials in tests.
- Use environment-scoped test accounts and deterministic cleanup.
- Mock unstable or third-party boundaries when live dependencies reduce reliability.

---

## Part 2: Playwright Standards

- Use tags intentionally: `@smoke`, `@critical`, `@regression`, `@a11y`.
- Keep one business behavior per test whenever possible.
- Extract Page Objects or fixtures only when duplication is real and repeated.
- Prefer API/database seeding to expensive UI setup.
- Keep CI runs headless, reproducible, and artifact-rich.
- Review browser/project matrix changes carefully to avoid invisible coverage regressions.

---

## Part 3: Verification Commands

Adapt commands to the repository package manager (`npm`, `pnpm`, or `bun`), but validate with equivalents to:

```bash
npm run lint
npx tsc --noEmit
npx playwright test
npx playwright test --ui
npx playwright show-trace path/to/trace.zip
```

---

## Part 4: Available Skills

This project has the following AI agent skills installed in `skills/`. Each skill is auto-invoked when you mention its trigger words, or you can call it explicitly.

### SDD Orchestrator

| Skill | Trigger words | Purpose |
|:---|:---|:---|
| `sdd-init` | "sdd init", "iniciar sdd" | Bootstrap `.sdd/` structure |
| `sdd-explore` | "explore", "investigar", "think through" | Explore ideas before committing |
| `sdd-propose` | "propose", "propuesta", "/sdd:new" | Create change proposal |
| `sdd-spec` | "spec", "requerimientos", "/sdd:ff" | Write specifications |
| `sdd-design` | "design", "diseño técnico" | Technical design document |
| `sdd-tasks` | "tasks", "breakdown" | Break change into an executable checklist |
| `sdd-apply` | "apply", "implement", "/sdd:apply" | Implement approved tasks |
| `sdd-verify` | "verify", "verificar" | Validate implementation vs specs |
| `sdd-archive` | "archive", "archivar" | Archive completed change |

### QA / Code Quality

| Skill | Trigger words | Purpose |
|:---|:---|:---|
| `playwright` | "playwright", "e2e", "flaky test", "page object model" | Playwright testing, debugging, architecture, accessibility, CI, and performance guidance |
| `biome` | "lint", "format", "code quality" | Linting, formatting, and code quality using Biome |
| `typescript` | "typescript", "type definitions", "generics" | Strict TypeScript patterns and type-safe test code |
| `git-commit` | "commit", "git", "versioning", "changelog" | Commit message standards (Conventional Commits) |

### Meta Skills

| Skill | Trigger words | Purpose |
|:---|:---|:---|
| `skill-creator` | "create a skill", "new skill", "document pattern" | Create new AI agent skills |
| `skill-registry` | "skill registry", "update skills" | Sync skill metadata with AGENTS.md |

---

## Auto-invoke Capabilities

| Action | Required Skill | Trigger Pattern |
| :--- | :--- | :--- |
| Playwright tests | `playwright` | Playwright tests |
| E2E testing | `playwright` | E2E testing |
| flaky tests | `playwright` | flaky tests |
| Page Object Model | `playwright` | Page Object Model |
| visual regression | `playwright` | visual regression |
| accessibility testing | `playwright` | accessibility testing |
| test automation | `playwright` | test automation |
| lint | `biome` | lint |
| format | `biome` | format |
| code quality | `biome` | code quality |
| Writing TypeScript code | `typescript` | Writing TypeScript code |
| Type definitions | `typescript` | Type definitions |
| Generics | `typescript` | Generics |
| commit | `git-commit` | commit |
| create a skill | `skill-creator` | create a skill |
| skill registry | `skill-registry` | skill registry |

---

## Part 5: How to invoke

```text
# SDD workflow
/sdd:new checkout-hardening
/sdd:ff checkout-hardening
/sdd:apply
/sdd:verify
/sdd:archive

# QA / Playwright requests
"Create Playwright coverage for the checkout happy path"
"Debug this flaky Playwright test in CI"
"Refactor repeated login steps into fixtures or storageState"
```
