---
description: Continue the next SDD phase in the dependency chain
agent: sdd-orchestrator
---

Follow the SDD orchestrator workflow to continue the active change.

WORKFLOW:
1. Check which artifacts already exist for the active change (proposal, specs, design, tasks, apply-progress, verify-report).
2. Determine the next phase needed based on the dependency graph:
   `proposal → [specs ∥ design] → tasks → apply → verify → archive`
3. Launch the appropriate sub-agent for the next phase. If a native sub-agent is not registered, read the matching skill file from the FIRST existing path below and follow it inline:
   - `~/.claude/skills/{skill}/SKILL.md`
   - `~/.config/opencode/skills/{skill}/SKILL.md`
   - `~/.gemini/skills/{skill}/SKILL.md`
   - `{workdir}/skills/{skill}/SKILL.md`
4. Present the result and ask the user to proceed.

CONTEXT:
- Working directory: {workdir}
- Current project: {project}
- Change name: {argument}
- Artifact store mode: engram

ENGRAM PERSISTENCE (artifact store mode: engram):
CRITICAL: `mem_search` returns 300-char PREVIEWS, not full content. You MUST call `mem_get_observation(id)` for EVERY artifact you intend to read.

STEP A — SEARCH (get IDs only):
  mem_search(query: "sdd/{argument}/proposal", project: "{project}") → proposal_id (if exists)
  mem_search(query: "sdd/{argument}/spec", project: "{project}") → spec_id (if exists)
  mem_search(query: "sdd/{argument}/design", project: "{project}") → design_id (if exists)
  mem_search(query: "sdd/{argument}/tasks", project: "{project}") → tasks_id (if exists)
  mem_search(query: "sdd/{argument}/apply-progress", project: "{project}") → progress_id (if exists)
  mem_search(query: "sdd/{argument}/verify-report", project: "{project}") → verify_id (if exists)

STEP B — Use the presence/absence of each ID to decide the next phase. Only call `mem_get_observation(id)` for the artifacts the chosen sub-agent requires (see that sub-agent's skill file for its specific requirements).

Read the orchestrator instructions to coordinate this workflow. Do NOT execute phase work inline when a native sub-agent is available.
