---
description: Guided SDD walkthrough — onboard a user through the full SDD cycle using their real codebase
agent: sdd-orchestrator
subtask: true
---

If the native `sdd-onboard` sub-agent is available, delegate this command to it.
Otherwise, locate and read the `sdd-onboard` skill file from the FIRST existing path below, then follow its instructions inline:
- `~/.claude/skills/sdd-onboard/SKILL.md`
- `~/.config/opencode/skills/sdd-onboard/SKILL.md`
- `~/.gemini/skills/sdd-onboard/SKILL.md`
- `~/.copilot/agents/sdd-onboard.md`
- `{workdir}/skills/sdd-onboard/SKILL.md`

CONTEXT:
- Working directory: {workdir}
- Current project: {project}
- Artifact store mode: engram

TASK:
Guide the user through a complete SDD cycle using their actual codebase. This is a real change with real artifacts, not a toy example. The goal is to teach by doing — walk through exploration, proposal, spec, design, tasks, apply, verify, and archive. Pause between phases and explain what's happening and why.

ENGRAM PERSISTENCE (artifact store mode: engram):
Save onboarding state as you progress so the user can resume later:
  mem_save(title: "sdd-onboard/{project}", topic_key: "sdd-onboard/{project}", type: "architecture", project: "{project}", content: "{current phase, next step, artifacts produced so far}")
`topic_key` ensures upserts — re-running updates the same observation.

Each sub-phase (explore, propose, spec, design, tasks, apply, verify, archive) will persist its own artifact per the normal SDD conventions — do not duplicate them here; only track onboarding progress.

Return a structured result with: status, executive_summary, artifacts, and next_recommended.
