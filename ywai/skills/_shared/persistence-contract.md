# Persistence Contract (shared across all SDD skills)

## Mode Resolution

The orchestrator passes `artifact_store.mode` with one of: `engram | sdd | none`.

Default resolution (when orchestrator does not explicitly set a mode):
1. If Engram is available → use `engram`
2. Otherwise → use `none`

`sdd` is NEVER used by default — only when the orchestrator explicitly passes `sdd`.

When falling back to `none`, recommend the user enable `engram` or `sdd` for better results.

## Behavior Per Mode

| Mode | Read from | Write to | Project files |
|------|-----------|----------|---------------|
| `engram` | Engram (see `engram-convention.md`) | Engram | Never |
| `sdd` | Filesystem (see `sdd-convention.md`) | Filesystem | Yes |
| `none` | Orchestrator prompt context | Nowhere | Never |

## Common Rules

- If mode is `none`, do NOT create or modify any project files. Return results inline only.
- If mode is `engram`, do NOT write any project files. Persist to Engram and return observation IDs.
- If mode is `sdd`, write files ONLY to the paths defined in `sdd-convention.md`.
- NEVER force `sdd/` creation unless the orchestrator explicitly passed `sdd` mode.
- If you are unsure which mode to use, default to `none`.

## Detail Level

The orchestrator may also pass `detail_level`: `concise | standard | deep`.
This controls output verbosity but does NOT affect what gets persisted — always persist the full artifact.
