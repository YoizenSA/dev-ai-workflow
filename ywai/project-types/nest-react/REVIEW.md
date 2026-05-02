# Full-Stack NestJS + React Code Review Checklist

The reviewer must verify **each** point before approving the PR.
Any violation of points marked as BLOCKING requires changes before merge.

## 0. Process (BLOCKING)
- [ ] Spec exists: proposal + tasks via SDD, or Bugfix Rationale in PR description.
- [ ] Implemented code matches the plan.

## 1. Backend — NestJS (BLOCKING)
- [ ] **Clean Architecture**: Domain does not import Infrastructure.
- [ ] **Single Responsibility**: Each service/class does one thing well.
- [ ] **Size Limits**: No file exceeds 500 lines. No function exceeds 80 lines. Constructor injections ≤ 5.
- [ ] **Location**: Code is in the correct folder (`dto`, `services`, `controllers`, `domain`, `infrastructure`).
- [ ] **NO Hardcoded Secrets**: No keys, tokens, or passwords.
- [ ] **DTOs**: Validators (`@IsString()`, etc.) with `whitelist: true`.
- [ ] **Async/Await**: No callback hell or detached promises.
- [ ] **Pagination** for list endpoints.
- [ ] **No N+1** queries in loops.

## 2. Frontend — React (BLOCKING)
- [ ] **Functional components only** — no class components.
- [ ] **No `useMemo` / `useCallback`** unless React Compiler is disabled.
- [ ] **Hooks rules**: top-level only, no hooks in loops/conditions.
- [ ] **`key` prop** on list items.
- [ ] **`alt` text** on images.
- [ ] **No `any`** — proper types or `unknown`.
- [ ] **Tailwind classes** for styling — no inline styles.
- [ ] **Responsive design** considered.
- [ ] **Components ≤ 80 lines**, files ≤ 400 lines.

## 3. TypeScript & Code Quality
- [ ] No `any` type.
- [ ] Props and function returns properly typed.
- [ ] Strict null checks: `?.` and `??` used where needed.
- [ ] Structured Logger (Pino) on backend — no `console.log`.
- [ ] Proper error handling: NestJS exceptions on backend, error boundaries on frontend.

## 4. Testing
- [ ] Backend: New business logic has unit tests.
- [ ] Frontend: New components have tests.
- [ ] E2E: Critical journeys covered if endpoints/routes changed.
- [ ] Coverage does not decrease significantly.

## 5. Maintainability
- [ ] No dead code or commented-out blocks.
- [ ] No unused imports.
- [ ] `TODOs` have associated ticket IDs.

---
*If this PR is an urgent Hotfix, it must bear `waiver-approved` label with a linked tech debt ticket.*
