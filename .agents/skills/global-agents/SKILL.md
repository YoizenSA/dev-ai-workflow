---
name: global-agents
description: >
  Create and maintain global user-profile agents (OpenCode/Copilot) with extension-based templates,
  Agent-Skills bundles, and invoke sync hints.
  Trigger: When the user asks to add/update global agents, bundles.json, or skills invoke behavior.
license: Apache-2.0
metadata:
  author: Yoizen
  version: "1.0"
  scope: [root]
  auto_invoke:
    - "global agents"
    - "bundles"
    - "invoke sync"
---

## When to Use

Use this skill when:
- Creating a new global agent role (template + bundle mapping)
- Updating `ywai/extensions/install-steps/global-agents/templates/*.md`
- Updating `ywai/extensions/install-steps/global-agents/bundles.json`
- Aligning generated `Skills bundle` / `Skills invoke` sections in global profiles

---

## Critical Patterns

### Pattern 1: Source of truth lives in extensions + Go generator

- Templates: `ywai/extensions/install-steps/global-agents/templates/<agent>.md`
- Bundles: `ywai/extensions/install-steps/global-agents/bundles.json`
- Project-type -> agents: `ywai/types/types.json` (`types.<type>.global_agents`)
- Generator: `ywai/setup/wizard/pkg/installer/globalagents/` (Go, cross-platform)
- `setup.sh --global-only`, `setup.ps1 -GlobalOnly`, `extensions/.../install.sh|ps1` all delegate to the Go binary (`ywai --update-global-agents --type=<type>`) and keep shell fallbacks for environments without the binary.
- Do NOT use project `AGENTS.md` as global-agent content source.

### Pattern 2: Agent <-> Skills bundle contract

- Every global agent has an entry in `bundles.json:defaults`.
- `by_project_type.<type>.<agent>` should contain ONLY real overrides (do not duplicate defaults).
- Skill names must match `skills/<name>/SKILL.md` directories.

### Pattern 3: Invoke hints come from skill metadata

- `Skills invoke` is derived from each skill's `metadata.auto_invoke` (first 3 patterns, joined with ` | `).
- When a skill has no `auto_invoke`, a generic "when its domain is required" fallback is emitted.
- Templates MUST NOT hardcode `## Skills invoke` lists; the generator renders that section.

### Pattern 4: Managed-file policy (preserve user files)

- Only files whose basenames match a template (e.g. `devops.md`, `sdd-orchestrator.md`) are owned by the generator.
- Any other `.md` in the destination directory is considered user-owned and is never removed.
- This is implemented in `globalagents.Generator.InstallAll` via `managedBasenames` + `removeManagedFiles`.

---

## Destinations

| Target         | Path                                                              |
|:---------------|:------------------------------------------------------------------|
| OpenCode       | `$XDG_CONFIG_HOME/opencode/agent/<agent>.md` (singular `agent/`)   |
| Copilot agent  | `~/.copilot/agents/<agent>.md`                                    |
| Copilot prompt | `<VSCode User>/prompts/<agent>.instructions.md`                   |
| Gemini         | `~/.gemini/agents/<agent>.md`                                     |
| Cursor         | `~/.cursor/agents/<agent>.md`                                     |
| Claude         | `~/.claude/agents/<agent>.md`                                     |

`<VSCode User>` resolves to `~/Library/Application Support/Code/User` (macOS), `$APPDATA\Code\User` (Windows), or `$XDG_CONFIG_HOME/Code/User` (Linux).

---

## Workflow

1. Identify target project types and global agents in `ywai/types/types.json:types.<type>.global_agents`.
2. Update templates in `ywai/extensions/install-steps/global-agents/templates/`.
3. Update `bundles.json` (only real overrides in `by_project_type`).
4. Run the generator:
   - `ywai --update-global-agents --type=<type>`  (preferred, cross-platform)
   - `bash ywai/skills/setup.sh --global-only --project-type=<type>`  (bash)
   - `powershell ywai/skills/setup.ps1 -GlobalOnly -ProjectType <type>`  (Windows)
5. Inspect a generated file to verify frontmatter, base directives, Skills bundle + invoke, and SDD/DevOps sections.

---

## Commands

```bash
# Validate shell scripts
bash -n ywai/skills/setup.sh
bash -n ywai/extensions/install-steps/global-agents/install.sh

# Generator unit tests (Go)
cd ywai/setup/wizard && go test ./pkg/installer/globalagents/...

# Smoke test via the Go binary (writes to user's real HOME; redirect XDG for isolation)
tmpdir="$(mktemp -d)"
XDG_CONFIG_HOME="$tmpdir/xdg" HOME="$tmpdir/home" \
  ywai --update-global-agents --type=devops --silent
sed -n '1,140p' "$tmpdir/xdg/opencode/agent/devops.md"
```

---

## Resources

- **Contract**: [references/bundles-contract.md](references/bundles-contract.md)
- **Templates**: `ywai/extensions/install-steps/global-agents/templates/`
- **Bundles**: `ywai/extensions/install-steps/global-agents/bundles.json`
- **Generator (canonical)**: `ywai/setup/wizard/pkg/installer/globalagents/`
- **Generator (bash fallback)**: `ywai/skills/setup.sh` (`setup_global_profile_agents`)
