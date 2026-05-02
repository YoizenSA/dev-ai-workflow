# Python Engineering Constitution & AI Agent Directives

## Part 1: Core Principles (NON-NEGOTIABLE)

### I. Technology & Version Lock
- **Runtime**: Python 3.11+ (defined in `.python-version` or `pyproject.toml`).
- **Type Hints**: Mandatory. All functions and methods must have full type annotations.
- **Package Manager**: `uv` preferred. `pip` acceptable. Never mix both.
- **Legacy Code Policy**: Untyped or non-PEP8 code is considered "Legacy". Refactor, don't extend.

### II. Architecture Strategy
- Follow **Clean Architecture** or **Hexagonal Architecture** depending on project size.
- **Domain Layer**: Pure Python classes, no framework dependencies (no FastAPI, Django ORM).
- **Application Layer**: Use cases / services. Depends only on domain interfaces.
- **Infrastructure Layer**: FastAPI routes, Django views, SQLAlchemy models, external APIs.

### III. Security-First
- **Zero Trust**: All external calls over HTTPS.
- **Secrets Management**: Use `python-dotenv` or environment variables. Never hardcode.
- ❌ `API_KEY = "sk-1234"` → Immediate BLOCK.
- **Input Validation**: Use `pydantic` models for all external input.
- **SQL Injection**: Always use parameterized queries or ORM — never f-strings in SQL.
- **Dependency Audit**: Run `pip audit` or `safety check` periodically.

### IV. Code Quality
- **Linter**: `ruff` (replaces flake8 + isort + black).
- **Formatter**: `ruff format`.
- **Max Line Length**: 100 characters.
- **No bare `except:`** — always catch specific exceptions.
- **Type checking**: Use `mypy` or `pyright` in strict mode when available.

### V. Environment Management
- **Virtual environments**: Always use `venv`, `uv venv`, or `conda` — never install globally.
- **Lock files**: Maintain `requirements.lock`, `uv.lock`, or `poetry.lock` for reproducibility.
- **Python version**: Pin in `.python-version` or `pyproject.toml`'s `requires-python`.
- **Docker**: Use multi-stage builds with pinned base images (`python:3.12-slim`).

---

## Part 2: Coding Standards

### File & Complexity Limits

| Element | Max Limit | Recommended |
|:---|:---:|:---:|
| **File Length** | **400 lines** | 150-200 |
| **Function Length** | **60 lines** | 15-30 |
| **Parameters** | 4 args | 1-3 |
| **Cyclomatic Complexity** | 10 | < 5 |

### Naming Conventions

| Type | Convention | Example |
|:---|:---|:---|
| **Files/Modules** | `snake_case` | `user_service.py` |
| **Classes** | `PascalCase` | `UserService` |
| **Functions/Methods** | `snake_case` | `find_active_user()` |
| **Constants** | `SCREAMING_SNAKE` | `MAX_RETRY_COUNT` |
| **Private** | `_` prefix | `_internal_helper()` |

---

## Part 3: FastAPI Specifics (if applicable)

- All route parameters must use `pydantic` `BaseModel` for request/response.
- Use `Annotated` for dependency injection.
- Always define `response_model` on endpoints.
- Use `AsyncSession` for async database operations.
- Group routes in routers (`APIRouter`), never define all routes in `main.py`.
- Return proper HTTP status codes (201 for creation, 204 for deletion, etc.).
- Use `BackgroundTasks` for fire-and-forget operations.

### Async Patterns
- Prefer `async def` for I/O-bound routes.
- Use `sync_to_async` or thread pools for CPU-bound work inside async handlers.
- Never use `time.sleep()` in async code — use `asyncio.sleep()`.
- Use `asyncio.gather()` for concurrent I/O operations.
- Use `AsyncContextManager` for resource cleanup.

---

## Part 4: Testing & Reliability

- **Framework**: `pytest` + `pytest-asyncio` for async.
- All external calls must be mockable (use interfaces/protocols).
- Minimum coverage: **80%** for business logic.
- No `time.sleep()` in tests — use `asyncio.sleep` or fixtures.
- Use `factory_boy` or `faker` for test data generation.
- Use `pytest.mark.parametrize` for testing multiple scenarios from specs.
- Prefer `httpx.AsyncClient` over `TestClient` for async FastAPI tests.

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


