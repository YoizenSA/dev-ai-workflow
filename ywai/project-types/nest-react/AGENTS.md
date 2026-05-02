# Full-Stack NestJS + React Engineering Constitution

## Stack
- **Backend**: NestJS + TypeScript (strict) + Clean Architecture
- **Frontend**: React 19 + TypeScript + Tailwind CSS 4
- **Shared**: TypeScript, Biome, Playwright

---

## Part 1: Backend — NestJS (NON-NEGOTIABLE)

### Architecture (Clean Architecture)
1. **Domain Layer (Pure)**: Entities and Business Rules.
   - ❌ No infrastructure dependencies (TypeORM, Axios, external modules).
   - ❌ No ORM decorators inside Domain Entities.
2. **Application Layer**: Use Cases / Services. Repository Interfaces (Ports).
   - ❌ No direct SQL/Redis queries.
3. **Infrastructure Layer**: Repository Implementations, HTTP Controllers, Cron Jobs.

### Security
- All external communication over HTTPS/TLS.
- Secrets in environment variables only — ❌ `const apiKey = "1234"`.
- All public endpoints use DTOs with `class-validator` (`whitelist: true`).

### Observability
- Structured logging with `Pino` (JSON in Production). No `console.log`.
- OpenTelemetry ready for distributed tracing.
- Stateless services — no in-memory state across restarts.

### NestJS Patterns
- Controllers: thin, delegate to Services immediately.
- Services: Dependency Injection via constructor.
- DTOs: `class-validator` decorators on all properties.
- Config: `ConfigService.get('VAR')` — never `process.env.VAR` directly.

### File & Complexity Limits

| Element | Max Limit | Recommended | Action if Exceeded |
|:---|:---:|:---:|:---|
| **File Length** | **500 lines** | 200-300 | Split into sub-services or utilities |
| **Method/Function** | **80 lines** | 20-40 | Extract to private methods or helpers |
| **Parameters** | 3 args | 1-2 | Use Options object or DTO |
| **Injections** | 5 deps | 3-4 | Apply Facade or split responsibilities |
| **Cyclomatic Complexity** | 10 | < 5 | Simplify / early returns |
| **Nesting Depth** | 3 levels | 2 | Guard Clauses (`if (!ok) return;`) |

### Naming Conventions

| Type | Convention | Example |
|:---|:---|:---|
| **Files** | `kebab-case` | `user-profile.service.ts` |
| **Classes** | `PascalCase` | `UserProfileService` |
| **Interfaces** | `I` + `PascalCase` | `IUserProfile` |
| **Methods/Variables** | `camelCase` | `findActiveProfile()` |
| **Constants** | `SCREAMING_SNAKE` | `MAX_RETRY_COUNT` |
| **Database Columns** | `snake_case` | `created_at` |
| **DTOs** | `PascalCase` + `Dto` | `CreateUserDto` |

### Backend Folder Structure
```text
src/modules/users/
├── controllers/
├── services/
├── domain/
├── infrastructure/
├── dto/
├── guards/
├── entities/
├── users.module.ts
└── users.constants.ts
```

---

## Part 2: Frontend — React 19 (NON-NEGOTIABLE)

### React Patterns
- **Functional components only** — no class components.
- **React Compiler** handles memoization — no `useMemo` / `useCallback` needed.
- **Hooks rules**: no hooks inside loops or conditions.
- **Server Components** by default — `'use client'` only when needed.

### Styling
- **Tailwind CSS 4** for all styling.
- Use `cn()` utility for conditional class merging.
- Theme variables via CSS custom properties, not `var()` in className.
- No inline styles. No CSS modules unless absolutely necessary.
- Responsive design by default.

### Component Rules
- One component per file. File name matches component name.
- Components ≤ 80 lines. Files ≤ 400 lines.
- Container/presentational pattern: smart components fetch data, dumb components receive props.
- Images must have `alt` text.

### Naming Conventions (React)

| Type | Convention | Example |
|:---|:---|:---|
| **Components** | `PascalCase.tsx` | `UserProfile.tsx` |
| **Hooks** | `use-kebab-case.ts` | `use-user-profile.ts` |
| **Utils** | `kebab-case.ts` | `format-date.ts` |
| **Constants** | `SCREAMING_SNAKE` | `MAX_RETRIES` |

### Frontend Folder Structure
```text
src/
├── components/
│   ├── ui/           # Shared presentational components
│   └── layout/       # Layout components
├── features/
│   ├── users/
│   │   ├── components/
│   │   ├── hooks/
│   │   └── utils/
│   └── dashboard/
├── hooks/            # Shared hooks
├── lib/              # Utilities, API clients
└── app/              # Routes / pages
```

---

## Part 3: Shared Standards

### TypeScript (strict)
- No `any` — use `unknown`, Generics, or DTO/Interface.
- No unused variables — remove or prefix with `_`.
- Strict null checks: use `?.` and `??`.
- Double quotes, semicolons, max 100 char lines.

### Testing
- **Backend**: Unit tests 80% coverage on services. E2E at least 1 success + 1 error per controller.
- **Frontend**: Component tests with React Testing Library.
- **E2E**: Playwright for critical user journeys across the full stack.
- Mock all external dependencies — never depend on real DB or API in unit tests.

### Error Handling
- Backend: Standard NestJS Exceptions (`NotFoundException`, `BadRequestException`). Never swallow errors.
- Frontend: Error boundaries + toast/notification for user-facing errors.

### Documentation
- Public APIs: JSDoc with `@param` and `@returns`.
- Complex logic: comment explaining WHY, not WHAT.
- Comments/code: English. User-facing text: adapt to user's language.

---

## Part 4: AI Agent Directives

### Implementation Workflow
1. **Analyze Context**: Read layer boundaries, naming conventions, existing patterns.
2. **Draft Code**: Generate the solution.
3. **Audit**:
   - File exceeds limits? → Split it.
   - Domain imports Infrastructure? → Move to Infrastructure.
   - Hardcoded secrets? → Replace with `ConfigService` / environment variables.
   - Using `useMemo` / `useCallback`? → Remove (React Compiler handles it).
   - Using class components? → Convert to functional.
4. **Final Output**: Clean, idiomatic code only.

### Safety Gates
- Hardcoded password/key → **WARNING** immediately.
- Destructive actions → Ask for explicit confirmation.

### Virtual Linter
Before outputting code, simulate `npm run lint` (Biome). Fix lint errors before showing code.

---

## Part 5: Available Skills

### React / Frontend

| Action | Skill |
|--------|-------|
| React components / hooks | `react-19` |
| Styling / Tailwind CSS | `tailwind-4` |
| Type definitions | `typescript` |

### Code Quality

| Action | Skill |
|--------|-------|
| Lint / format | `biome` |
| Git commit | `git-commit` |
| Create a skill | `skill-creator` |

### Auto-invoke

| Action | Required Skill | Trigger Pattern |
| :--- | :--- | :--- |
| Writing React code | `react-19` | Writing React code |
| Components | `react-19` | Components |
| Hooks | `react-19` | Hooks |
| State management | `react-19` | State management |
| Styling with Tailwind | `tailwind-4` | Styling with Tailwind |
| CSS utilities | `tailwind-4` | CSS utilities |
| Responsive design | `tailwind-4` | Responsive design |
| Writing TypeScript code | `typescript` | Writing TypeScript code |
| Type definitions | `typescript` | Type definitions |
| Generics | `typescript` | Generics |
| lint | `biome` | lint |
| format | `biome` | format |
| commit | `git-commit` | commit |
