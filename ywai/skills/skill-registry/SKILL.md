---
name: skill-registry
description: >
  Generate or update the project's compact skill registry — a catalog of every
  installed skill with 5–15 line actionable rules that delegators inject into
  sub-agent prompts. Sub-agents consume the registry instead of loading full
  SKILL.md files, cutting prompt tokens ~40% on multi-skill projects.
  Trigger: After installing/removing skills, when bootstrapping a new project,
  when the user asks to "update skill registry" / "rebuild skill-registry" /
  "regenerate compact rules" / "sync skills".
license: Apache-2.0
metadata:
  author: Yoizen
  version: "1.0"
  scope: [root]
  auto_invoke:
    - "update skill registry"
    - "rebuild skill-registry"
    - "regenerate compact rules"
    - "skill registry"
    - "sync skills"
    - "refresh skills"
allowed-tools: [Read, Edit, Write, Glob, Grep, Bash]
---

## Purpose

Generate or update the **skill registry** — a catalog of every available skill
with **compact rules** (pre-digested, 5–15 line summaries) that any delegator
injects directly into sub-agent prompts. Sub-agents do NOT read individual
`SKILL.md` files — they receive compact rules pre-resolved in their launch
prompt.

This is the foundation of the **Skill Resolver Protocol** (see
`skills/_shared/skill-resolver.md`). The registry is built ONCE (expensive),
then read cheaply at every delegation.

## When to Run

- After installing or removing skills in `skills/` or `~/.<agent>/skills/`.
- After bootstrapping a new project via `sdd-init`.
- When the user explicitly asks to refresh or regenerate the registry.

## What to Do

### Step 1: Scan Skills

Glob `*/SKILL.md` across every known skill directory. Scan ALL that exist, not
just the first match:

**User-level (global skills):**
- `~/.claude/skills/` — Claude Code
- `~/.config/opencode/skills/` — OpenCode
- `~/.gemini/skills/` — Gemini CLI
- `~/.cursor/skills/` — Cursor
- `~/.copilot/skills/` — VS Code Copilot
- The parent directory of this skill file (catch-all for any tool)

**Project-level (workspace skills):**
- `{project-root}/.claude/skills/`
- `{project-root}/.gemini/skills/`
- `{project-root}/.agent/skills/`
- `{project-root}/skills/`

**SKIP** the following directories:
- `sdd-*` — those are SDD workflow skills, not coding/task skills
- `_shared` — shared helpers, not independent skills
- `skill-registry` — this very skill
- `skill-sync` — it generates tables, not compact rules

**Deduplicate:** if the same skill name appears in multiple locations, keep the
project-level version (more specific). If only user-level, keep the first
found.

For each remaining skill, read `SKILL.md` (if it exceeds 200 lines, focus on
the frontmatter and `## Critical Patterns` / `## Rules` sections only) to
extract:
- `name` (frontmatter)
- trigger text from `description` (anything after `Trigger:`)
- Compact rules (see Step 1b)

### Step 1b: Generate Compact Rules

For each skill, emit a **compact rules block** (5–15 lines max) containing
ONLY:
- Actionable rules and constraints (`do X`, `never Y`, `prefer Z over W`).
- Key patterns with one-line examples when critical.
- Breaking changes or gotchas that would cause bugs if missed.

**DO NOT include:** purpose, when-to-use, full code examples, installation
steps, or anything the sub-agent does not need to APPLY the skill.

Format:

```markdown
### {skill-name}
- Rule 1
- Rule 2
- ...
```

**Example — compact rules for a React 19 skill:**

```markdown
### react-19
- No useMemo/useCallback — React Compiler handles memoization
- use() hook for promises/context, replaces useEffect for data fetching
- Server Components by default; add 'use client' only for interactivity/hooks
- ref is a regular prop — no forwardRef needed
- Actions: useActionState for mutations, useOptimistic for optimistic UI
```

**The compact rules are the MOST IMPORTANT output of this skill.** Invest time
making them accurate and concise — the sub-agents will not see anything else.

### Step 2: Scan Project Conventions

Check the project root for convention/index files:

- `AGENTS.md` / `agents.md`
- `CLAUDE.md` (project-level only, NOT `~/.claude/CLAUDE.md`)
- `.cursorrules`
- `GEMINI.md`
- `.github/copilot-instructions.md`

If an **index file** is found (e.g., `AGENTS.md`): READ its contents and
extract every referenced file path. Include the index file AND all paths it
references in the registry — zero extra hops for sub-agents.

For standalone files (`.cursorrules`, `CLAUDE.md`, etc.): record the file
directly.

### Step 3: Write the Registry

Build the registry markdown with this shape:

```markdown
# Skill Registry

**Delegator use only.** Any agent that launches sub-agents reads this registry
to resolve compact rules, then injects matching blocks into sub-agent prompts.
Sub-agents do NOT read this registry or individual SKILL.md files.

See `skills/_shared/skill-resolver.md` for the full resolution protocol.

## Skills

| Trigger | Skill | Path |
|---------|-------|------|
| {trigger} | {name} | {full path to SKILL.md} |
| ... | ... | ... |

## Compact Rules

Pre-digested rules per skill. Delegators copy matching blocks into sub-agent
prompts as `## Project Standards (auto-resolved)`.

### {skill-name-1}
- Rule 1
- Rule 2

### {skill-name-2}
- Rule 1
- Rule 2

## Project Conventions

| File | Path | Notes |
|------|------|-------|
| {index file} | {path} | Index — references files below |
| {referenced file} | {extracted path} | Referenced by {index file} |
| {standalone file} | {path} | |
```

### Step 4: Persist the Registry

**This step is MANDATORY — do NOT skip it.**

#### A. Always write the file (guaranteed availability)

Create `.ywai/` in the project root if it does not exist, then write:

```
.ywai/skill-registry.md
```

Add `.ywai/` to `.gitignore` if the file exists and `.ywai/` is not already
listed.

#### B. If engram is available, also persist cross-session

```
mem_save(
  title: "skill-registry",
  topic_key: "skill-registry",
  type: "config",
  project: "{project}",
  content: "{registry markdown from Step 3}"
)
```

`topic_key` ensures upserts — re-running updates the same observation.

### Step 5: Return Summary

```markdown
## Skill Registry Updated

**Project**: {project name}
**Location**: .ywai/skill-registry.md
**Engram**: {saved / not available}

### Skills Found
| Skill | Trigger |
|-------|---------|
| {name} | {trigger} |

### Project Conventions Found
| File | Path |
|------|------|
| {file} | {path} |

### Next Steps
The orchestrator reads this registry once per session and passes pre-resolved
compact rules to sub-agents via their launch prompts. Re-run this skill after
installing or removing skills.
```

## Rules

- ALWAYS write `.ywai/skill-registry.md` regardless of the active SDD persistence mode.
- ALWAYS save to engram when `mem_save` is available — fall back silently when not.
- SKIP `sdd-*`, `_shared`, `skill-registry`, and `skill-sync` directories when scanning.
- Read `SKILL.md` files (respecting the 200-line guard in Step 1) to generate accurate compact rules — this is a build-time cost, not a runtime cost.
- Compact rules MUST be 5–15 lines per skill — concise, actionable, no fluff.
- Include ALL convention index files found (not just the first) and expand their referenced paths inline.
- If no skills or conventions are found, write an empty registry so sub-agents do not waste time searching.
- Add `.ywai/` to `.gitignore` when a `.gitignore` exists and `.ywai/` is missing.

## Resources

- `skills/skill-sync/assets/sync.sh --registry` — mechanical generator used by CI and by sub-agents; this skill documents the protocol a human/LLM can follow manually when the script is unavailable.
- `skills/_shared/skill-resolver.md` — delegator-side resolution protocol (to be added; consumers of this registry).
