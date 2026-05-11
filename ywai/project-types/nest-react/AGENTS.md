# NestJS + React Project Agent Instructions

## Scope

- This template applies to full-stack repositories with a NestJS backend and React 19 frontend.
- Keep backend DTOs/contracts, frontend API clients, and UI behavior in sync.
- Follow nested `AGENTS.md` files if individual apps/packages provide more specific rules.

## Operating Workflow

1. Detect workspace/package manager from lockfiles and workspace config.
2. Identify the owning app/package before editing (`apps/`, `packages/`, `libs/`, or repo-specific layout).
3. For API changes, update backend validation, shared types/clients, frontend states, and tests together.
4. Run existing lint/typecheck/test commands for touched packages.
5. Use Playwright for critical end-to-end flows when a user journey changes.

## Backend: NestJS Rules

- Keep controllers thin; place behavior in application services/use cases.
- Keep domain/business logic independent of Nest, ORM, Redis, and HTTP client details.
- DTOs own external validation; use `class-validator`/pipes when configured.
- Use `ConfigService` or typed config modules instead of scattered `process.env` reads.
- Use structured logging and never log secrets, tokens, or PII.

## Frontend: React 19 Rules

- Use functional components only.
- Respect Rules of Hooks; never call hooks conditionally or inside loops.
- React Compiler handles most memoization. Do not add `useMemo`/`useCallback` unless there is a measured need or API identity requirement.
- Use Server Components by default in frameworks that support them; add `use client` only for interactivity/browser APIs.
- Keep components focused; split UI when a component exceeds roughly 80 lines or mixes data fetching with presentation.

## TypeScript and Contracts

- `strict` stays enabled; avoid `any` and validate `unknown` at boundaries.
- Keep API request/response contracts explicit and version/migrate breaking changes.
- Do not expose database entities directly to the frontend; use DTOs/view models.
- Prefer discriminated unions for async UI states (`idle`, `loading`, `success`, `error`).

## Styling and UI

- Use Tailwind CSS 4 when configured; do not introduce a competing styling system without approval.
- Use shared class merging helpers (`cn`) for conditional classes.
- Prefer design tokens/theme variables over hardcoded one-off values.
- Images need useful `alt` text or explicit decorative handling.
- Ensure forms and interactive controls are keyboard accessible.

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
- Frontend: test behavior with React Testing Library or the configured runner.
- E2E: cover critical success/error paths that cross frontend and backend.

## Skills

Read `.atl/skill-registry.md` (or `skills/skill-registry.md` in older setups) for the authoritative skill list, trigger patterns, and compact rules. When work matches a skill trigger, invoke that skill first.
