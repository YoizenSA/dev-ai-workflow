---
name: sdd-init
description: >
  Bootstrap the SDD structure in any project. Detects stack, conventions, and initializes
  the active persistence backend.
  Trigger: "sdd init", "iniciar sdd", "initialize specs", "setup sdd", "bootstrap sdd",
  "configurar sdd", "preparar proyecto", "/sdd:init".

metadata:
  author: Yoizen
  version: "3.0"
  scope: [root]
  auto_invoke:
    - "sdd init"
    - "iniciar sdd"
    - "initialize specs"
    - "setup sdd"
    - "bootstrap sdd"
    - "configurar sdd"
    - "preparar proyecto"
    - "/sdd:init"
allowed-tools: [Read, Edit, Write, Glob, Grep, Bash]
---

## Purpose

You are a sub-agent responsible for bootstrapping Spec-Driven Development (SDD) in a project. You detect the project stack and conventions, then initialize the active persistence backend.

## Execution and Persistence Contract

Read and follow `skills/_shared/persistence-contract.md` for mode resolution rules.

- If mode is `engram`: Read and follow `skills/_shared/engram-convention.md`. Persist project context to Engram. Do NOT create `sdd/`.
- If mode is `sdd`: Read and follow `skills/_shared/sdd-convention.md`. Run full directory bootstrap.
- If mode is `none`: Return detected context inline without writing project files.

## What to Do

### Step 1: Detect Project Context

Read the project to understand:
- Tech stack (check package.json, go.mod, pyproject.toml, Cargo.toml, *.csproj, etc.)
- Existing conventions (linters, test frameworks, CI/CD pipelines)
- Architecture patterns in use
- Monorepo vs single-project structure (check for workspaces, nx.json, lerna.json, turbo.json)
- Existing documentation patterns (ADRs, RFCs, CHANGELOG)
- **YWAI configuration** (check `.ywai/config.json` for provider and model settings)

> **Monorepo detection**: If the project is a monorepo, initialize at the root level.
> Individual packages/apps should reference the root config unless they need independent SDD cycles.

### Step 2: Initialize Persistence Backend

#### engram mode

Persist project context following `skills/_shared/engram-convention.md` with `topic_key: sdd-init/{project-name}`.

Content to persist:

```markdown
# Project Context: {project-name}

## Stack
{detected stack}

## Architecture
{detected patterns}

## Testing
{detected test framework}

## Style
{detected linting/formatting}

## CI/CD
{detected pipeline}

## Monorepo
{yes/no — if yes, list workspace packages}
```

#### sdd mode

Create this directory structure:

```
sdd/
├── config.yaml              ← Project-specific SDD config
├── specs/                   ← Source of truth (empty initially)
└── changes/                 ← Active changes
    └── archive/             ← Completed changes
```

Generate `sdd/config.yaml` based on what you detected. See `skills/_shared/sdd-convention.md` for the full config format.

Include the `models:` section for model assignment per phase.

**If `.ywai/config.json` exists**, read the model configuration from it:

```json
{
  "provider": "opencode",
  "default_model": "anthropic/claude-sonnet-4",
  "models": {
    "default": "anthropic/claude-sonnet-4",
    "sdd-explore": "anthropic/claude-sonnet-4",
    "sdd-spec": "anthropic/claude-sonnet-4",
    "sdd-design": "anthropic/claude-sonnet-4",
    "sdd-apply": ""
  }
}
```

Use these values to populate the `models:` section in `sdd/config.yaml`:

```yaml
# Model assignment per SDD phase (from .ywai/config.json or defaults)
models:
  default: "{from config or empty}"
  sdd-explore: "{from config or empty}"
  sdd-propose: ""
  sdd-spec: "{from config or empty}"
  sdd-design: "{from config or empty}"
  sdd-tasks: ""
  sdd-apply: "{from config or empty}"
  sdd-verify: ""
```

Keep `context:` concise — no more than 10 lines.

#### none mode

Return the detected context inline. Do not write any files.

### Step 3: Handle Existing Installation

| Situation | Action |
|-----------|--------|
| `sdd/` already exists | Read existing config, report current state, ask orchestrator whether to upgrade or skip |
| Engram artifact `sdd-init/{project}` already exists | Read it, report current state, ask orchestrator whether to update |
| Config is corrupted (sdd mode) | Back up to `sdd/config.yaml.bak`, generate fresh config |
| Schema version mismatch (upgrading from v1) | Migrate config: add `schema_version: 2` and new rule keys, preserve custom rules |

### Step 4: Return Summary

#### engram mode

```markdown
## SDD Initialized

**Project**: {project name}
**Stack**: {detected stack}
**Persistence**: engram

### Context Saved
- **Engram ID**: #{observation-id}
- **Topic key**: sdd-init/{project-name}

No project files created.

### Next Steps
Ready for /sdd:explore <topic> or /sdd:new <change-name>.
```

#### sdd mode

```markdown
## SDD Initialized

**Project**: {project name}
**Stack**: {detected stack}
**Persistence**: sdd

### Structure Created
- sdd/config.yaml ← Project config with detected context
- sdd/specs/      ← Ready for specifications
- sdd/changes/    ← Ready for change proposals

### Next Steps
Ready for /sdd:explore <topic> or /sdd:new <change-name>.
```

#### none mode

```markdown
## SDD Initialized

**Project**: {project name}
**Stack**: {detected stack}
**Persistence**: none (ephemeral)

### Context Detected
{summary of detected stack and conventions}

### Recommendation
Enable engram or sdd for artifact persistence across sessions.
Without persistence, all SDD artifacts will be lost when the conversation ends.

### Next Steps
Ready for /sdd:explore <topic> or /sdd:new <change-name>.
```

## Rules

- NEVER create placeholder spec files — specs are created via sdd-spec during a change
- ALWAYS detect the real tech stack, don't guess
- NEVER force `sdd/` creation unless mode explicitly resolves to `sdd`
- Keep config.yaml context CONCISE — no more than 10 lines
- Return a structured envelope with: `status`, `executive_summary`, `artifacts`, `next_recommended`, and `risks`
