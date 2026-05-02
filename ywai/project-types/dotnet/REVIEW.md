# .NET / C# Code Review Checklist

## Automatic BLOCK Conditions

A PR is **blocked** and cannot merge if any of the following are true:

- [ ] Secret or credential hardcoded anywhere in source or config files.
- [ ] Raw SQL string interpolation (SQL injection risk).
- [ ] `.Result` or `.Wait()` called on a `Task` (deadlock risk).
- [ ] `<Nullable>` not enabled in a new project file.
- [ ] Business logic placed in a controller, handler, or data access class.
- [ ] `IServiceProvider` injected to resolve dependencies at runtime (service-locator anti-pattern).
- [ ] `Console.WriteLine` / `Debug.WriteLine` in production code (use `ILogger<T>`).
- [ ] Domain model annotated with EF Core data annotations.
- [ ] `dotnet format` not run — build fails due to formatting.

---

## Required Checks

### Architecture
- [ ] Dependency direction respected: Presentation → Application → Domain; Infrastructure → Application.
- [ ] No cross-layer leakage (e.g., EF Core `DbContext` in Application layer).
- [ ] New use-cases are in Application layer behind an interface.
- [ ] Infrastructure implementations do not bleed into domain/application.

### C# Code Quality
- [ ] All `async` methods return `Task` / `Task<T>` / `ValueTask` and are named with `Async` suffix.
- [ ] Nullable warnings resolved — no `#nullable disable` suppressions without justification.
- [ ] Collections returned from domain/application are `IReadOnlyList<T>` or similar read-only types.
- [ ] `record` used for DTOs and value objects (immutability by default).
- [ ] No unused `using` directives.

### ASP.NET Core
- [ ] Controller/handler only validates, delegates to application service, and maps result to HTTP response.
- [ ] `[ProducesResponseType]` or typed results (`Results.Ok<T>`) documented on all endpoints.
- [ ] Errors returned as `ProblemDetails` — no plain string error messages.
- [ ] Authorization applied to all non-public endpoints.

### Entity Framework Core
- [ ] All DB access uses parameterized queries or EF.Core — no raw string interpolation.
- [ ] Read-only queries use `AsNoTracking()`.
- [ ] New migrations have been generated and reviewed.
- [ ] `DbContext` lifetime is Scoped (not Singleton).

### Testing
- [ ] New use-case / service has corresponding unit tests.
- [ ] Mocks use Moq or NSubstitute — no concrete dependency instantiation in unit tests.
- [ ] Integration tests use `WebApplicationFactory<TProgram>` with a test DB.
- [ ] Test names follow `MethodName_StateUnderTest_ExpectedBehavior` convention.
- [ ] Coverage on Application layer ≥ **80%**.

### Security
- [ ] No secrets in `appsettings.json` or `appsettings.Development.json`.
- [ ] All user input validated at API boundary (FluentValidation / DataAnnotations).
- [ ] HTTPS redirection and HSTS enabled in production middleware pipeline.
- [ ] No PII or tokens logged.

---

## Code Quality

### Formatting
Run before requesting review:
```bash
dotnet format
```
All issues must be resolved. No suppression comments without justification.

### Static Analysis
If Roslyn analyzers or SonarAnalyzer are configured:
```bash
dotnet build --no-incremental
```
Zero new warnings introduced.

---

## Review Sign-off

| Area | Status |
|:---|:---:|
| Clean Architecture layer rules | ⬜ |
| C# code quality & nullability | ⬜ |
| ASP.NET Core patterns | ⬜ |
| Entity Framework Core | ⬜ |
| Testing | ⬜ |
| Security | ⬜ |
| `dotnet format` clean | ⬜ |
