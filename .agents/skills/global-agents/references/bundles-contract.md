# Global Agents Bundles Contract

## Files

- Templates: `ywai/extensions/install-steps/global-agents/templates/<agent>.md`
- Bundles: `ywai/extensions/install-steps/global-agents/bundles.json`
- Project-type agents: `ywai/types/types.json` (`types.<type>.global_agents`)
- Generator (canonical): `ywai/setup/wizard/pkg/installer/globalagents/`

## JSON schema (practical)

```json
{
  "defaults": {
    "<agent-name>": ["<skill>", "<skill>"]
  },
  "by_project_type": {
    "<type>": {
      "<agent-name>": ["<skill>"]
    }
  }
}
```

## Rules

1. Keep `defaults` complete for each global agent role.
2. Add `by_project_type.<type>.<agent>` ONLY when the bundle differs from `defaults`. Never duplicate defaults.
3. Skill names must match `skills/<name>/SKILL.md` directories.
4. Generated global agents expose (rendered by the generator, not by templates):
   - `## Skills bundle (global)`
   - `## Skills invoke` (uses `metadata.auto_invoke` from each `SKILL.md`, first 3 patterns joined with ` | `)
   - `## SDD quick commands` (only if the bundle contains any `sdd-*` skill)
   - `## DevOps trigger keywords` (only if the bundle contains the `devops` skill)
5. Templates contain: base frontmatter (optional), `## Role`, `## Priorities`, `## Operating rules`, `## Agent focus`. No skills-invoke lists.
6. Never source global agent directives from project `AGENTS.md`.

## Managed-file policy

- The generator only owns file basenames matching a template (`<agent>.md` for most targets, `<agent>.instructions.md` for Copilot prompts).
- Any other `.md` in a destination directory is preserved across runs.
- This prevents user-authored agents from being wiped on reinstall.

## Target destinations

| Target         | Path                                                |
|:---------------|:----------------------------------------------------|
| OpenCode       | `$XDG_CONFIG_HOME/opencode/agent/<agent>.md`         |
| Copilot agent  | `~/.copilot/agents/<agent>.md`                      |
| Copilot prompt | `<VSCode User>/prompts/<agent>.instructions.md`     |
| Gemini         | `~/.gemini/agents/<agent>.md`                       |
| Cursor         | `~/.cursor/agents/<agent>.md`                       |
| Claude         | `~/.claude/agents/<agent>.md`                       |

## Example

- `devops` agent bundle: `["devops"]`
- `sdd-orchestrator` bundle: full SDD chain (`sdd-init` ... `sdd-archive`)
- A real override example (hypothetical): `by_project_type.dotnet.devops = ["devops", "biome"]`
