# Consulting ADRs and linking them to code

Reference for `adr-skill`. Read this before implementing in a repo that has ADRs.

## Consulting ADRs (Read Workflow)

Agents should read existing ADRs **before implementing changes** in a codebase that has them. This is not part of the create-an-ADR workflow — it's a standalone operation any agent should do.

### When to Consult ADRs

- Before starting work on a feature that touches architecture (auth, data layer, API design, infrastructure)
- When you encounter a pattern in the code and wonder "why is it done this way?"
- Before proposing a change that might contradict an existing decision
- When a human says "check the ADRs" or "there's a decision about this"
- When you find an `ADR-NNNN` reference in a code comment

### How to Consult ADRs

1. **Find the ADR directory.** Check `docs/decisions/`, `adr/`, `docs/adr/`, `decisions/`. Also check for an index file (`README.md` or `index.md`).

2. **Scan titles and statuses.** Read the index or list filenames. Focus on `accepted` ADRs — these are active decisions.

3. **Read relevant ADRs fully.** Don't just read the title — read context, decision, consequences, non-goals, AND the Implementation Plan. The Implementation Plan tells you what patterns to follow and what files are governed by this decision.

4. **Respect the decisions.** If an accepted ADR says "use PostgreSQL," don't propose switching to MongoDB without creating a new ADR that supersedes it. If you find a conflict between what the code does and what the ADR says, flag it to the human.

5. **Follow the Implementation Plan.** When implementing code in an area governed by an ADR, follow the patterns specified in its Implementation Plan. If the plan says "all new queries go through the data-access layer in `src/db/`," do that.

6. **Reference ADRs in your work.** Add `ADR-NNNN` references in code comments and PR descriptions (see "Code ↔ ADR Linking" below).

## Code ↔ ADR Linking

ADRs should be bidirectionally linked to the code they govern.

### ADR → Code (in the Implementation Plan)

The Implementation Plan section names specific files, directories, and patterns:

```markdown
## Implementation Plan
- **Affected paths**: `src/db/`, `src/config/database.ts`, `tests/integration/`
- **Pattern**: all database queries go through `src/db/client.ts`
```

### Code → ADR (in comments)

When implementing code guided by an ADR, add a comment referencing it:

```typescript
// ADR-0004: Using better-sqlite3 for test database
// See: docs/decisions/0004-use-sqlite-for-test-database.md
import Database from 'better-sqlite3';
```

Keep these lightweight — one comment at the entry point, not on every line. The goal is discoverability: when a future agent reads this code, they can find the reasoning.

### Why This Matters

- An agent working in `src/db/` can find which ADRs govern that area
- An agent reading an ADR can find the code that implements it
- When an ADR is superseded, the code references make it easy to find everything that needs updating

