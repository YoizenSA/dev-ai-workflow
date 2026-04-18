# SDD File Convention (shared across all SDD skills)

## Directory Structure

```
sdd/
├── config.yaml              <- Project-specific SDD config
├── specs/                   <- Source of truth (main specs)
│   └── {domain}/
│       └── spec.md
└── changes/                 <- Active changes
    ├── archive/             <- Completed changes (YYYY-MM-DD-{change-name}/)
    └── {change-name}/       <- Active change folder
        ├── exploration.md   <- (optional) from sdd-explore
        ├── proposal.md      <- from sdd-propose
        ├── specs/           <- from sdd-spec
        │   └── {domain}/
        │       └── spec.md  <- Delta spec
        ├── design.md        <- from sdd-design
        ├── tasks.md         <- from sdd-tasks (updated by sdd-apply)
        └── verify-report.md <- from sdd-verify
```

## Artifact File Paths

| Skill | Creates / Reads | Path |
|-------|----------------|------|
| sdd-init | Creates | `sdd/config.yaml`, `sdd/specs/`, `sdd/changes/`, `sdd/changes/archive/` |
| sdd-explore | Creates (optional) | `sdd/changes/{change-name}/exploration.md` |
| sdd-propose | Creates | `sdd/changes/{change-name}/proposal.md` |
| sdd-spec | Creates | `sdd/changes/{change-name}/specs/{domain}/spec.md` |
| sdd-design | Creates | `sdd/changes/{change-name}/design.md` |
| sdd-tasks | Creates | `sdd/changes/{change-name}/tasks.md` |
| sdd-apply | Updates | `sdd/changes/{change-name}/tasks.md` (marks `[x]`) |
| sdd-verify | Creates | `sdd/changes/{change-name}/verify-report.md` |
| sdd-archive | Moves | `sdd/changes/{change-name}/` → `sdd/changes/archive/YYYY-MM-DD-{change-name}/` |
| sdd-archive | Updates | `sdd/specs/{domain}/spec.md` (merges deltas into main specs) |

## Reading Artifacts

Each skill reads its dependencies from the filesystem:

```
Proposal:  sdd/changes/{change-name}/proposal.md
Specs:     sdd/changes/{change-name}/specs/  (all domain subdirectories)
Design:    sdd/changes/{change-name}/design.md
Tasks:     sdd/changes/{change-name}/tasks.md
Verify:    sdd/changes/{change-name}/verify-report.md
Config:    sdd/config.yaml
Main specs: sdd/specs/{domain}/spec.md
```

## Writing Rules

- ALWAYS create the change directory (`sdd/changes/{change-name}/`) before writing artifacts
- If a file already exists, READ it first and UPDATE it (don't overwrite blindly)
- If the change directory already exists with artifacts, the change is being CONTINUED
- Use the `sdd/config.yaml` `rules` section to apply project-specific constraints per phase

## Config File Reference

```yaml
# sdd/config.yaml
schema: spec-driven

context: |
  Tech stack: {detected}
  Architecture: {detected}
  Testing: {detected}
  Style: {detected}

# Model assignment per SDD phase (optional)
# Use this to assign different AI models to different phases
# Powerful models for design, fast/cheap models for implementation
models:
  default: ""                              # Default model (empty = use agent's default)
  sdd-explore: "anthropic/claude-sonnet-4" # Powerful for analysis
  sdd-propose: ""                          # Use default
  sdd-spec: "anthropic/claude-sonnet-4"    # Powerful for specs
  sdd-design: "anthropic/claude-sonnet-4"  # Powerful for architecture
  sdd-tasks: ""                            # Use default
  sdd-apply: "openrouter/qwen/qwen3-30b"   # Fast/cheap for implementation
  sdd-verify: ""                           # Use default

rules:
  proposal:
    - Include rollback plan for risky changes
  specs:
    - Use Given/When/Then for scenarios
    - Use RFC 2119 keywords (MUST, SHALL, SHOULD, MAY)
  design:
    - Include sequence diagrams for complex flows
    - Document architecture decisions with rationale
  tasks:
    - Group by phase, use hierarchical numbering
    - Keep tasks completable in one session
  apply:
    - Follow existing code patterns
    tdd: false           # Set to true to enable RED-GREEN-REFACTOR
    test_command: ""     # e.g., "npm test", "pytest"
  verify:
    test_command: ""     # Override for verification
    build_command: ""    # Override for build check
    coverage_threshold: 0  # Set > 0 to enable coverage check
  archive:
    - Warn before merging destructive deltas
```

## Archive Structure

When archiving, the change folder moves to:
```
sdd/changes/archive/YYYY-MM-DD-{change-name}/
```

Use today's date in ISO format. The archive is an AUDIT TRAIL — never delete or modify archived changes.
