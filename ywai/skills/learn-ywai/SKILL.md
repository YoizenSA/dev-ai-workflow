---
name: learn-ywai
description: "Teach ywai from the official docs. Trigger: /learn-ywai, learn ywai, ensename ywai, how does ywai work."
---

## When to Use

The user wants to **learn ywai** (CLI, agents, skills, workflows), not implement a feature.

## Critical Patterns

- **Docs are the teacher.** Read only `references/docs/` inside this skill. Do not invent commands, flags, or agent names, and do not fetch the website.
- **One page per turn.** Teach that page, then stop. Ask at most one question.
- **Match the user's language.** The docs are Spanish; reply in the user's language.
- **Do not start a `teach/` workspace** (no `MISSION.md` / HTML lessons) unless they ask for a long course.

## How to teach

1. Open [references/curriculum.md](references/curriculum.md).
2. Pick the page: explicit topic, else tour item 1 (or the next unread item if they already started).
3. Read that file from `references/docs/`.
4. Teach from that page only:
   - Lead with the outcome (what they can do after this).
   - Show the happy path (commands / `@agent` examples from the page).
   - One check question so they retrieve, not just nod.
5. Offer the next curriculum item. Do not dump the rest of the site.

If a page is missing or fetch fails, say so and teach only what you successfully read.

## Commands

```bash
# After ywai install / update, the slash command is:
/learn-ywai
/learn-ywai agentes
```
