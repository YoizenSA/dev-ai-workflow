---
name: sdd-archive
description: >
  Sync delta specs to main specs and archive a completed change.
  Trigger: "archive", "archivar", "close change", "cerrar cambio",
  "sdd archive", "finalizar", "merge specs", "/sdd:archive".

metadata:
  author: Yoizen
  version: "3.0"
  scope: [root]
---

## Purpose

You are a sub-agent responsible for ARCHIVING. You merge delta specs into the main specs (source of truth), then close the change. You complete the SDD cycle.

## What You Receive

From the orchestrator:
- Change name
- The verification report (confirm the change is PASS or PASS WITH WARNINGS)
- The full change folder contents
- Artifact store mode (`engram | sdd | none`)

## Execution and Persistence Contract

Read and follow `skills/_shared/persistence-contract.md` for mode resolution rules.

- If mode is `engram`: Read and follow `skills/_shared/engram-convention.md`. Retrieve ALL prior artifacts by topic_key to collect observation IDs for the lineage record. Artifact type: `archive-report`. Do NOT move any project files.
- If mode is `sdd`: Read and follow `skills/_shared/sdd-convention.md`. Perform spec merge and move change folder. Create archive report as part of the move.
- If mode is `none`: Return the full archive summary inline. Do NOT create or move any project files.

## What to Do

### Step 1: Confirm Readiness

Read the verification report:
- **engram**: 2-step recovery — `mem_search(query: "sdd/{change-name}/verify-report")` + `mem_get_observation`
- **sdd**: Read `sdd/changes/{change-name}/verify-report.md`
- **none**: Use content passed by orchestrator

If verdict is FAIL (CRITICAL issues present), REFUSE to archive. Return status `blocked` with the list of critical issues.

### Step 2: Sync Delta Specs to Main Specs (sdd mode only)

For each delta spec in `sdd/changes/{change-name}/specs/`:

#### If Main Spec Exists (`sdd/specs/{domain}/spec.md`)

Read the existing main spec and apply the delta:

```
FOR EACH SECTION in delta spec:
├── ADDED Requirements → Append to main spec's Requirements section
├── MODIFIED Requirements → Replace the matching requirement in main spec
└── REMOVED Requirements → Delete the matching requirement from main spec
```

Merge carefully:
- Match requirements by ID (e.g., `REQ-AUTH-001`) or by name
- Preserve all OTHER requirements not in the delta
- Maintain proper Markdown formatting and heading hierarchy

#### If Main Spec Does NOT Exist

Copy the delta spec directly:
```
sdd/changes/{change-name}/specs/{domain}/spec.md
  → sdd/specs/{domain}/spec.md
```

> **engram mode**: The spec content is already in Engram as `sdd/{change-name}/spec`. Note the observation ID in the archive-report as the new source of truth for that domain. No file merge needed.

### Step 3: Capture Lessons Learned

```markdown
# Lessons Learned: {change-name}

## What Went Well
- {Things that worked smoothly in this SDD cycle}

## What Could Improve
- {Friction points, gaps in specs, design mismatches}

## Surprises / Discoveries
- {Unexpected findings during implementation or verification}

## Recommendations for Future Changes
- {Process improvements, missing skills, config adjustments}
```

> If no notable lessons (simple change, everything smooth), write: `No significant lessons — straightforward change.`

### Step 4: Generate Changelog Entry

```markdown
### {Change Title} ({YYYY-MM-DD})

{One-line description of what changed from the user's perspective.}

- {Bullet point of visible change 1}
- {Bullet point of visible change 2}
```

If the project has a `CHANGELOG.md`, append this entry under the appropriate section (Added/Changed/Fixed/Removed).
If no changelog exists, include the entry in the archive summary only.

### Step 5: Collect Metrics

| Metric | Value |
|--------|-------|
| Total tasks | {N} |
| Tasks completed | {N} |
| Phases | {N} |
| Files created | {N} |
| Files modified | {N} |
| Files deleted | {N} |
| Verify verdict | {PASS/PASS WITH WARNINGS/FAIL} |
| Critical issues found | {N} |
| Warnings found | {N} |
| Effort estimate (proposal) | {XS/S/M/L/XL} |

### Step 6: Move to Archive (sdd mode only)

Move the change folder:

```
sdd/changes/{change-name}/
  → sdd/changes/archive/YYYY-MM-DD-{change-name}/
```

Use today's date in ISO format (e.g., `2026-03-02`).

If `sdd/changes/archive/` doesn't exist, create it first.

### Step 7: Collect Engram Artifact Lineage (engram mode)

For engram mode, collect ALL observation IDs for the change to create a complete lineage record:

```
Retrieve IDs via mem_search + mem_get_observation for:
  - sdd/{change-name}/explore        (if exists)
  - sdd/{change-name}/proposal
  - sdd/{change-name}/spec
  - sdd/{change-name}/design
  - sdd/{change-name}/tasks
  - sdd/{change-name}/apply-progress (one or more)
  - sdd/{change-name}/verify-report
```

Include all collected IDs in the `archive-report` content under a **Lineage** section.

### Step 8: Persist Archive Report

- **engram**: `mem_save` with `topic_key: sdd/{change-name}/archive-report` — include lineage section with all observation IDs
- **sdd**: Write `lessons.md` to the change folder (before moving it), then move the folder to archive
- **none**: Return content inline only

### Step 9: Verify Archive

Confirm:
- [ ] Verification report confirmed (PASS or PASS WITH WARNINGS)
- [ ] Delta specs synced (or noted in engram lineage)
- [ ] Lessons learned captured
- [ ] Changelog entry generated
- [ ] Change folder archived (sdd) or archive-report saved (engram)
- [ ] All artifacts present

### Step 10: Return Summary

```markdown
## Change Archived

**Change**: {change-name}
**Date**: {YYYY-MM-DD}
**Persistence**: {engram (archive-report ID: #{id}) | sdd (archive path) | none (inline)}

### Specs Synced
| Domain | Action | Details |
|--------|--------|---------|
| {domain} | Created/Updated | {N added, M modified, K removed requirements} |

### Archive Contents
- proposal.md ✅
- specs/ ✅
- design.md ✅
- tasks.md ✅ ({N}/{N} tasks complete)
- verify-report.md ✅
- lessons.md ✅

### Changelog Entry
```
{The changelog entry generated in Step 4}
```

### Metrics
{The metrics table from Step 5}

### Engram Lineage (engram mode only)
| Artifact | Observation ID |
|----------|----------------|
| explore | #{id or "—"} |
| proposal | #{id} |
| spec | #{id} |
| design | #{id} |
| tasks | #{id} |
| apply-progress | #{id(s)} |
| verify-report | #{id} |
| archive-report | #{id} |

### Source of Truth Updated
{sdd: "The following specs now reflect the new behavior: sdd/specs/{domain}/spec.md"}
{engram: "Spec content is in Engram observation #{spec-id}. Use topic_key sdd/{change-name}/spec to retrieve."}

### SDD Cycle Complete
The change has been fully planned, implemented, verified, and archived.
Ready for the next change.
```

## Error Recovery

| Situation | Action |
|-----------|--------|
| Verification report has CRITICAL issues | REFUSE to archive; return status `blocked` with list of critical issues |
| Verification report is missing | Ask orchestrator to run sdd-verify first; do not archive without verification |
| Delta spec merge conflicts with main spec | Perform manual merge carefully; flag conflicting sections in summary |
| Changelog file doesn't exist | Include changelog entry in return summary only; do not create CHANGELOG.md |
| Archive folder already exists for this date+name | Append suffix: `YYYY-MM-DD-{change-name}-2` |
| Some artifacts are missing (fast-tracked change) | Archive anyway; note missing artifacts; they were intentionally skipped |
| Cannot collect all Engram IDs for lineage | Archive with partial lineage; note which IDs could not be retrieved |

## Rules

- NEVER archive a change that has CRITICAL issues in its verification report
- ALWAYS sync delta specs BEFORE moving to archive (sdd) or record lineage (engram)
- ALWAYS capture lessons learned before archiving
- ALWAYS generate a changelog entry
- When merging into existing specs, PRESERVE requirements not mentioned in the delta
- Use ISO date format (YYYY-MM-DD) for archive folder prefix
- If the merge would be destructive (removing large sections), WARN the orchestrator and ask for confirmation
- The archive is an AUDIT TRAIL — never delete or modify archived changes
- In `none` mode, NEVER create or modify any project files
- Apply any `rules.archive` from `sdd/config.yaml` or the engram project context
- Return a structured envelope with: `status`, `executive_summary`, `detailed_report` (optional), `artifacts`, `next_recommended`, and `risks`
