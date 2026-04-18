# global-agents

Source templates and bundle config for global user-profile agents
(OpenCode / GitHub Copilot / Gemini / Cursor / Claude).

## Scope

- Templates in `templates/*.md` are the canonical source for global agent
  content. `AGENTS.md` from project types is intentionally NOT used.
- Bundles in `bundles.json` map each global agent to its skills.
- Project-type agent lists live in `ywai/types/types.json`
  (`types.<type>.global_agents`).

## How it runs

All entry points delegate to the same generator in the `ywai` binary
(`pkg/installer/globalagents/`):

- `ywai --update-global-agents --type=<type>`  (cross-platform, preferred)
- `bash ywai/skills/setup.sh --global-only --project-type=<type>`
- `powershell ywai/skills/setup.ps1 -GlobalOnly -ProjectType <type>`
- `bash ywai/extensions/install-steps/global-agents/install.sh` (via wizard or extensions flow)
- `powershell ywai/extensions/install-steps/global-agents/install.ps1`

Shell scripts retain a fallback that copies templates directly when the
binary is not on PATH.

## Template location

`templates/<agent-name>.md`

Supported names:
- `sdd-orchestrator`
- `fe-engineer`
- `nest-engineer`
- `dotnet-engineer`
- `qa-playwright`
- `devops`

Templates expose: `## Role`, `## Priorities`, `## Operating rules`,
`## Agent focus`. They MUST NOT hardcode `## Skills invoke` lists - the
generator renders those dynamically from `bundles.json` + `SKILL.md`
metadata.

## Agent-Skills bundles

`bundles.json`:

- `defaults.<agent>`: default bundle for any project type.
- `by_project_type.<type>.<agent>`: ONLY for real overrides
  (never duplicate `defaults`).

Example:

- `devops` agent -> `devops` skill
- `sdd-orchestrator` agent -> full SDD skill set (`sdd-init` ... `sdd-archive`)

During generation each file gets:

- Target-specific frontmatter (OpenCode / Copilot prompt / Copilot agent / Gemini / Cursor / Claude).
- `## Base directives (from extensions)` (template body without frontmatter).
- `## Skills bundle (global)`.
- `## Skills invoke` using `metadata.auto_invoke` from each `skills/*/SKILL.md` (first 3 patterns joined with ` | `; generic fallback if absent).
- `## SDD quick commands` if the bundle includes any `sdd-*` skill.
- `## DevOps trigger keywords` if the bundle includes `devops`.

## Destinations

| Target         | Path                                              |
|:---------------|:--------------------------------------------------|
| OpenCode       | `$XDG_CONFIG_HOME/opencode/agent/<agent>.md`       |
| Copilot agent  | `~/.copilot/agents/<agent>.md`                    |
| Copilot prompt | `<VSCode User>/prompts/<agent>.instructions.md`   |
| Gemini         | `~/.gemini/agents/<agent>.md`                     |
| Cursor         | `~/.cursor/agents/<agent>.md`                     |
| Claude         | `~/.claude/agents/<agent>.md`                     |

## File preservation

The generator only overwrites / removes files whose basenames match a
template (e.g. `devops.md`, `devops.instructions.md`). Any other `.md`
file in a destination directory is considered user-owned and is left
intact across re-runs.
