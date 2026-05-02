# NestJS Engineering Constitution & AI Agent Directives

## Part 1: Core Principles (NON-NEGOTIABLE)

### I. Technology & Version Lock
Technology versions are fixed to ensure stability and reproducibility across environments.
- **Runtime**: Node.js Active LTS (defined in `.nvmrc` or `package.json` engines).
- **Framework**: NestJS (Current Stable Major Version).
- **Language**: TypeScript (Latest Stable). **Strict Mode**: `true`.
- **Legacy Code Policy**: Any code not adhering to Clean Architecture principles is considered "Legacy". It must be refactorized, not extended.

### II. Architecture Strategy
Every NestJS service must adhere to **Modular Monolith** or **Microservices** patterns under **Clean Architecture** principles:

1.  **Domain Layer (Pure)**:
    - Contains Entities and Business Rules.
    - ❌ **FORBIDDEN**: Dependencies on infrastructure (TypeORM, Axios, external NestJS modules).
    - ❌ **FORBIDDEN**: ORM decorators inside Domain Entities.
2.  **Application Layer (Orchestration)**:
    - Contains Use Cases / Services.
    - ✅ **ALLOWED**: Repository Interfaces (Ports).
    - ❌ **FORBIDDEN**: Direct SQL/Redis queries (must use Repositories).
3.  **Infrastructure Layer (Adapters)**:
    - Contains Repository Implementations, HTTP Controllers, Cron Jobs.
    - ✅ **ALLOWED**: Third-party libraries, Database Drivers.

### III. Security-First
- **Zero Trust**: All external communication must be encrypted (HTTPS/TLS).
- **Secrets Management**: Credentials, tokens, and keys **MUST** reside in environment variables.
    - ❌ `const apiKey = "1234"` (Immediate BLOCK in Code Review).
- **Sanitization**: All public endpoints must use DTOs with strict validation (`class-validator` with `whitelist: true`).

### IV. Observability & Reliability
- **Structured Logging**: Mandatory use of `Pino` (JSON format in Production). No `console.log`.
- **Tracing**: OpenTelemetry instrumentation ready for distributed tracing.
- **Statelessness**: Services must not store state in local memory that needs to persist across restarts. Use Redis or SQL.

---

## Part 2: Coding Standards & Constraints

### File & Complexity Limits
Code must be readable and maintainable. If it exceeds these limits, it **must** be refactored.

| Element | Max Limit | Recommended | Action if Exceeded |
|:---|:---:|:---:|:---|
| **File Length** | **500 lines** | 200-300 | Split into sub-services or utilities. |
| **Method/Function** | **80 lines** | 20-40 | Extract logic to private methods or helpers. |
| **Parameters** | 3 args | 1-2 | Use an `Options` object or DTO. |
| **Injections (Constructor)** | 5 deps | 3-4 | Apply Facade Pattern or split responsibilities. |
| **Cyclomatic Complexity** | 10 | < 5 | Simplify logic / Use early returns. |
| **Nesting Depth** | 3 levels | 2 | Use Guard Clauses (`if (!ok) return;`). |

### Naming Conventions

| Type | Convention | Example |
|:---|:---|:---|
| **Files** | `kebab-case` | `user-profile.service.ts` |
| **Classes** | `PascalCase` | `UserProfileService` |
| **Interfaces** | `I` + `PascalCase` | `IUserProfile` |
| **Methods/Variables** | `camelCase` | `findActiveProfile()` |
| **Constants** | `SCREAMING_SNAKE` | `MAX_RETRY_COUNT` |
| **Database Columns** | `snake_case` | `created_at`, `user_id` |
| **DTOs** | `PascalCase` + `Dto` | `CreateUserDto` |

---

## Part 3: Folder Structure & Organization

### NestJS Feature Module (Standard)
```text
src/modules/users/
├── controllers/       # HTTP Endpoints
├── services/          # Application/Business Logic
├── domain/            # (Optional) Pure Models if strict Clean Arch
├── infrastructure/    # (Optional) Concrete Repositories
├── dto/               # Data Transfer Objects (Validation)
├── guards/            # Authorization Guards
├── entities/          # DB Entities (TypeORM/Prisma/Mongoose)
├── users.module.ts    # Module Definition
└── users.constants.ts # Local constants

```

### Shared / Libs Structure

Reusable code must reside in libraries or a `shared` module.

```text
libs/ (or src/shared/)
├── database/          # Connection configs
├── logging/           # Pino configuration
├── utils/             # Pure helpers (dates, strings)
└── filters/           # Global Exception Filters

```

---

## Part 4: Best Practices

### Error Handling

* Use **Standard NestJS Exceptions** (`NotFoundException`, `BadRequestException`).
* Never silently swallow errors.
* `try/catch` blocks should only be used in Infrastructure layers or when calling external APIs.

### Database Interaction

* **Soft Deletes**: Mandatory for critical entities (`deletedAt`).
* **Pagination**: Mandatory for endpoints returning lists (`limit`, `offset`/`cursor`).
* **QueryBuilder**: Preferred over complex "magic" ORM methods for better performance and control.

### Testing

* **Unit Tests**: Minimum 80% coverage in `services/` and business logic.
* **E2E Tests**: At least 1 success case and 1 error case per critical Controller.
* **Mocking**: Do not depend on a real DB for unit tests.

---

## Part 5: AI Agent & Linter Directives

### ⚡ Prime Directive: The "Virtual Linter"
Before outputting ANY code block, you must run a recursive internal simulation of `npm run lint` (Biome).

**If your generated code would fail the linter, you must FIX IT before showing it to the user.**

### strict-mode rules you must simulate:
1.  **No `any`**: Never use `any`. Use `unknown`, Generics `<T>`, or define a DTO/Interface.
2.  **No Unused Variables**: If a variable is declared but not used, remove it or prefix with `_`.
3.  **Strict Null Checks**: Do not assume values exist. Use optional chaining (`?.`) or Nullish Coalescing (`??`).
4.  **Formatting**:
    - Use double quotes `"` (unless configured otherwise).
    - Always include semicolons `;`.
    - Max line length: 80-100 characters (wrap long lines).

### Implementation Workflow

When asked to "Implement", "Refactor", or "Fix" something, follow this mental loop:

1.  **Analyze Context**: Read these constraints (Layer boundaries, naming conventions).
2.  **Draft Code**: Generate the solution mentally.
3.  **Audit (The Agent Step)**:
    - *Check:* Does this file exceed 500 lines? -> **Action:** Split it.
    - *Check:* Am I importing TypeORM in the Domain layer? -> **Action:** Move to Infrastructure.
    - *Check:* Are there hardcoded secrets? -> **Action:** Replace with `ConfigService`.
4.  **Final Output**: Present only the sanitized, compliant code.

### Security & Safety Gates

- **Secrets**: If you see a hardcoded password/key in the user's prompt or code, **WARNING** the user immediately and refactor it to use `process.env`.
- **Destructive Actions**: If asked to `DROP` tables or `rm -rf`, ask for explicit confirmation and explain risks.

### Code Documentation Strategy

- **Public Methods**: Must have JSDoc explaining `@param` and `@returns`.
- **Complex Logic**: Add a comment explaining *WHY*, not *WHAT*.
- **Language**:
    - Comments/Code: **English**.
    - User Interaction/Explanation: **English** (unless user speaks Spanish, then adapt).

### NestJS Specific Patterns

1.  **Controllers**: Keep them thin. Delegate logic to Services immediately.
2.  **Services**: Use Dependency Injection via constructor.
3.  **DTOs**: Always decorate properties with `class-validator` (e.g., `@IsString()`, `@IsOptional()`).
4.  **Configs**: Never use `process.env.VAR` directly in code. Use `ConfigService.get('VAR')`.

---

## Part 6: Agent Skills

Use these skills for detailed, project-specific patterns and workflows.

### Available Skills

| Skill | Description | URL |
|-------|-------------|-----|
| `biome` | Linting, formatting, and code quality using Biome | [SKILL.md](skills/biome/SKILL.md) |
| `git-commit` | Git commit standards and commit message formatting | [SKILL.md](skills/git-commit/SKILL.md) |
| `skill-creator` | Create new AI agent skills | [SKILL.md](skills/skill-creator/SKILL.md) |
| `skill-registry` | Sync skill metadata with AGENTS.md auto-invoke tables | [SKILL.md](skills/skill-registry/SKILL.md) |

### Auto-invoke Skills

When performing these actions, ALWAYS invoke the corresponding skill FIRST:

| Action | Skill |
|--------|-------|
| Run linting or formatting | `biome` |
| Work on code quality tasks (Biome/ESLint/Prettier) | `biome` |
| Create a git commit | `git-commit` |
| Work on versioning or release notes | `git-commit` |
| Create or document a new skill | `skill-creator` |
| After creating/modifying a skill | `skill-registry` |
| Regenerate AGENTS.md auto-invoke tables | `skill-registry` |
| Troubleshoot why a skill is missing from AGENTS.md auto-invoke | `skill-registry` |

---

