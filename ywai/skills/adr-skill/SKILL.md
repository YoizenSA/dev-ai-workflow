---
name: adr-skill
description: "Write and maintain Architecture Decision Records. Trigger: propose/update/accept/deprecate an ADR, bootstrap adr folder."
---

# ADR Skill

## Philosophy

ADRs created with this skill are **executable specifications for coding agents**. A human approves the decision; an agent implements it. The ADR must contain everything the agent needs to write correct code without asking follow-up questions.

This means:
- Constraints must be explicit and measurable, not vibes
- Decisions must be specific enough to act on ("use PostgreSQL 16 with pgvector" not "use a database")
- Consequences must map to concrete follow-up tasks
- Non-goals must be stated to prevent scope creep
- The ADR must be self-contained — no tribal knowledge assumptions
- **The ADR must include an implementation plan** — which files to touch, which patterns to follow, which tests to write, and how to verify the decision was implemented correctly

## When to Write an ADR

Write an ADR when a decision:
- **Changes how the system is built or operated** (new dependency, architecture pattern, infrastructure choice, API design)
- **Is hard to reverse** once code is written against it
- **Affects other people or agents** who will work in this codebase later
- **Has real alternatives** that were considered and rejected

Do NOT write an ADR for:
- Routine implementation choices within an established pattern
- Bug fixes or typo corrections
- Decisions already captured in an existing ADR (update it instead)
- Style preferences already covered by linters or formatters

When in doubt: if a future agent working in this codebase would benefit from knowing *why* this choice was made, write the ADR.

### Proactive ADR Triggers (For Agents)

If you are an agent coding in a repo and you encounter any of these situations, **stop and propose an ADR** before continuing:

- You are about to introduce a new dependency that doesn't already exist in the project
- You are about to create a new architectural pattern (new way of handling errors, new data access layer, new API convention) that other code will need to follow
- You are about to make a choice between two or more real alternatives and the tradeoffs are non-obvious
- You are about to change something that contradicts an existing accepted ADR
- You realize you're writing a long code comment explaining "why" — that reasoning belongs in an ADR

**How to propose**: Tell the human what decision you've hit, why it matters, and ask if they want to capture it as an ADR. If yes, run the full four-phase workflow. If no, note the decision in a code comment and move on.

## Workflow — read the reference for the task at hand

| Task | Reference |
|------|-----------|
| **Write a new ADR** | [references/workflow.md](references/workflow.md) — the four phases: scan the codebase → capture intent (Socratic) → draft → review against the checklist |
| **Implement in a repo that has ADRs** | [references/consulting.md](references/consulting.md) — how to find, read and respect existing decisions, and how to link code ↔ ADR |
| **Accept, deprecate, supersede, index, bootstrap** | [references/operations.md](references/operations.md) — status changes, post-acceptance lifecycle, categories for large repos |
| **Run the scripts / pick a template** | [references/resources.md](references/resources.md) — `new_adr.js`, `set_adr_status.js`, `bootstrap_adr.js`, template usage |
| **Review an ADR for agent-readiness** | [references/review-checklist.md](references/review-checklist.md) |
| **Directory, filename and status conventions** | [references/adr-conventions.md](references/adr-conventions.md) |
| **Simple vs MADR template** | [references/template-variants.md](references/template-variants.md) |
| **See a filled-out ADR** | [references/examples.md](references/examples.md) |

Never draft an ADR without running the four phases — skipping the codebase scan
is what produces a decision that contradicts the code it is supposed to govern.
