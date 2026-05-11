# Python Project Agent Instructions

## Scope

- This template applies to Python backend projects such as FastAPI, Django, workers, and service libraries.
- Discover the actual framework and tooling from `pyproject.toml`, lockfiles, app layout, and tests before editing.
- Keep domain code independent from web framework, ORM, and external-service details when the project uses layered architecture.

## Operating Workflow

1. Detect Python version, package manager, formatter/linter, type checker, and test runner.
2. Use the existing environment workflow (`uv`, Poetry, pip-tools, venv, or repo scripts); do not mix package managers.
3. Implement minimal changes with full type annotations.
4. Add/update tests for changed behavior.
5. Run configured lint/format/type/test commands, or explain why they are unavailable.

## Architecture Rules

- Prefer Clean/Hexagonal Architecture for non-trivial services.
- Domain layer: pure Python types and business rules; no FastAPI/Django/SQLAlchemy dependencies.
- Application layer: use cases/services that depend on interfaces/protocols.
- Infrastructure layer: routes/views, ORM models, repositories, clients, queues, and adapters.
- Keep API schemas separate from persistence models unless the project intentionally combines them.

## Python Standards

- Python 3.11+ unless the project pins another version.
- Full type annotations for functions, methods, and public attributes.
- Prefer `dataclass`, `pydantic`, or explicit classes according to existing style.
- Avoid bare `except`; catch specific exceptions and preserve context.
- Avoid mutable default arguments.
- Keep imports organized by the configured tool.

## FastAPI / API Guidance

- Use Pydantic models for request/response validation.
- Define `response_model` or typed responses according to project style.
- Use `Annotated` dependencies when the project supports it.
- Prefer async routes for I/O-bound work; never call `time.sleep()` inside async code.
- Return appropriate HTTP status codes and avoid leaking internal exception details.

## Security and Observability

- Never hardcode secrets or credentials; use environment variables or secret managers.
- Do not log tokens, passwords, PII, or raw authorization headers.
- Use parameterized SQL or ORM query APIs; never interpolate SQL with f-strings.
- Validate file paths, URLs, shell inputs, and user-provided identifiers before use.
- Use structured logging where the project supports it.

## Testing and Verification

Prefer repo-defined commands. Common checks when configured:

```bash
uv run ruff check .
uv run ruff format --check .
uv run mypy .
uv run pytest
```

If the repo does not use `uv`, run the equivalent Poetry, tox, nox, make, or venv commands already documented.

Testing guidance:

- Use `pytest` and `pytest-asyncio` for async code when configured.
- Mock external services and use factories/fixtures for deterministic data.
- Prefer `pytest.mark.parametrize` for behavior matrices.
- Avoid real network calls in unit tests.

## Skills

Read `.atl/skill-registry.md` (or `skills/skill-registry.md` in older setups) for the authoritative skill list, trigger patterns, and compact rules. When work matches a skill trigger, invoke that skill first.
