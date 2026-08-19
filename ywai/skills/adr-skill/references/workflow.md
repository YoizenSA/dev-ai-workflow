# Creating an ADR — the four-phase workflow

Reference for `adr-skill`. Run this when writing a new ADR.

## Creating an ADR: Four-Phase Workflow

Every ADR goes through four phases. Do not skip phases.

### Phase 0: Scan the Codebase

Before asking any questions, gather context from the repo:

1. **Find existing ADRs.** Check `docs/decisions/`, `adr/`, `docs/adr/`, `decisions/` for existing records. Read them. Note:
   - Existing conventions (directory, naming, template style)
   - Decisions that relate to or constrain the current one
   - Any ADRs this new decision might supersede

2. **Check the tech stack.** Read `package.json`, `go.mod`, `requirements.txt`, `Cargo.toml`, or equivalent. Note relevant dependencies and versions.

3. **Find related code patterns.** If the decision involves a specific area (e.g., "how we handle auth"), scan for existing implementations. Identify the specific files, directories, and patterns that will be affected by the decision.

4. **Check for ADR references in code.** Look for `ADR-NNNN` references in comments and docs (see "Code ↔ ADR Linking" below). This reveals which existing decisions govern which parts of the codebase.

5. **Note what you found.** Carry this context into Phase 1 — it will sharpen your questions and prevent the ADR from contradicting existing decisions.

### Phase 1: Capture Intent (Socratic)

Interview the human to understand the decision space. Ask questions **one at a time**, building on previous answers. Do not dump a list of questions.

**Core questions** (ask in roughly this order, skip what's already clear from context or Phase 0):

1. **What are you deciding?** — Get a short, specific title. Push for a verb phrase ("Choose X", "Adopt Y", "Replace Z with W").
2. **Why now?** — What broke, what's changing, or what will break if you do nothing? This is the trigger.
3. **What constraints exist?** — Tech stack, timeline, budget, team size, existing code, compliance. Be concrete. Reference what you found in Phase 0 ("I see you're already using X — does that constrain this?").
4. **What does success look like?** — Measurable outcomes. Push past "it works" to specifics (latency, throughput, DX, maintenance burden).
5. **What options have you considered?** — At least two. For each: what's the core tradeoff? If they only have one option, help them articulate why alternatives were rejected.
6. **What's your current lean?** — Capture gut intuition early. Often reveals unstated priorities.
7. **Who needs to know or approve?** — Decision-makers, consulted experts, informed stakeholders.
8. **What would an agent need to implement this?** — Which files/directories are affected? What existing patterns should it follow? What should it avoid? What tests would prove it's working? This directly feeds the Implementation Plan.

**Adaptive follow-ups**: Based on answers, probe deeper where the decision is fuzzy. Common follow-ups:
- "What's the worst-case outcome if this decision is wrong?"
- "What would make you revisit this in 6 months?"
- "Is there anything you're explicitly choosing NOT to do?"
- "What prior art or existing patterns in the codebase does this relate to?"
- "I found [existing ADR/pattern] — does this new decision interact with it?"

**When to stop**: You have enough when you can fill every section of the ADR — including the Implementation Plan — without making things up. If you're guessing at any section, ask another question.

**Intent Summary Gate**: Before moving to Phase 2, present a structured summary of what you captured and ask the human to confirm or correct it:

> **Here's what I'm capturing for the ADR:**
>
> - **Title**: {title}
> - **Trigger**: {why now}
> - **Constraints**: {list}
> - **Options**: {option 1} vs {option 2} [vs ...]
> - **Lean**: {which option and why}
> - **Non-goals**: {what's explicitly out of scope}
> - **Related ADRs/code**: {what exists that this interacts with}
> - **Affected files/areas**: {where in the codebase this lands}
> - **Verification**: {how we'll know it's implemented correctly}
>
> **Does this capture your intent? Anything to add or correct?**

Do NOT proceed to Phase 2 until the human confirms the summary.

### Phase 2: Draft the ADR

1. **Choose the ADR directory.**
   - If one exists (found in Phase 0), use it.
   - If none exists, create `docs/decisions/` (MADR default) or `adr/` (simpler repos).

2. **Choose a filename strategy.**
   - If existing ADRs use numeric prefixes (`0001-...`), continue that.
   - Otherwise use slug-only filenames (`choose-database.md`).

3. **Choose a template.**
   - Use `assets/templates/adr-simple.md` for straightforward decisions (one clear winner, minimal tradeoffs).
   - Use `assets/templates/adr-madr.md` when you need to document multiple options with structured pros/cons/drivers.
   - See `references/template-variants.md` for guidance.

4. **Fill every section from the confirmed intent summary.** Do not leave placeholder text. Every section should contain real content or be removed (optional sections only).

5. **Write the Implementation Plan.** This is the most important section for agent-first ADRs. It tells the next agent exactly what to do. See the template for structure.

6. **Write Verification criteria as checkboxes.** These must be specific enough that an agent can programmatically or manually check each one.

7. **Generate the file.**
   - Preferred: run `scripts/new_adr.js` (handles directory, naming, and optional index updates).
   - If you can't run scripts, copy a template from `assets/templates/` and fill it manually.

### Phase 3: Review Against Checklist

After drafting, review the ADR against the agent-readiness checklist in `references/review-checklist.md`.

**Present the review as a summary**, not a raw checklist dump. Format:

> **ADR Review**
>
> ✅ **Passes**: {list what's solid — e.g., "context is self-contained, implementation plan covers affected files, verification criteria are checkable"}
>
> ⚠️ **Gaps found**:
> - {specific gap 1 — e.g., "Implementation Plan doesn't mention test files — which test suite should cover this?"}
> - {specific gap 2}
>
> **Recommendation**: {Ship it / Fix the gaps first / Needs more Phase 1 work}

Only surface failures and notable strengths — do not recite every passing checkbox.

If there are gaps, propose specific fixes. Do not just flag problems — offer solutions and ask the human to approve.

Do not finalize until the ADR passes the checklist or the human explicitly accepts the gaps.

