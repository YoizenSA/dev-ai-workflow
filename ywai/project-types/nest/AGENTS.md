# NestJS — Project Engineering Directives

## Architecture

**Stack**: NestJS · TypeScript (strict) · Node.js LTS · configured ORM/DB/cache

### Layer Boundaries (Clean Architecture)

| Layer | Contains | Allowed | Forbidden |
|:---|:---|:---|:---|
| **Domain** | Entities, value objects, business rules | Pure TS, interfaces | ORM decorators, Axios, NestJS modules, infrastructure imports |
| **Application** | Use cases, services, orchestration | Repository interfaces (ports), DTO contracts | Direct SQL/Redis queries, framework glue |
| **Infrastructure** | Repositories, controllers, cron jobs, adapters | Third-party libs, DB drivers, Nest providers | Business rules |

- ORM decorators/configuration go in Infrastructure entities, never in Domain entities.
- Controllers are delivery adapters; they validate, delegate, and map responses.
- Legacy code that violates boundaries should be refactored toward these boundaries, not extended further.

## Date/Time Convention (CRITICAL)

Unless the project has a stricter convention, use one strategy end-to-end:

1. **Transit**: UTC timestamps (`epoch ms` or ISO UTC). Carry client offset/timezone explicitly when needed.
2. **Persistence**: UTC only. Never persist local time or mixed timezone suffixes.
3. **Queries**: filter in UTC. Apply offsets only to projected/display fields.
4. **Frontend/API display**: never apply offset twice; document whether a field is raw UTC or display-adjusted.
5. **Reports**: decide one layer to apply timezone conversion and keep upstream data raw UTC.
6. **Formats**: use `HH` for 24h persistence; use `hh` only with `A`/`a` for user-facing 12h.

## Coding Standards

### Limits

| Element | Max | Ideal |
|:---|:---:|:---:|
| File | 500 lines | 200-300 |
| Method | 80 lines | 20-40 |
| Params | 3 | 1-2 (use Options/DTO) |
| Constructor deps | 5 | 3-4 (Facade or split) |
| Cyclomatic | 10 | <5 |
| Nesting | 3 levels | 2 (guard clauses) |

### Naming

| Type | Convention | Example |
|:---|:---|:---|
| Files | `kebab-case` | `user-profile.service.ts` |
| Classes | `PascalCase` | `UserProfileService` |
| Interfaces | `I` + `PascalCase` | `IUserProfile` |
| Methods/Vars | `camelCase` | `findActiveProfile()` |
| Constants | `SCREAMING_SNAKE` | `MAX_RETRY_COUNT` |
| DB Columns | `snake_case` | `created_at` |
| DTOs | `PascalCase` + `Dto` | `CreateUserDto` |

### Structure

```text
src/modules/{feature}/
├── controllers/          # Thin HTTP endpoints
├── services/             # Application/business orchestration
├── domain/               # Pure models and business rules
├── infrastructure/       # Concrete repositories/adapters
├── dto/                  # Validation DTOs
├── guards/               # AuthZ guards
├── entities/             # ORM entities
├── {feature}.module.ts
└── {feature}.constants.ts
```

## NestJS Patterns

- Controllers delegate to services/use cases immediately.
- Services use constructor dependency injection.
- DTOs use `class-validator`; global validation should use `whitelist: true` when compatible.
- Config via `ConfigService.get()` or typed config wrappers; avoid raw `process.env` outside config modules.
- Keep providers focused. Split services that mix orchestration, persistence, and external API logic.

## Implementation Workflow

For every "Implement", "Refactor", or "Fix" task:

1. **Analyze**: read constraints above, existing module conventions, tests, and package scripts.
2. **Draft**: implement the smallest cohesive change in the owning layer/module.
3. **Audit**: file > 500 lines? ORM in Domain? hardcoded secrets? `any`? missing tests? Fix before output.
4. **Verify**: run the relevant existing lint/typecheck/test command or state why it could not run.
5. **Output**: summarize only sanitized, compliant changes and verification results.

## Quality Gates

- **Linter/formatter**: Biome when configured. Do not introduce a competing formatter.
- **No `any`**: use `unknown`, generics, DTOs, or explicit interfaces.
- **Strict nulls**: use optional chaining (`?.`) and nullish coalescing (`??`) where appropriate.
- **No unused vars**: remove or prefix with `_` only when required by an interface/contract.
- **Formatting**: double quotes, semicolons, and 80-100 char lines unless repo config differs.
- **Destructive actions**: `DROP TABLE`, data deletion, `rm -rf`, or force-push -> STOP and ask explicit confirmation with risk explanation.

## Security

- Secrets in environment variables or secret managers only. Hardcoded keys = immediate BLOCK.
- Public endpoints use DTO validation and sanitization.
- External communication uses HTTPS/TLS.
- Never log tokens, passwords, PII, or raw authorization headers.

## Observability

- Use structured logging; prefer Pino JSON in production when configured.
- Keep OpenTelemetry/correlation context intact when adding middleware or clients.
- Do not keep persistent state in local memory. Use Redis/SQL/queues for state that must survive restarts.

## Error Handling

- Use NestJS standard exceptions (`NotFoundException`, `BadRequestException`, etc.) at API boundaries.
- Never swallow errors silently.
- `try/catch` belongs around Infrastructure/external API calls or when adding context before rethrowing.
- Return safe error messages; do not expose internals to clients.

## Database

- Soft deletes on critical entities when business recovery/audit matters.
- Pagination is mandatory for list endpoints.
- Prefer explicit QueryBuilder/repository methods over magic ORM calls when queries are non-trivial.
- Keep read-only queries non-tracking/lightweight when the ORM supports it.

## Testing

- Unit: services/use cases and business logic, with mocked dependencies.
- E2E: at least one success and one error path per critical controller change.
- No real external services in unit tests; mock DB/API/cache boundaries.
- Run the package manager scripts already defined by the repo (`lint`, `typecheck`, `test`, `test:e2e`).

## Documentation

- Public methods: JSDoc with `@param` and `@returns` when the method is part of a public/internal API surface.
- Complex logic: comment WHY, not WHAT.
- Code/comments in English. Respond to the user in their language.

## Skills

Read `.atl/skill-registry.md` (or `skills/skill-registry.md` in older setups) for the authoritative skill list, trigger patterns, and compact rules. When work matches a skill trigger, invoke that skill first.
