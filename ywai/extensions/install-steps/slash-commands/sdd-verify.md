---
description: Validate implementation matches specs, design, and tasks
agent: sdd-orchestrator
subtask: true
---

If the native `sdd-verify` sub-agent is available, delegate this command to it.
Otherwise, locate and read the `sdd-verify` skill file from the FIRST existing path below, then follow its instructions inline:
- `~/.claude/skills/sdd-verify/SKILL.md`
- `~/.config/opencode/skills/sdd-verify/SKILL.md`
- `~/.gemini/skills/sdd-verify/SKILL.md`
- `~/.copilot/agents/sdd-verify.md`
- `{workdir}/skills/sdd-verify/SKILL.md`

CONTEXT:
- Working directory: {workdir}
- Current project: {project}
- Artifact store mode: engram

TASK:
Verify the active SDD change.

ENGRAM PERSISTENCE (artifact store mode: engram):
CRITICAL: `mem_search` returns 300-char PREVIEWS, not full content. You MUST call `mem_get_observation(id)` for EVERY artifact.

STEP A — SEARCH (get IDs only):
  mem_search(query: "sdd/{change-name}/spec", project: "{project}")           → save spec_id
  mem_search(query: "sdd/{change-name}/design", project: "{project}")         → save design_id
  mem_search(query: "sdd/{change-name}/tasks", project: "{project}")          → save tasks_id
  mem_search(query: "sdd/{change-name}/apply-progress", project: "{project}") → save progress_id (if exists)

STEP B — RETRIEVE FULL CONTENT (mandatory):
  mem_get_observation(id: spec_id)     → full spec
  mem_get_observation(id: design_id)   → full design
  mem_get_observation(id: tasks_id)    → full tasks
  IF progress_id exists: mem_get_observation(id: progress_id) → full apply-progress (and TDD Cycle Evidence table if present)

Save report:
  mem_save(title: "sdd/{change-name}/verify-report", topic_key: "sdd/{change-name}/verify-report", type: "architecture", project: "{project}", content: "{verification report}")

Then:
1. Check completeness — are all tasks marked `[x]`?
2. Check correctness — does code match the specs?
3. Check coherence — were design decisions followed?
4. Run tests and build (real execution — not just "looks right").
5. Build the spec compliance matrix.
6. **Strict TDD gate**: if Strict TDD Mode was active during `sdd-apply`, the apply-progress MUST include a complete **TDD Cycle Evidence** table with RED/GREEN/TRIANGULATE/REFACTOR columns for every task. If the table is missing or incomplete, fail verification with reason "Strict TDD evidence missing/incomplete".

Return a structured verification report with: status, executive_summary, detailed_report, artifacts, and next_recommended.
