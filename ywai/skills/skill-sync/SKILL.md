---
name: skill-sync
description: >
  Sync skill metadata with the Auto-invoke sections in AGENTS.md and
  generate `.ywai/skill-registry.md` (compact rules) for sub-agent injection.
  Trigger: When you change a skill's metadata (metadata.scope/metadata.auto_invoke),
  regenerate the Auto-invoke tables, or run ./skills/skill-sync/assets/sync.sh.

metadata:
  author: Yoizen
  version: "2.0"
  scope: [root]
  auto_invoke:
    - "After creating/modifying a skill"
    - "Regenerate AGENTS.md Auto-invoke tables (sync.sh)"
    - "Generate .ywai/skill-registry.md compact rules (sync.sh --registry)"
    - "Troubleshoot why a skill is missing from AGENTS.md auto-invoke"
allowed-tools: Read, Edit, Write, Glob, Grep, Bash
---

## When to Use It

- After creating a new skill under `skills/`.
- After updating the metadata (`scope` or `auto_invoke`) of an existing skill.
- When troubleshooting a missing skill entry in an `AGENTS.md` Auto-invoke section.

## Critical Patterns

- **ALWAYS** run the script from a compatible shell (Git Bash on Windows) to avoid incompatibilities.
- **NEVER** edit the "Auto-invoke Capabilities" tables manually; the script overwrites them.
- **ALWAYS** ensure `SKILL.md` has valid YAML frontmatter before syncing.
- **ALWAYS** include the `root` scope when the skill must appear in the top-level `AGENTS.md`.

## Purpose (Overview)

Every skill that should show up in an Auto-invoke section must define these `metadata` fields.

`auto_invoke` can be a single string or a list of actions:

```yaml
metadata:
  author: YourName
  version: "1.0"
  scope: [root, backend]                   # AGENTS.md files to update

  # Option A: single action
  auto_invoke: "Creating/modifying components"

  # Option B: multiple actions
  # auto_invoke:
  #   - "Creating/modifying components"
  #   - "Refactoring business logic"
```

### Scope Values

| Scope | Updated File |
|-------|---------|
| `root` | `AGENTS.md` (root of project) |
| `copilot` | `.github/copilot-instructions.md` |
| `<custom>` | Auto-detected `AGENTS.md` in subdirectories matching scope name |

**Examples:**
- `scope: [root]` → Updates `/AGENTS.md`
- `scope: [root, backend]` → Updates `/AGENTS.md` and `/Backend/AGENTS.md` (if exists)
- `scope: [api, web]` → Updates any subdirectory containing "api" or "web" with AGENTS.md

Skills can define multiple scopes: `scope: [root, backend, api]`

---

## Usage

### After Creating/Modifying a Skill

```bash
./skills/skill-sync/assets/sync.sh
```

### What the Script Does

1. Reads every `skills/*/SKILL.md` file.
2. Extracts `metadata.scope` and `metadata.auto_invoke`.
3. Generates Auto-invoke tables for each `AGENTS.md`.
4. Rewrites the `### Auto-invoke Skills` section in every file.

---

## Commands

```bash
# Sync every AGENTS.md
./skills/skill-sync/assets/sync.sh

# Dry run (preview changes)
./skills/skill-sync/assets/sync.sh --dry-run

# Sync a specific scope
./skills/skill-sync/assets/sync.sh --scope backend

# Generate .ywai/skill-registry.md (compact rules for sub-agent injection)
./skills/skill-sync/assets/sync.sh --registry
```

## Compact Rules Registry

The `--registry` mode generates `.ywai/skill-registry.md` at the project root with
one compact block per installed skill. Each block has:

- `name`, `triggers`, `scope`, `version`, `source`
- A `rules:` list with 5–15 actionable bullets extracted from the skill's
  `## Critical Patterns` / `## Rules` section (falls back to the first bullet list
  found if neither heading exists).

Sub-agents consume this single file instead of loading every full `SKILL.md`,
reducing prompt tokens by ~40% on multi-skill project types.

### Example block

```markdown
## skill: typescript
triggers: ["*.ts", "*.tsx", "TypeScript refactor"]
scope: [root]
version: 1.0
source: skills/typescript/SKILL.md
rules:
- NEVER use `any`; prefer `unknown` with narrowing or generics.
- Explicit types on public signatures; inference in locals.
- `readonly` by default on arrays/objects of domain.
- Avoid `enum`; use union types `as const`.
- Discriminated unions to model state (loading/ok/error).
```
