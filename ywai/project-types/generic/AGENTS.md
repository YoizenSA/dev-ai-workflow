# Generic Project Agent Instructions

## Scope

- This file is the default `ywai init generic` template for repositories without a more specific profile.
- If a nested `AGENTS.md` exists, follow the nearest file for that path and keep this file as the global baseline.
- Do not assume a stack. Discover it from `README`, lockfiles, manifests, CI files, and existing tests before changing code.

## Operating Workflow

1. Discover: inspect the smallest useful set of files and existing conventions first.
2. Plan: for multi-file work, state a short plan and identify verification commands before editing.
3. Implement: make the smallest cohesive change; avoid unrelated rewrites and formatting churn.
4. Verify: run the project commands that already exist. If a command is missing or too expensive, say so explicitly.
5. Document: update `README`, `AGENTS.md`, `REVIEW.md`, or examples when behavior or workflows change.

## Non-Negotiables

- Never hardcode secrets, credentials, tokens, or shared test accounts.
- Treat external input as untrusted; validate at boundaries and sanitize before persistence or shell/API use.
- Ask before destructive actions such as deleting data, dropping databases, rewriting history, or force-pushing.
- Keep code/comments in English. User-facing product text should match the product or user's language.
- Prefer simple, boring solutions over speculative abstractions.

## Code Quality Baseline

- Keep modules focused and cohesive; split files that accumulate unrelated responsibilities.
- Use clear names that describe domain intent, not implementation tricks.
- Prefer early returns/guard clauses over deep nesting.
- Delete dead code instead of commenting it out.
- Preserve public contracts unless the requested change explicitly includes a migration.
- Follow the repository formatter/linter; do not introduce a second formatter without approval.

Recommended complexity limits unless the project says otherwise:

| Item | Limit | Preferred |
| --- | ---: | ---: |
| File length | 400 lines | 150-250 |
| Function/method length | 60 lines | 15-30 |
| Parameters | 4 | 1-3 |
| Nesting depth | 3 | 1-2 |
| Cyclomatic complexity | 10 | < 5 |

## Testing Baseline

- Add or update tests for changed behavior.
- Prefer deterministic tests with isolated data and mocked external services.
- Do not use arbitrary sleeps for synchronization; use observable state or test framework waits.
- Cover success, validation failure, and important edge cases for business logic.

## Verification Command Discovery

Use commands declared by the repo. Do not invent scripts.

- Node/TypeScript: detect `npm`, `pnpm`, `yarn`, or `bun` from the lockfile and use existing `package.json` scripts.
- Python: prefer `uv` if `uv.lock` or `pyproject.toml` indicates it; otherwise use the existing virtualenv/tooling.
- .NET: prefer `dotnet restore`, `dotnet build`, `dotnet test`, and `dotnet format` when a solution/project exists.
- DevOps: prefer static rendering/linting (`helm lint`, `helm template`, dry-runs, pipeline validators) before deployment.

## Skills

Read `.atl/skill-registry.md` (or `skills/skill-registry.md` in older setups) for the authoritative skill list, trigger patterns, and compact rules. When work matches a skill trigger, invoke that skill first.
