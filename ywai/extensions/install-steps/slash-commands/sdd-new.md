---
description: Start a new SDD change — runs exploration then creates a proposal
agent: sdd-orchestrator
---

Follow the SDD orchestrator workflow for starting a new change named "{argument}".

WORKFLOW:
Launch these sub-agents in sequence. If a native sub-agent is not registered for a step, read the matching skill file from the FIRST existing path below and follow it inline:
- `~/.claude/skills/{skill}/SKILL.md`
- `~/.config/opencode/skills/{skill}/SKILL.md`
- `~/.gemini/skills/{skill}/SKILL.md`
- `{workdir}/skills/{skill}/SKILL.md`

1. `sdd-explore` — investigate the codebase for this change
2. Present the exploration summary to the user
3. `sdd-propose` — create a proposal based on the exploration
4. Present the proposal summary and ask the user if they want to continue with specs and design

CONTEXT:
- Working directory: {workdir}
- Current project: {project}
- Change name: {argument}
- Artifact store mode: engram

ENGRAM PERSISTENCE (artifact store mode: engram):
Sub-agents handle persistence automatically. Each phase saves its artifact with `topic_key`:
- `sdd/{argument}/exploration` (if sdd-explore persists)
- `sdd/{argument}/proposal` (sdd-propose)

Read the orchestrator instructions to coordinate this workflow. Do NOT execute phase work inline when a native sub-agent is available.
