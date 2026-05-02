# Full-Stack NestJS + Angular Engineering Constitution

## Stack
- **Backend**: NestJS + TypeScript (strict) + Clean Architecture
- **Frontend**: Angular (standalone components, signals, zoneless)
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

## Part 2: Frontend — Angular (NON-NEGOTIABLE)

### Architecture (Scope Rule)
- Each component/directive/pipe belongs to exactly ONE feature or shared lib.
- `shared/` only for truly cross-cutting UI (buttons, inputs, modals).
- Feature modules import from `shared/` — features NEVER import from other features.

### Angular Patterns
- **Standalone components** — no NgModules.
- **Signals** for reactive state (`signal()`, `computed()`, `effect()`).
- **`inject()`** over constructor injection.
- **Zoneless** change detection preferred (`provideZonelessChangeDetection()`).
- **Built-in control flow** (`@if`, `@for`, `@switch`) — no `*ngIf`, `*ngFor`.

### Component Rules
- One component per file. File name matches selector: `user-profile.component.ts`.
- Components ≤ 80 lines. Files ≤ 400 lines.
- Smart/dumb pattern: container components handle state, presentational components receive `input()` and emit `output()`.

### Styling
- Prefer Tailwind CSS or Angular Material — no inline styles.
- Responsive design by default.

### Naming Conventions (Angular)

| Type | Convention | Example |
|:---|:---|:---|
| **Components** | `kebab-case.component.ts` | `user-profile.component.ts` |
| **Services** | `kebab-case.service.ts` | `user-profile.service.ts` |
| **Directives** | `kebab-case.directive.ts` | `highlight.directive.ts` |
| **Pipes** | `kebab-case.pipe.ts` | `truncate.pipe.ts` |
| **Selectors** | `app-kebab-case` | `<app-user-profile>` |

### Frontend Folder Structure
```text
src/app/
├── features/
│   ├── users/
│   │   ├── pages/
│   │   ├── components/
│   │   ├── services/
│   │   └── user.routes.ts
│   └── dashboard/
├── shared/
│   ├── components/
│   ├── directives/
│   ├── pipes/
│   └── utils/
└── app.routes.ts
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
- **Frontend**: Component tests with harnesses. Signal-based state testing.
- **E2E**: Playwright for critical user journeys across the full stack.
- Mock all external dependencies — never depend on real DB or API in unit tests.

### Error Handling
- Backend: Standard NestJS Exceptions (`NotFoundException`, `BadRequestException`). Never swallow errors.
- Frontend: Global error handler + component-level error boundaries. User-friendly messages.

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
   - Using `*ngIf` / `*ngFor`? → Use `@if` / `@for`.
   - Using constructor injection? → Use `inject()`.
4. **Final Output**: Clean, idiomatic code only.

### Safety Gates
- Hardcoded password/key → **WARNING** immediately.
- Destructive actions → Ask for explicit confirmation.

### Virtual Linter
Before outputting code, simulate `npm run lint` (Biome). Fix lint errors before showing code.

---

## Part 5: Available Skills

### Angular

| Action | Skill |
|--------|-------|
| Component architecture / file placement | `angular` (architecture) |
| Standalone components / signals / inject | `angular` (core) |
| Signal Forms / Reactive Forms | `angular` (forms) |
| Performance / lazy loading / SSR | `angular` (performance) |

### Code Quality

| Action | Skill |
|--------|-------|
| Lint / format | `biome` |
| Git commit | `git-commit` |
| Create a skill | `skill-creator` |

### Auto-invoke

| Action | Required Skill | Trigger Pattern |
| :--- | :--- | :--- |
| Angular architecture | `angular` | Angular architecture |
| Angular components | `angular` | Angular components |
| Angular signals | `angular` | Angular signals |
| Angular forms | `angular` | Angular forms |
| Angular performance | `angular` | Angular performance |
| Standalone components | `angular` | Standalone components |
| Writing TypeScript code | `typescript` | Writing TypeScript code |
| Type definitions | `typescript` | Type definitions |
| lint | `biome` | lint |
| format | `biome` | format |
| commit | `git-commit` | commit |
