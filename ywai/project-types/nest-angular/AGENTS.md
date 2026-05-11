# NestJS + Angular Project Agent Instructions

## Scope

- This template applies to full-stack repositories with a NestJS backend and Angular frontend.
- Keep backend and frontend boundaries explicit; avoid leaking persistence models directly into UI code.
- Follow nested `AGENTS.md` files if individual apps/packages provide more specific rules.

## Operating Workflow

1. Detect workspace/package manager from lockfiles and workspace config.
2. Identify the owning app/package before editing (`apps/`, `packages/`, `libs/`, or repo-specific layout).
3. For API changes, update DTOs/contracts, backend tests, frontend clients, and UI behavior together.
4. Run existing lint/typecheck/test commands for the touched packages.
5. Use Playwright for critical end-to-end flows when a user journey changes.

## Backend: NestJS Rules

- Keep controllers thin; place behavior in application services/use cases.
- Keep domain/business logic independent of Nest, ORM, Redis, and HTTP client details.
- DTOs own external validation; use `class-validator`/pipes when configured.
- Use `ConfigService` or typed config modules instead of scattered `process.env` reads.
- Use structured logging and never log secrets, tokens, or PII.

## Frontend: Angular Rules

- Prefer standalone components; do not introduce NgModules unless the codebase still depends on them.
- Use signals (`signal`, `computed`, `effect`) for local reactive state when compatible with existing patterns.
- Prefer `inject()` over constructor injection for new code unless local style says otherwise.
- Prefer built-in control flow (`@if`, `@for`, `@switch`) for new templates.
- Zoneless change detection is preferred for new setup, but do not partially migrate an existing app without a plan.
- Keep components focused; move shared UI into `shared/` only when it is truly reused.

## TypeScript and Contracts

- `strict` stays enabled; avoid `any` and validate `unknown` at boundaries.
- Keep API request/response contracts explicit and version/migrate breaking changes.
- Do not expose database entities directly to the frontend; use DTOs/view models.
- Keep generated clients or schema artifacts in sync when the repo uses them.

## Styling and UI

- Follow the project design system and component library first.
- If Tailwind is configured, prefer utility classes and shared `cn`/class helpers where available.
- Ensure form controls have labels, validation messages, and accessible error states.
- Do not use inline styles unless values are dynamic and cannot be represented by the styling system.

## Testing and Verification

Detect package manager from lockfile and use existing scripts. Typical checks:

```bash
<pm> run lint
<pm> run typecheck
<pm> test
<pm> exec playwright test
```

Testing guidance:

- Backend: unit test services/use cases and E2E test changed controllers.
- Frontend: test components/services with Angular testing utilities or configured test runner.
- E2E: cover critical success/error paths that cross frontend and backend.

## Skills

Read `.atl/skill-registry.md` (or `skills/skill-registry.md` in older setups) for the authoritative skill list, trigger patterns, and compact rules. When work matches a skill trigger, invoke that skill first.
