---
description: Implement SDD tasks — writes code following specs and design (TDD-aware)
agent: sdd-orchestrator
subtask: true
---

If the native `sdd-apply` sub-agent is available, delegate this command to it.
Otherwise, locate and read the `sdd-apply` skill file from the FIRST existing path below, then follow its instructions inline:
- `~/.claude/skills/sdd-apply/SKILL.md`
- `~/.config/opencode/skills/sdd-apply/SKILL.md`
- `~/.gemini/skills/sdd-apply/SKILL.md`
- `~/.copilot/agents/sdd-apply.md`
- `{workdir}/skills/sdd-apply/SKILL.md`

The `sdd-apply` skill (v3.1) supports three modes:
- **Standard**: TDD off — implement straight from specs.
- **Light TDD**: RED → GREEN → REFACTOR per `[RED]/[GREEN]/[REFACTOR]` task triplets in `tasks.md`.
- **Strict TDD**: load `skills/sdd-apply/strict-tdd.md` for Safety Net + Triangulation + Assertion Quality Rules + mandatory TDD Cycle Evidence table. The skill resolves the mode automatically based on `sdd/config.yaml` (`rules.apply.tdd` / `rules.apply.strict_tdd`) or project conventions.

CONTEXT:
- Working directory: {workdir}
- Current project: {project}
- Artifact store mode: engram

TASK:
Implement the remaining incomplete tasks for the active SDD change.

ENGRAM PERSISTENCE (artifact store mode: engram):
CRITICAL: `mem_search` returns 300-char PREVIEWS, not full content. You MUST call `mem_get_observation(id)` for EVERY artifact.

STEP A — SEARCH (get IDs only):
  mem_search(query: "sdd/{change-name}/spec", project: "{project}")   → save spec_id
  mem_search(query: "sdd/{change-name}/design", project: "{project}") → save design_id
  mem_search(query: "sdd/{change-name}/tasks", project: "{project}")  → save tasks_id (KEEP for updates)

STEP A2 — CHECK PREVIOUS PROGRESS (before starting work):
  mem_search(query: "sdd/{change-name}/apply-progress", project: "{project}") → if found, save progress_id

STEP B — RETRIEVE FULL CONTENT (mandatory):
  mem_get_observation(id: spec_id)   → full spec
  mem_get_observation(id: design_id) → full design
  mem_get_observation(id: tasks_id)  → full tasks
  IF progress_id exists: mem_get_observation(id: progress_id) → previous progress; skip completed tasks and MERGE when saving

Update tasks as you complete them:
  mem_update(id: tasks_id, content: "{updated tasks with [x] marks}")

Save progress (MERGE with any prior progress_id content — never overwrite completed work):
  mem_save(title: "sdd/{change-name}/apply-progress", topic_key: "sdd/{change-name}/apply-progress", type: "architecture", project: "{project}", content: "{cumulative progress report}")

For each task:
1. Read the relevant spec scenarios (acceptance criteria)
2. Read the design decisions (technical approach)
3. Read existing code patterns in the project
4. If Strict TDD is active: follow the Safety Net + RED → GREEN → TRIANGULATE → REFACTOR cycle from `strict-tdd.md`. If Light TDD: RED → GREEN → REFACTOR. Otherwise: write the code directly.
5. Mark the task as complete `[x]`

Return a structured result with: status, executive_summary, detailed_report (files changed), artifacts, next_recommended. If Strict TDD Mode was active, INCLUDE the TDD Cycle Evidence table — `sdd-verify` will reject the change without it.
