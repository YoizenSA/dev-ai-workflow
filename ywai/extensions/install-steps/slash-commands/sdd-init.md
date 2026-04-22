---
description: Initialize SDD context — detects project stack and bootstraps persistence backend
agent: sdd-orchestrator
subtask: true
---

If the native `sdd-init` sub-agent is available, delegate this command to it.
Otherwise, locate and read the `sdd-init` skill file from the FIRST existing path below, then follow its instructions inline:
- `~/.claude/skills/sdd-init/SKILL.md`
- `~/.config/opencode/skills/sdd-init/SKILL.md`
- `~/.gemini/skills/sdd-init/SKILL.md`
- `~/.copilot/agents/sdd-init.md`
- `{workdir}/skills/sdd-init/SKILL.md`

CONTEXT:
- Working directory: {workdir}
- Current project: {project}
- Artifact store mode: files / sdd 

TASK:
Initialize Spec-Driven Development in this project. Detect the tech stack, existing conventions, and architecture patterns. Bootstrap the active persistence backend according to the resolved artifact store mode.

ENGRAM PERSISTENCE (artifact store mode: engram):
Save the detected project context for later phases:
  mem_save(title: "sdd/{project}/project-context", topic_key: "sdd/{project}/project-context", type: "config", project: "{project}", content: "{stack, conventions, architecture detected}")
`topic_key` ensures upserts — re-running updates the same observation.

Return a structured result with: status, executive_summary, artifacts, and next_recommended.
