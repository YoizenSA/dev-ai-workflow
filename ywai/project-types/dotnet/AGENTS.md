# .NET / C# Engineering Constitution & AI Agent Directives

## Part 1: Core Principles (NON-NEGOTIABLE)

### I. Architecture (Clean Architecture)
- **Layers**: `Domain` → `Application` → `Infrastructure` → `Presentation`. Dependencies point **inward only**.
- Domain entities have **no framework dependencies** — no EF Core annotations in domain models.
- Application layer defines interfaces; Infrastructure implements them.
- No business logic in controllers, minimal-api handlers, or data access classes.

### II. C# Code Quality
- **Nullable reference types ON**: `<Nullable>enable</Nullable>` in every project.
- `async`/`await` all the way down — never `.Result` or `.Wait()` on tasks.
- Use `record` for immutable value objects and DTOs.
- Use primary constructors where it reduces boilerplate without hiding intent.
- Prefer `ImmutableList<T>` / `IReadOnlyList<T>` for collections exposed from domain.

### III. Security-First
- **Never** store secrets in `appsettings.json` — use environment variables or Azure Key Vault / AWS Secrets Manager.
- Always use parameterized queries or EF Core — **no raw SQL string interpolation**.
- Validate all input at the API boundary using FluentValidation or DataAnnotations.
- Use `[Authorize]` + policy-based authorization — no ad-hoc permission checks in service layers.
- Enable HTTPS redirection and HSTS in production.

### IV. Observability
- Use `ILogger<T>` — no `Console.WriteLine` in production code.
- Structured logging: log at the right level (`Debug`, `Information`, `Warning`, `Error`, `Critical`).
- Correlate requests with `Activity` / OpenTelemetry trace IDs.
- No sensitive data (PII, tokens) in logs.

### V. Containers & Deployment
- Use multi-stage Docker builds with `mcr.microsoft.com/dotnet/sdk` for build, `mcr.microsoft.com/dotnet/aspnet` for runtime.
- Implement health checks using `IHealthCheck` + `app.MapHealthChecks("/health")`.
- Store connection strings and secrets in Azure Key Vault, AWS Secrets Manager, or environment variables — never in `appsettings.json`.
- Use `.dockerignore` to exclude `bin/`, `obj/`, and `.git/`.

---

## Part 2: Coding Standards

### Complexity Limits

| Element | Max Limit | Recommended |
|:---|:---:|:---:|
| **File Length** | **400 lines** | 100-200 |
| **Method Length** | **60 lines** | 15-30 |
| **Parameters** | 5 | 1-3 |
| **Cyclomatic Complexity** | 10 | < 5 |
| **Nesting Depth** | 3 | 2 |

### Naming Conventions (Microsoft standard)
- Types, methods, properties: `PascalCase`
- Local variables, parameters: `camelCase`
- Private fields: `_camelCase`
- Constants: `PascalCase` (not `UPPER_SNAKE`)
- Interfaces: `IEntityName`
- Async methods: suffix `Async` — `GetUserAsync()`

### Formatting
- Use `dotnet format` before every commit.
- EditorConfig file at repo root must define `indent_style`, `indent_size`, `end_of_line`, `charset`.
- Prefer `var` when the type is obvious from the right-hand side; use explicit types otherwise.

### General Rules
- Early returns / guard clauses to reduce nesting.
- Avoid `this.` prefix unless resolving ambiguity.
- Delete dead code — no commented-out blocks committed.
- One class per file. File name matches class name exactly.

---

## Part 3: ASP.NET Core Standards

### Controllers / Minimal APIs
- Keep handlers thin: validate → call Application service → return result.
- Use `[ProducesResponseType]` attributes or typed results (`Results.Ok<T>`) for clear OpenAPI docs.
- Return `ProblemDetails` for errors (use `app.UseExceptionHandler` / `IProblemDetailsService`).
- Do not inject `DbContext` directly into controllers — use repositories or use-case services.

### Dependency Injection
- Register services with the correct lifetime: `Transient`, `Scoped`, `Singleton`.
- **Never inject `IServiceProvider` to manually resolve dependencies** — that's a service-locator anti-pattern.
- Use `IOptions<T>` for configuration binding, never raw `IConfiguration` in application services.

### Entity Framework Core
- Migrations live in `Infrastructure` project, never in domain or app layers.
- Configure entities via `IEntityTypeConfiguration<T>` — no data annotations on domain models.
- Always dispose or scope `DbContext` correctly (never a singleton DbContext).
- Use `AsNoTracking()` for read-only queries.

---

## Part 4: Testing

- **Unit**: xUnit + Moq (or NSubstitute). Test one class in isolation — mock all dependencies.
- **Integration**: `WebApplicationFactory<TProgram>` for API integration tests with a test database.
- **E2E**: Playwright or Selenium for critical user journeys (use sparingly).
- Minimum coverage: **80%** on Application layer (use-cases / services).
- Arrange / Act / Assert structure — one logical assertion per test.
- Test method naming: `MethodName_StateUnderTest_ExpectedBehavior`.
- No `Thread.Sleep` in tests — use `Task.Delay` only with proper cancellation, or mock time providers.
- Use `Respawn` or `Testcontainers` for integration test database management.
- Use `Bogus` for test data generation.

---

## Part 5: Available Skills

This project has the following AI agent skills installed in `skills/`. Each skill is auto-invoked when you mention its trigger words, or you can call it explicitly.

### SDD Orchestrator

| Skill | Trigger words | Purpose |
|:---|:---|:---|
| `sdd-init` | "sdd init", "iniciar sdd" | Bootstrap `.sdd/` structure |
| `sdd-explore` | "explore", "investigar", "think through" | Explore ideas before committing |
| `sdd-propose` | "propose", "propuesta", "/sdd:new" | Create change proposal |
| `sdd-spec` | "spec", "requerimientos", "/sdd:ff" | Write specifications |
| `sdd-design` | "design", "diseño técnico" | Technical design document |
| `sdd-tasks` | "tasks", "breakdown" | Break change into tasks |
| `sdd-apply` | "apply", "implement", "/sdd:apply" | Implement tasks |
| `sdd-verify` | "verify", "verificar" | Validate implementation vs specs |
| `sdd-archive` | "archive", "archivar" | Archive completed change |

### Code Quality

| Skill | Trigger words | Purpose |
|:---|:---|:---|
| `git-commit` | "commit", "git", "versioning", "changelog" | Commit message standards (Conventional Commits) |

### Meta Skills

| Skill | Trigger words | Purpose |
|:---|:---|:---|
| `skill-creator` | "create a skill", "new skill", "document pattern" | Create new AI agent skills |
| `skill-registry` | "skill registry", "update skills" | Sync skill metadata with AGENTS.md |

### How to invoke

```
# SDD workflow
/sdd:new feature-name    # Start a new feature
/sdd:ff                  # Fast-forward (propose + spec + design + tasks)
/sdd:apply               # Implement tasks
/sdd:verify              # Verify implementation
/sdd:archive             # Archive when done
/sdd:status              # Show active changes

# Commits
> Write a conventional commit for these changes
```

---

## Auto-invoke Capabilities
| Action | Required Skill | Trigger Pattern |
| :--- | :--- | :--- |
| Entity Framework | `dotnet` | Entity Framework |
| Implement .NET patterns | `dotnet` | Implement .NET patterns |
| Minimal APIs | `dotnet` | Minimal APIs |
| Writing C# code | `dotnet` | Writing C# code |
