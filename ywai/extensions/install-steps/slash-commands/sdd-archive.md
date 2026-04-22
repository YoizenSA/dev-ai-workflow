---
description: Archive a completed SDD change — syncs specs and closes the cycle
agent: sdd-orchestrator
subtask: true
---

If the native `sdd-archive` sub-agent is available, delegate this command to it.
Otherwise, locate and read the `sdd-archive` skill file from the FIRST existing path below, then follow its instructions inline:
- `~/.claude/skills/sdd-archive/SKILL.md`
- `~/.config/opencode/skills/sdd-archive/SKILL.md`
- `~/.gemini/skills/sdd-archive/SKILL.md`
- `~/.copilot/agents/sdd-archive.md`
- `{workdir}/skills/sdd-archive/SKILL.md`

CONTEXT:
- Working directory: {workdir}
- Current project: {project}
- Artifact store mode: files / sdd 

TASK:
Archive the active SDD change. Read the verification report first to confirm the change is PASS or PASS WITH WARNINGS.

ENGRAM PERSISTENCE (artifact store mode: engram):
CRITICAL: `mem_search` returns 300-char PREVIEWS, not full content. You MUST call `mem_get_observation(id)` for EVERY artifact.

STEP A — SEARCH (get IDs only):
  mem_search(query: "sdd/{change-name}/proposal", project: "{project}")      → save proposal_id
  mem_search(query: "sdd/{change-name}/spec", project: "{project}")          → save spec_id
  mem_search(query: "sdd/{change-name}/design", project: "{project}")        → save design_id
  mem_search(query: "sdd/{change-name}/tasks", project: "{project}")         → save tasks_id
  mem_search(query: "sdd/{change-name}/apply-progress", project: "{project}") → save progress_id (if exists)
  mem_search(query: "sdd/{change-name}/verify-report", project: "{project}") → save verify_id

STEP B — RETRIEVE FULL CONTENT (mandatory):
  mem_get_observation(id: proposal_id) → full proposal
  mem_get_observation(id: spec_id)     → full spec
  mem_get_observation(id: design_id)   → full design
  mem_get_observation(id: tasks_id)    → full tasks
  IF progress_id exists: mem_get_observation(id: progress_id) → full apply-progress
  mem_get_observation(id: verify_id)   → full verification report

Record ALL observation IDs in the archive report for traceability.

Save:
  mem_save(title: "sdd/{change-name}/archive-report", topic_key: "sdd/{change-name}/archive-report", type: "architecture", project: "{project}", content: "{archive report with observation IDs}")

Then:
1. Sync delta specs into main specs (source of truth).
2. Move the change folder to archive with date prefix (filesystem mode) or mark as archived in engram.
3. Verify the archive is complete.

Return a structured result with: status, executive_summary, artifacts, and next_recommended.
