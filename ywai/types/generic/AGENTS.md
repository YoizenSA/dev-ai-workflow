# Engineering Constitution & AI Agent Directives

## Part 1: Core Principles (NON-NEGOTIABLE)

### I. Code Quality
- Write clean, readable, and maintainable code.
- Follow the language's idiomatic style and conventions.
- Avoid over-engineering: solve the problem at hand, not hypothetical future ones.

### II. Architecture
- **Single Responsibility**: Every module, class, and function does one thing well.
- **Dependency Direction**: High-level modules must not depend on low-level details.
- **No God Objects**: Split large classes or modules when they exceed their responsibility.

### III. Security-First
- **Zero Trust**: Never trust external input without validation.
- **Secrets Management**: Credentials and tokens **MUST** come from environment variables.
  - ❌ `const apiKey = "sk-1234"` → Immediate BLOCK.
- **Sanitize All Input**: Validate and sanitize any data coming from users or external APIs.
- **HTTPS Only**: All external communication must be encrypted.

### IV. Observability
- Use structured logging instead of raw print/console statements.
- No debug logs left in production code.
- Correlate logs with request/transaction IDs when applicable.

### V. Error Handling
- Never silently swallow errors — always log or propagate.
- Use the language's idiomatic error handling (exceptions, Result types, error returns).
- Differentiate between operational errors (expected) and programming errors (bugs).
- Return meaningful error messages at API boundaries — never expose internal details.

### VI. Documentation
- Public APIs and exported functions MUST have documentation comments.
- Complex logic needs a comment explaining WHY, not WHAT.
- Comments/code: **English**. User-facing text: adapt to user's language.
- Keep README updated when project setup or usage changes.

---

## Part 2: Coding Standards

### Complexity Limits

| Element | Max Limit | Recommended |
|:---|:---:|:---:|
| **File Length** | **400 lines** | 150-250 |
| **Function Length** | **60 lines** | 15-30 |
| **Parameters** | 4 | 1-3 |
| **Cyclomatic Complexity** | 10 | < 5 |
| **Nesting Depth** | 3 | 2 |

### Universal Rules
- Use early returns / guard clauses to reduce nesting.
- Prefer immutable data where possible.
- Name variables and functions to describe **what** they do, not **how**.
- Delete dead code instead of commenting it out.
- One class/component per file. File name reflects the primary export.
- Group imports: standard library → third-party → local. Alphabetize within groups.
- Prefer composition over inheritance.

---

## Part 3: Testing

- All new features require tests.
- Mock external dependencies in unit tests.
- Tests must be deterministic — no time-dependent or random failures.
- Aim for **80% minimum coverage** on business logic.
- Use Arrange/Act/Assert (or Given/When/Then) structure.
- Test names should describe the behavior being tested, not the implementation.
- Prefer testing behavior over implementation details.

---

## Part 4: AI Agent Directives

### Implementation Workflow

When asked to "Implement", "Refactor", or "Fix" something, follow this loop:

1. **Analyze Context**: Read constraints, existing patterns, architecture.
2. **Draft Code**: Generate the solution.
3. **Audit**:
   - Does this file exceed limits? → **Split it.**
   - Are there hardcoded secrets? → **Replace with env vars.**
   - Does it follow existing patterns? → **Align with codebase.**
4. **Final Output**: Present only clean, idiomatic code.

### Security & Safety Gates

- **Secrets**: If you see a hardcoded password/key, **WARNING** the user immediately.
- **Destructive Actions**: If asked to drop tables or delete data, ask for explicit confirmation.

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
| `skill-sync` | "skill-sync", "sync skills" | Sync skill metadata with AGENTS.md |

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
| CSS utilities | `tailwind-4` | CSS utilities |
| Components | `react-19` | Components |
| Generics | `typescript` | Generics |
| Hooks | `react-19` | Hooks |
| Responsive design | `tailwind-4` | Responsive design |
| State management | `react-19` | State management |
| Styling with Tailwind | `tailwind-4` | Styling with Tailwind |
| Type definitions | `typescript` | Type definitions |
| Writing React code | `react-19` | Writing React code |
| Writing TypeScript code | `typescript` | Writing TypeScript code |
