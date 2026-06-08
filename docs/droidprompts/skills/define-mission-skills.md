Skill "define-mission-skills" is now active.

<skill name="define-mission-skills" filePath="builtin:define-mission-skills">
# Designing Your Worker System

Your job is to design a system of workers that will produce complete, high-quality work.

## Step 1: Analyze Effective Work Boundaries

Ask yourself:
- What distinct layers or domains does this mission touch?
- Do different areas benefit from different procedures or tools?

Each distinct boundary typically maps to a worker type.

## Step 2: Design Worker Types

For each boundary, determine:
- What skills/tools are essential for doing thorough work in this area?
- How does it verify its work? (TDD + manual verification)
- What does a thorough handoff look like?

## Automatic Validation (Builtin)

The system automatically injects two validation features when a milestone completes:

1. **scrutiny-validator** — Runs validators, spawns review subagents for each completed feature, synthesizes findings. If it fails, goes back to pending for re-run after fixes.
2. **user-testing-validator** — Determines testable assertions from `fulfills`, sets up environment, spawns flow validator subagents, synthesizes results. If it fails, goes back to pending for re-run after fixes.

You do NOT create these yourself — they are auto-injected by the system.

## Guiding Principles

1. **Procedural Clarity** - There should be no important ambiguity about what to do, in what order, and with what.

2. **Test-Driven Development** - Tests are written before implementation, always. Workers write failing tests first (red), then implement to make them pass (green).

3. **Manual Verification** - Automated tests are necessary but not sufficient. Workers must manually verify their work catches issues tests miss.

4. **No orphaned processes** - Workers must not leave any test runners or other processes running:
  - Avoid watch/interactive modes for tests unless explicitly required.
  - If a test command starts a long-running process (e.g., watch mode, browser runner), the worker must stop it and ensure any child processes they started are also terminated (by PID, not by name).
---

## Creating Worker Skills

For each worker type, create a skill in missionDir:

```
skills/{worker-type}/SKILL.md
```

**IMPORTANT:** Skills go in missionDir, NOT in any repository `.factory/` directory. Mission sessions load skills from `{missionDir}/skills/`.

### Worker Skill Structure

Every worker skill MUST include:

1. **YAML frontmatter** - name and description
2. **Required Skills and Tools** - skills and tools workers of this type must use during their work. Include anything the user or the mission finalized as binding. "None" if not applicable.
3. **Work Procedure** - step-by-step process. Be specific about required skills/tools.
4. **Example Handoff** - a complete, realistic handoff showing what thorough work looks like
5. **When to Return to Orchestrator** - skill-specific conditions

```markdown
---
name: { worker-type }
description: { One-line description }
---

# {Worker Type}

NOTE: Startup and cleanup are handled by `worker-base`. This skill defines the WORK PROCEDURE.

## Required Skills and Tools

{Skills and tools workers of this type must use during their work. Include anything the user or the mission finalized as binding.}

## Work Procedure

{Step-by-step procedure - testing, implementation, verification. Be specific about tools, commands, and what thorough work looks like at each step.}

## Example Handoff

{A complete JSON example showing what a thorough handoff looks like for this worker type}

## When to Return to Orchestrator

{Skill-specific conditions beyond standard cases}
```

**The Example Handoff defines the upper bound of worker effort.** Workers pattern-match against it; the effort you show is the effort you'll get back. Write the example with the depth the worker's scope warrants, covering the full breadth of responsibilities in the Work Procedure. Keep it grounded in what a real, thorough handoff for this worker would contain.

**Handoff fields** (used by EndFeatureRun tool):

| Field                             | Purpose                                                |
| --------------------------------- | ------------------------------------------------------ |
| `salientSummary`                  | 1–4 sentence summary of what happened in the session   |
| `whatWasImplemented`              | Concrete description of what was built (min 50 chars)  |
| `whatWasLeftUndone`               | What's incomplete - empty string if truly done         |
| `verification.commandsRun`        | Shell commands with `{command, exitCode, observation}` |
| `verification.interactiveChecks`  | UI/browser checks with `{action, observed}` |
| `tests.added`                     | Test files with `{file, cases: [{name, description}]}`. `name` matches the test runner identifier (e.g., the string in `it(...)`, or the test function name). `description` is prose about what the test checks. |
| `discoveredIssues`                | Issues found: `{severity, description, suggestedFix?}` |

Examples of good `salientSummary` (be concrete, 1–4 sentences):
- Success: "Implemented GET /api/products/search with cursor pagination + min-length validation; ran `npm test -- --grep 'product search'` (4 passing) and verified 400 on `q=a` plus 200 on a real curl request."
- Failure: "Tried to wire logout to `SessionStore`, but `bun run typecheck` failed (missing import) and `bun test auth` had 2 failing tests; returning to orchestrator to decide whether to add session persistence or change logout semantics."

## When to Return to Orchestrator

- Feature depends on an API endpoint or data model that doesn't exist yet
- Requirements are ambiguous or contradictory
- Existing bugs affect this feature
````

---

## Checklist

Before proceeding to create mission artifacts:

- [ ] Each worker skill exists at `{missionDir}/skills/{worker-type}/SKILL.md`
- [ ] Each skill has YAML frontmatter (name, description)
- [ ] Each skill has an Example Handoff section with a complete, realistic JSON example
- [ ] Example handoffs are thorough and explicit - they set the quality bar workers will follow
- [ ] Each skill's Required Skills and Tools section includes every skill and tool the worker must use
- [ ] Each skill's Work Procedure ends with a programmatic verification step that reflects the user-approved Programmatic Validation Plan
</skill>