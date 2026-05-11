# React Project Agent Instructions

## Scope

- This template applies to React 19 + TypeScript frontend projects, commonly with Tailwind CSS 4.
- Follow nested `AGENTS.md` files if app, package, or design-system directories provide more specific rules.
- Do not add a new framework, router, styling system, or state library without an explicit request.

## Operating Workflow

1. Detect package manager from lockfiles and use it consistently.
2. Read `package.json`, `tsconfig*.json`, Vite/Next/router config, Tailwind config/CSS, and test setup before editing.
3. Keep changes close to the owning feature/component.
4. Add/update tests for changed user-visible behavior.
5. Run existing lint/typecheck/test scripts, or report why they are unavailable.

## React 19 Rules

- Functional components only; no class components.
- Respect Rules of Hooks; never call hooks in loops, conditions, or nested functions.
- React Compiler handles most memoization. Avoid `useMemo`/`useCallback` unless there is a measured need or stable identity contract.
- Use Server Components by default in frameworks that support them; add `use client` only for browser APIs/interactivity.
- Keep components focused; split when a component exceeds roughly 80 lines or mixes unrelated responsibilities.
- Prefer colocated feature components/hooks over dumping everything into global folders.

## TypeScript Rules

- Keep `strict` enabled; avoid `any` and validate/narrow `unknown` at boundaries.
- Type component props explicitly and keep public component APIs small.
- Model async state explicitly (`idle`, `loading`, `success`, `error`) instead of scattered booleans.
- Do not suppress type errors without a short justification.

## Styling and Accessibility

- Use Tailwind CSS 4 when configured; do not introduce competing CSS systems.
- Use shared class merging helpers (`cn`) for conditional classes.
- Prefer design tokens/theme variables over hardcoded one-off values.
- Images need useful `alt` text or explicit decorative handling.
- Interactive elements must be keyboard accessible and use semantic HTML before custom ARIA.

## Testing and Verification

Detect package manager from lockfile (`pnpm-lock.yaml`, `package-lock.json`, `yarn.lock`, `bun.lock`/`bun.lockb`). Then use existing scripts. Typical checks:

```bash
<pm> run lint
<pm> run typecheck
<pm> test
<pm> exec playwright test
```

Testing guidance:

- Prefer React Testing Library for component behavior.
- Use Playwright for critical user journeys and accessibility smoke coverage.
- Mock network boundaries at the API/client layer unless the test is intentionally E2E.

## Skills

Read `.atl/skill-registry.md` (or `skills/skill-registry.md` in older setups) for the authoritative skill list, trigger patterns, and compact rules. When work matches a skill trigger, invoke that skill first.
