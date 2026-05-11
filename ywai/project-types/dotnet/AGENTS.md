# .NET Project Agent Instructions

## Scope

- This template applies to .NET / C# backend projects, especially ASP.NET Core APIs and Clean Architecture solutions.
- Follow the nearest nested `AGENTS.md` if a solution, app, or library adds more specific rules.
- Keep domain code independent from frameworks and infrastructure.

## Operating Workflow

1. Discover the solution layout: `.sln`, project references, `Directory.Build.props`, `global.json`, and test projects.
2. Respect existing architecture boundaries and dependency direction.
3. Implement in the smallest layer that owns the behavior.
4. Add/update tests near the changed code.
5. Verify with `dotnet` commands already supported by the repo.

## Architecture Rules

- Dependencies point inward: `Domain` <- `Application` <- `Infrastructure` <- `Presentation`.
- Domain entities/value objects must not depend on EF Core, ASP.NET Core, logging, or configuration APIs.
- Application defines ports/interfaces and use cases; Infrastructure implements adapters and persistence.
- Controllers/minimal APIs stay thin: validate, call application code, map results.
- Do not put business rules in controllers, EF configurations, repositories, or background job glue.

## C# Standards

- Nullable reference types must stay enabled.
- Use `async`/`await` end to end; never block with `.Result` or `.Wait()`.
- Prefer records for immutable DTOs/value objects and classes for behavior-rich domain entities.
- Use primary constructors when they reduce boilerplate without hiding dependencies.
- Prefer `IReadOnlyList<T>` or immutable collections for exposed collections.
- One public type per file; file name matches the primary type.

## ASP.NET Core Standards

- Return typed results or documented response types and use `ProblemDetails` for errors.
- Validate input at API boundaries with FluentValidation, DataAnnotations, or existing project conventions.
- Use `IOptions<T>`/options validation for configuration; avoid raw `IConfiguration` in domain/application services.
- Register services with intentional lifetimes; never use `IServiceProvider` as a service locator.
- Use policy-based authorization for permissions; avoid ad-hoc checks scattered across services.

## EF Core / Persistence

- Keep migrations and entity configurations in Infrastructure.
- Configure entities with `IEntityTypeConfiguration<T>` when using Clean Architecture.
- Use `AsNoTracking()` for read-only queries.
- Never interpolate raw SQL. Use LINQ, parameters, or safe APIs.
- Keep DbContext scoped; never singleton.

## Security and Observability

- No secrets in `appsettings*.json`; use environment variables or a secret manager.
- Do not log PII, credentials, tokens, or connection strings.
- Use `ILogger<T>` with structured logging; avoid `Console.WriteLine` in production code.
- Preserve trace/correlation IDs when adding middleware or clients.
- Add health checks for services that are deployed behind orchestrators/load balancers.

## Testing and Verification

Preferred commands when a solution exists:

```bash
dotnet restore
dotnet build --no-restore
dotnet test --no-build
dotnet format --verify-no-changes
```

Testing guidance:

- Unit test application/domain behavior in isolation.
- Integration test API and persistence boundaries with `WebApplicationFactory` and test databases when configured.
- Use deterministic test data; avoid `Thread.Sleep`.
- Cover success, validation failure, and authorization/error paths.

## Skills

Read `.atl/skill-registry.md` (or `skills/skill-registry.md` in older setups) for the authoritative skill list, trigger patterns, and compact rules. When work matches a skill trigger, invoke that skill first.
