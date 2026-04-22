---
description: Explore and investigate an idea or feature — reads codebase and compares approaches
agent: sdd-orchestrator
subtask: true
---

If the native `sdd-explore` sub-agent is available, delegate this command to it.
Otherwise, locate and read the `sdd-explore` skill file from the FIRST existing path below, then follow its instructions inline:
- `~/.claude/skills/sdd-explore/SKILL.md`
- `~/.config/opencode/skills/sdd-explore/SKILL.md`
- `~/.gemini/skills/sdd-explore/SKILL.md`
- `~/.copilot/agents/sdd-explore.md`
- `{workdir}/skills/sdd-explore/SKILL.md`

CONTEXT:
- Working directory: {workdir}
- Current project: {project}
- Topic to explore: {argument}
- Artifact store mode: files / sdd 

TASK:
Explore the topic "{argument}" in this codebase. Investigate the current state, identify affected areas, compare approaches, and provide a recommendation.

This is an exploration only — do NOT create any files or modify code. Just research and return your analysis.

ENGRAM PERSISTENCE (artifact store mode: engram):
Persist ONLY when this exploration is tied to a named change. If so, save:
  mem_save(title: "sdd/{argument}/exploration", topic_key: "sdd/{argument}/exploration", type: "architecture", project: "{project}", content: "{exploration report}")
`topic_key` ensures upserts — re-running updates the same observation.

Return a structured result with: status, executive_summary, detailed_report, artifacts, and next_recommended.
