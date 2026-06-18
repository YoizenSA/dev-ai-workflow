# Skills Review Report — writing-great-skills rubric

**Reviewed**: 2026-06-18
**Rubric source**: `writing-great-skills/SKILL.md` + `GLOSSARY.md`

## Summary
- Total skills reviewed: 25
- Critical issues: 14
- Suggestions: 31

## Per-Skill Assessment

### adr-skill
**Invocation**: model-invoked — appropriate (agent needs to discover and fire it autonomously)
**Description**: Front-loads "Create" well. The trigger list is long but covers distinct branches (propose, write, update, accept/reject, deprecate, supersede, bootstrap, consult, enforce). Minor duplication between "propose" and "write" — both mean drafting a new ADR.
**Structure**: Four-phase workflow is well-structured with clear steps and completion gates (Intent Summary Gate). Good use of progressive disclosure via `references/` and `assets/templates/`. The "Consulting ADRs" read-workflow is a separate branch and cleanly separated.
**Issues**:
- **Suggestion**: Philosophy section (lines 9-14) is exposition — it frames the skill but doesn't change agent behavior. Could be compressed to one sentence or moved to a pointer.
- **Suggestion**: "Code ↔ ADR Linking" section partially duplicates what the Implementation Plan already covers (affected paths). Could be a pointer.
- **Suggestion**: No strong leading word beyond "ADR" itself. Consider anchoring around a concept like _specification_ or _contract_ to focus the agent's execution model.

**Recommendation**: Trim philosophy, consider disclosing the linking section. Overall one of the better-structured skills.

---

### angular
**Invocation**: model-invoked — appropriate (router skill for Angular sub-skills)
**Description**: Front-loads "Angular development patterns" — clear. Trigger list: "Angular components, signals, inject, zoneless, project structure, file placement, forms, validation, performance optimization, lazy loading, images." This is a flat list of triggers from all 4 sub-skills combined, which is the correct pattern for a router skill.
**Structure**: Pure router — delegates to 4 sub-skills via a table. Clean and minimal.
**Issues**:
- **Suggestion**: The trigger list could be collapsed into higher-level branch names: "components & signals", "architecture & structure", "forms & validation", "performance & images" — saves tokens without losing discoverability.

**Recommendation**: Fine as-is. Minor trigger compression opportunity.

---

### angular-architecture
**Invocation**: model-invoked — appropriate (distinct leading word: "architecture", "structure")
**Description**: "Angular architecture and project structure. Trigger: structuring Angular projects, file placement." — Front-loads "Angular architecture" well. The two triggers ("structuring Angular projects", "file placement") overlap — file placement IS structuring projects. **Duplication** in the trigger list.
**Structure**: Concise reference. Scope Rule is well-anchored. Style Guide section is good reference.
**Issues**:
- **Suggestion**: Trigger duplication — "structuring Angular projects" and "file placement" are one branch.
- **Suggestion**: Commands section at the bottom is useful reference but could be a pointer for a slimmer top-level.

**Recommendation**: Collapse the two triggers into one. Minor.

---

### angular-core
**Invocation**: model-invoked — appropriate (core Angular patterns)
**Description**: "Angular core patterns. Trigger: Angular components, signals, inject, zoneless." — Front-loads "Angular core patterns." Good leading words: "signals", "inject", "zoneless."
**Structure**: All reference, well-organized by topic. REQUIRED labels create clear completion criteria for agents.
**Issues**:
- **Suggestion**: "NO Lifecycle Hooks (REQUIRED)" — the section title uses a negative. The rubric prefers positive framing. "Signals replace lifecycle hooks" is the actual rule; the section could lead with that.
- **Suggestion**: The "When to Use What" table within the lifecycle section partially duplicates the "RxJS - Only When Needed" table below. Both address "when to use signals vs RxJS."

**Recommendation**: Collapse the two decision tables into one. Minor duplication.

---

### angular-forms
**Invocation**: model-invoked — appropriate
**Description**: "Angular forms. Trigger: forms, validation, form state in Angular." — Front-loads "Angular forms." The three triggers are distinct branches (forms, validation, form state).
**Structure**: Good structure — "When to Use What" decision table, then patterns by approach. Concise.
**Issues**:
- **Suggestion**: Signal Forms marked "experimental" (v21+) — if the project doesn't use v21+, this branch is dead weight for most runs. Consider disclosing behind a pointer.

**Recommendation**: Fine. Consider progressive disclosure for experimental content.

---

### angular-performance
**Invocation**: model-invoked — appropriate
**Description**: "Angular performance. Trigger: optimizing Angular apps, images, lazy loading." — Front-loads "Angular performance." Good leading words.
**Structure**: Clean reference organized by topic. NgOptimizedImage rules are clear with REQUIRED framing.
**Issues**:
- **Suggestion**: "NEVER trigger reflows/repaints in lifecycle hooks" at the bottom is a no-op if the agent is already following the angular-core skill's "NO lifecycle hooks" rule. Cross-skill duplication.

**Recommendation**: Fine. The lifecycle warning could reference angular-core instead of restating.

---

### architect-worker
**Invocation**: model-invoked — **should be user-invoked or orchestrator-internal**
**Description**: "Architecture and design worker — makes design decisions and defines structure before implementation" — This is an orchestrator-delegated subagent. It should never be autonomously discovered and fired by the model. Having a description wastes context load.
**Structure**: Minimal — 6 steps + handoff template. Steps are clear but very high-level.
**Issues**:
- **Critical**: Should set `disable-model-invocation: true` — this is an orchestrator-internal role, not a user-facing skill. Paying context load for no discoverability benefit.
- **Suggestion**: "Required Skills and Tools" lists only `git` — is that accurate for architecture work?

**Recommendation**: Add `disable-model-invocation: true`. This is a pattern that applies to ALL worker skills (see Cross-Cutting Patterns).

---

### backend-worker
**Invocation**: model-invoked — **should be user-invoked or orchestrator-internal**
**Description**: "Backend implementation worker" — Extremely generic. No leading word, no trigger branches. Front-loads "Backend" but "implementation worker" is vague.
**Structure**: 5 steps + handoff. Steps are skeletal — "Write failing tests first (TDD)" doesn't say what kind of tests or how to determine scope.
**Issues**:
- **Critical**: Should set `disable-model-invocation: true` — orchestrator-internal role.
- **Critical**: Description is a no-op — "Backend implementation worker" doesn't help the agent distinguish this from `implementation` or `frontend-worker`. **Duplication** across three nearly identical worker skills.
- **Suggestion**: Steps lack completion criteria. "Manually verify the implementation" — how? What does verified look like?

**Recommendation**: Add `disable-model-invocation: true`. Differentiate from implementation/frontend-worker or merge them.

---

### condition-based-waiting
**Invocation**: **Missing frontmatter entirely** — no `name`, no `description`, no invocation config. This means it's invisible to the agent and the system unless manually referenced.
**Description**: N/A (missing). The heading "Condition-Based Waiting" is the only identity.
**Structure**: Excellent reference — Quick Reference table, Core Principle, 8 pattern categories, anti-patterns, decision matrix, integration example. Well-organized.
**Issues**:
- **Critical**: Missing frontmatter. Needs `name`, `description`, and invocation decision. Should be model-invoked since other testing skills could benefit from referencing it, OR disclosed behind a context pointer from the Playwright skills.
- **Suggestion**: "Real-World Impact" section (debugging session stats) is stale evidence — it won't change agent behavior. **Sediment**.
- **Suggestion**: The "Core Principle" ("Wait for conditions, not time") is a strong leading word opportunity — the word _condition_ could anchor the whole skill.

**Recommendation**: Add frontmatter. Remove "Real-World Impact." Consider whether this should be a pointer target from playwright skills rather than standalone.

---

### devops
**Invocation**: model-invoked — appropriate (distinct domain: Azure Pipelines + Helm)
**Description**: "Azure DevOps pipelines and Helm Umbrella charts. Trigger: CI/CD, Azure Pipelines, Helm, Kubernetes." — Front-loads "Azure DevOps pipelines." Good leading words: "Umbrella charts," "{{chartversion}}."
**Structure**: Excellent progressive disclosure — core rules inline, full YAML examples in `references/PIPELINES.md`, chart templates in `references/HELM-STRUCTURE.md`, values reference in `references/VALUES-REFERENCE.md`. Clean hierarchy.
**Issues**:
- **Suggestion**: Trigger "Kubernetes" is broad — this skill is specifically about Azure DevOps + Helm on AKS, not general K8s. Could misfire for generic K8s tasks.
- **Suggestion**: Troubleshooting table is good reference but could be a pointer for slimmer top-level.

**Recommendation**: Tighten "Kubernetes" trigger to "AKS" or "Kubernetes deployment via Helm." Overall well-designed.

---

### devops-worker
**Invocation**: model-invoked — **should be user-invoked or orchestrator-internal**
**Description**: "DevOps and infrastructure worker" — Same pattern as backend/frontend-worker. Generic, no triggers.
**Structure**: 5 steps + handoff. Skeletal.
**Issues**:
- **Critical**: Should set `disable-model-invocation: true`.
- **Critical**: Description is generic — doesn't distinguish from devops skill.

**Recommendation**: Add `disable-model-invocation: true`. Consider whether this separate skill is needed vs. just using devops.

---

### docker
**Invocation**: model-invoked — appropriate (distinct domain)
**Description**: "Hardened multi-stage Dockerfiles (NestJS/Node 24 backends, .NET 10 APIs, Angular SPAs on nginx). Trigger: creating or reviewing a Dockerfile, containerizing a service, image size or security hardening." — Front-loads "Hardened" — strong leading word that signals the skill's quality bar. Triggers are distinct branches.
**Structure**: Excellent — template selection table, hard rules (numbered), runtime contract, digest pinning, review checklist, anti-patterns table. Good mix of steps (pick template) and reference (rules, checklist).
**Issues**:
- **Suggestion**: Rule 10 ("No HEALTHCHECK instruction") overlaps with devops skill's Helm liveness/readiness probe guidance. Cross-reference instead of restating rationale.
- **Suggestion**: Anti-patterns table partially duplicates the Hard Rules — e.g., "FROM node:latest" is already covered by Rule 2. **Duplication**.

**Recommendation**: Anti-patterns table could reference rule numbers instead of restating fixes. Overall excellent skill.

---

### frontend-worker
**Invocation**: model-invoked — **should be user-invoked or orchestrator-internal**
**Description**: "Frontend implementation worker" — Same generic pattern.
**Structure**: 5 steps + handoff. Identical to backend-worker except "Manually verify the implementation in browser" adds specificity.
**Issues**:
- **Critical**: Should set `disable-model-invocation: true`.
- **Critical**: Near-identical to backend-worker and implementation. Three skills with the same structure and 90% same content. **Duplication**.

**Recommendation**: Merge with implementation or differentiate clearly.

---

### git-commit
**Invocation**: model-invoked — appropriate (commit-related tasks)
**Description**: "Git commit standards and conventional commits." — Front-loads "Git commit standards." Clean.
**Structure**: Large reference covering conventions, examples, hooks, versioning, changelog, branching, troubleshooting, best practices. Well-organized with clear sections.
**Issues**:
- **Suggestion**: **Sprawl** — the skill is very long (~300 lines). The Versioning, Release Branches, Changelog, and Troubleshooting sections are heavy reference that could be disclosed behind pointers (e.g., `references/VERSIONING.md`, `references/CHANGELOG.md`).
- **Suggestion**: Bilingual content — "Convenciones de Commit (Commit Conventions)" and "Referencias (References)" mix Spanish headers with English content. Inconsistent with the rubric's "technical artifacts default to English."
- **Suggestion**: "Common Patterns" section (Feature Addition, Bug Fix, Breaking Change, Release Preparation) overlaps heavily with the earlier examples. **Duplication**.
- **Suggestion**: "Best Practices" section contains no-ops — "Small, Focused Commits" and "Test Before Commit" are things the agent already does by default.

**Recommendation**: Disclose Versioning/Changelog/Troubleshooting behind pointers. Remove Common Patterns (already covered by examples). Remove no-op best practices.

---

### implementation
**Invocation**: model-invoked — **should be user-invoked or orchestrator-internal**
**Description**: "Generic implementation worker (default dev role)" — Generic. No triggers.
**Structure**: 5 steps + handoff. Identical to backend-worker.
**Issues**:
- **Critical**: Should set `disable-model-invocation: true`.
- **Critical**: **Duplication** — this is a carbon copy of backend-worker with slightly different example handoff. Both are "implementation worker" with TDD steps.
- **Suggestion**: "Required Skills and Tools" lists only `git` — same concern as architect-worker.

**Recommendation**: Merge with backend-worker/frontend-worker into one unified worker skill, or differentiate by scope.

---

### legacy-migration-workflow
**Invocation**: model-invoked — appropriate (distinct domain: ASPX migration)
**Description**: "Use when running ASPX to WebApi plus Angular migrations with mandatory soft gates between planning, build, and validation." — Front-loads the migration context. Good leading words: "soft gates," "mandatory."
**Structure**: Comprehensive workflow with gate model, commands, planning loop, evidence requirements, governance, autonomous orchestration, huge surface handling. This is a well-thought-out workflow.
**Issues**:
- **Critical**: **Extreme sprawl** — the skill is enormous (~300+ lines). Massive sections like "Evidence requirements," "Mandatory governance coverage," "Huge legacy surfaces and work graphs," "Evidence-first dependency audit," "Token-efficient validation," and "Compact durable sections" are all heavy reference that should be disclosed behind pointers.
- **Suggestion**: The "Gate model" section and "Commands" section could be collapsed — the gate model IS the commands' behavior.
- **Suggestion**: "Autonomous orchestration" section duplicates the gate model logic in a different format. **Duplication**.

**Recommendation**: Disclose "Huge legacy surfaces," "Evidence-first dependency audit," "Token-efficient validation," and "Evidence artifact policy" behind pointers. Collapse gate model + commands. This skill has excellent content but needs aggressive progressive disclosure.

---

### planner
**Invocation**: model-invoked — appropriate
**Description**: "Mission planner — breaks goals into milestones, features, and validation contracts" — Front-loads "Mission planner." Clean.
**Structure**: 5 steps + handoff. **Extremely skeletal** — steps are generic ("Decompose into milestones and features") with no methodology.
**Issues**:
- **Critical**: **Too narrow / too thin** — the skill contains no actual planning methodology. It's a template, not a skill. The agent gets no guidance on HOW to decompose, WHAT makes a good milestone, or HOW to assign roles. Compared to adr-skill's rich four-phase workflow, this is empty.
- **Suggestion**: "Required Skills and Tools" lists `opencode` — why? Planning doesn't require a specific tool.

**Recommendation**: Either expand with actual planning methodology (decomposition strategies, milestone criteria, role assignment heuristics) or merge into the orchestrator as an inline capability.

---

### playwright-e2e-testing
**Invocation**: `disable-model-invocation: true` (set in frontmatter) — appropriate for a reference-only skill, but the `user-invocable: false` field is non-standard.
**Description**: Uses non-standard `progressive_disclosure` frontmatter — this is not recognized by the skill system. The `description` field says "Playwright modern end-to-end testing framework..." — this is the same description repeated in the `progressive_disclosure.entry_point.summary`.
**Structure**: **Extreme sprawl** — the skill is 4200-5200 tokens of reference covering installation, config, fundamentals, locators, page objects, interactions, assertions, auth, network, test organization, visual testing, parallel execution, CI/CD, debugging, and best practices. Despite the `progressive_disclosure` markers in the content, the ENTIRE file is loaded — the HTML comments (`<!-- ENTRY POINT -->`, `<!-- FULL CONTENT -->`) have no mechanical effect.
**Issues**:
- **Critical**: **Complete duplication** — the file at `playwright-e2e-testing/SKILL.md` and `playwright-e2e-testing/playwright/SKILL.md` are **100% identical** (byte-for-byte same content). This is the most severe duplication in the entire skill set.
- **Critical**: The `progressive_disclosure` YAML in the frontmatter and in-content HTML comments are non-standard — the skill system doesn't parse them. The entire 5000-token file loads regardless.
- **Suggestion**: Many "Best Practices" items are no-ops — "Use Stable Locators" and "Leverage Auto-Waiting" are already Playwright defaults.

**Recommendation**: Delete the duplicate file. Implement actual progressive disclosure by splitting into smaller files (e.g., `locators.md`, `auth.md`, `network.md`, `ci.md`) with context pointers. Remove non-standard frontmatter.

---

### playwright-e2e-testing/playwright
**Invocation**: Same as parent — `disable-model-invocation: true`
**Description**: Identical to parent.
**Structure**: Identical to parent.
**Issues**:
- **Critical**: **100% duplicate** of `playwright-e2e-testing/SKILL.md`. Delete this file entirely.

**Recommendation**: Delete.

---

### qa-worker
**Invocation**: model-invoked — **should be user-invoked or orchestrator-internal**
**Description**: "QA and testing worker" — Generic.
**Structure**: 6 steps + handoff. Steps are slightly more specific than backend-worker ("Check for edge cases and error conditions") but still skeletal.
**Issues**:
- **Critical**: Should set `disable-model-invocation: true`.
- **Critical**: "Required Skills and Tools" section is empty — no tools listed at all.
- **Suggestion**: Steps lack completion criteria. "Write comprehensive tests" — comprehensive by what measure?

**Recommendation**: Add `disable-model-invocation: true`. List required tools. Add completion criteria.

---

### reviewer-worker
**Invocation**: model-invoked — **should be user-invoked or orchestrator-internal**
**Description**: "Code review worker — audits diffs and reports findings without editing code" — Front-loads "Code review worker." The "without editing code" constraint is valuable.
**Structure**: 5 steps + handoff. Clear that this is a read-only role.
**Issues**:
- **Critical**: Should set `disable-model-invocation: true`.
- **Suggestion**: Steps say "Audit for correctness, security, performance, readability, and project conventions" — this is a flat checklist with no prioritization or methodology. The testing-expert skill has a much more detailed review approach.

**Recommendation**: Add `disable-model-invocation: true`. Consider linking to testing-expert's inspection methodology for test review.

---

### tailwind-4
**Invocation**: model-invoked — appropriate (distinct domain)
**Description**: "Tailwind CSS 4 patterns. Trigger: styling, CSS utilities, responsive design." — Front-loads "Tailwind CSS 4." Triggers are clean.
**Structure**: Good reference — decision tree, critical rules, cn() utility, patterns by category. Concise.
**Issues**:
- **Suggestion**: "Keywords" section at the bottom ("tailwind, css, styling, cn, utility classes, responsive") is non-standard metadata — the skill system doesn't parse it. **Sediment**.
- **Suggestion**: "Common Patterns" section (Flexbox, Grid, Spacing, Typography, etc.) is generic Tailwind reference that duplicates official docs. Most of this is no-op — the agent already knows basic Tailwind classes.

**Recommendation**: Remove "Keywords" section. Trim "Common Patterns" to only project-specific conventions (if any).

---

### testing-expert
**Invocation**: `disable-model-invocation: true` — appropriate (user-invoked reference)
**Description**: Frontmatter name is `test-quality-inspector` but directory is `testing-expert`. Description says "Test quality inspection framework for reviewing test coverage, identifying gaps, and ensuring comprehensive validation." The name mismatch is confusing.
**Structure**: This is a **single worked example** — a complete inspection report for a fictional user-registration test suite. It's not a methodology; it's a demonstration.
**Issues**:
- **Critical**: **Name mismatch** — frontmatter says `test-quality-inspector`, directory says `testing-expert`, description says "Test quality inspection framework." Three different names for the same skill. This will confuse both humans and agents.
- **Suggestion**: The skill has no steps or methodology — it's entirely one example. The rubric says "a demanding completion criterion drives thorough legwork" — this skill has no completion criterion at all.
- **Suggestion**: Extremely long (~400 lines of example). The example teaches by demonstration, but the actual inspection criteria are buried in the example rather than stated as reference.

**Recommendation**: Rename to one consistent name. Extract the inspection criteria (what to check, severity levels, decision matrix) into reference. Keep the example as a pointer target.

---

### webapp-testing
**Invocation**: `disable-model-invocation: true` — appropriate
**Description**: Uses non-standard `progressive_disclosure` frontmatter (same as playwright-e2e-testing). Description: "Comprehensive web application testing patterns with Playwright TypeScript, selectors, wait strategies, and best practices."
**Structure**: Pure reference — selectors, wait strategies, interactions, assertions, test organization, network, screenshots, debugging, parallel execution. Well-organized by topic.
**Issues**:
- **Critical**: **Massive overlap with playwright-e2e-testing and condition-based-waiting** — this is the THIRD Playwright reference skill. Selectors, wait strategies, assertions, network interception, and debugging are covered in all three with different examples.
- **Suggestion**: Non-standard `progressive_disclosure` frontmatter (same issue as playwright-e2e-testing).
- **Suggestion**: "Selector Best Practices" priority order differs from playwright-e2e-testing's recommendation (this skill says `data-testid` first; playwright says role-based first). **Contradictory guidance** across skills.

**Recommendation**: Merge into playwright-e2e-testing or replace with a pointer. Three separate Playwright skills is excessive.

---

### yz-ui
**Invocation**: model-invoked — appropriate (distinct design system domain)
**Description**: "Yoizen UI design system (Dark Glass theme). Trigger: Yoizen UI components, styling, colors, typography, light/dark theme, tables, modals, dropdowns." — Front-loads "Yoizen UI design system." Good leading words: "Dark Glass theme."
**Structure**: Excellent progressive disclosure — core doctrine inline, deep references in `references/` (theming, tables, forms-modals, performance, docs). The "One Rule" section is a strong anchor. Design tokens table is a useful quick reference.
**Issues**:
- **Suggestion**: "Design Tokens (palette.css)" table duplicates what's already in `assets/theme/palette.css`. The table is useful as a quick reference, but adding a row "See `assets/theme/palette.css` for full list" would clarify the relationship.
- **Suggestion**: "Contrast Rules (learned the hard way)" — 6 rules that are all important but could be consolidated. Some rules repeat the same principle (e.g., rules 2 and 6 both address text color on dark surfaces).

**Recommendation**: Minor consolidation of contrast rules. Overall one of the best-designed skills — strong leading word, good progressive disclosure, clear single source of truth.

---

## Cross-Cutting Patterns

### 1. Worker Skills Should Not Be Model-Invoked (7 skills affected)
**Affected**: `architect-worker`, `backend-worker`, `frontend-worker`, `implementation`, `devops-worker`, `qa-worker`, `reviewer-worker`

All seven worker skills have `description` fields (making them model-invoked) but are only ever dispatched by the orchestrator. They should never be autonomously discovered and fired by the agent. This wastes context load on every turn for zero benefit.

**Fix**: Add `disable-model-invocation: true` to all worker skills. The orchestrator already knows their names and dispatches them explicitly.

### 2. Worker Skills Are Near-Identical (4 skills are duplicates)
**Affected**: `backend-worker`, `frontend-worker`, `implementation`, `qa-worker`

These four skills share the same 5-6 step structure:
1. Read the feature description
2. Write failing tests (TDD)
3. Implement / write tests
4. Run tests
5. Verify
6. Return handoff

The only differences are the example handoff content and one step ("Manually verify in browser" for frontend). This is **duplication** — the same meaning in multiple places.

**Fix**: Merge into one `worker` skill with role-specific branches, OR differentiate each worker with genuinely distinct methodology (e.g., backend-worker focuses on API design patterns, frontend-worker focuses on component architecture).

### 3. Playwright Skill Triplication (3 skills overlap)
**Affected**: `playwright-e2e-testing`, `playwright-e2e-testing/playwright`, `webapp-testing`, `condition-based-waiting`

Three skills cover Playwright testing with massive overlap:
- `playwright-e2e-testing` and its `playwright/` subdirectory are **100% identical**
- `webapp-testing` covers 80% of the same material (selectors, waits, assertions, network, debugging)
- `condition-based-waiting` is a focused subset (waiting strategies)

**Fix**: Delete the duplicate `playwright/SKILL.md`. Merge `webapp-testing` into `playwright-e2e-testing` (it's a superset). Keep `condition-based-waiting` as a focused reference and point to it from the Playwright skill's waiting section.

### 4. Non-Standard Frontmatter (3 skills)
**Affected**: `playwright-e2e-testing`, `playwright-e2e-testing/playwright`, `webapp-testing`, `testing-expert`

These skills use `progressive_disclosure` YAML blocks and `user-invocable` fields in their frontmatter. The skill system doesn't parse these — they're dead metadata that adds tokens without effect.

**Fix**: Remove non-standard frontmatter. Implement actual progressive disclosure via linked `.md` files and context pointers.

### 5. Missing Frontmatter (1 skill)
**Affected**: `condition-based-waiting`

This skill has no frontmatter at all — no `name`, no `description`, no invocation config. It's invisible to the skill system.

**Fix**: Add frontmatter with appropriate invocation config.

### 6. Name Inconsistency (1 skill)
**Affected**: `testing-expert` (directory) vs `test-quality-inspector` (frontmatter name)

**Fix**: Choose one name and use it consistently across directory, frontmatter, and description.

### 7. Skills Lacking Progressive Disclosure
**Affected**: `adr-skill`, `git-commit`, `legacy-migration-workflow`, `playwright-e2e-testing`, `webapp-testing`

These skills are long enough to warrant progressive disclosure but keep all reference inline. This causes sprawl — the agent wades through material only some runs need.

**Fix**: For each, identify reference sections only some branches need and move them to linked `.md` files with context pointers.

### 8. Weak or Missing Leading Words
**Affected**: `backend-worker`, `frontend-worker`, `implementation`, `planner`, `qa-worker`

These skills have generic descriptions that don't recruit pretrained concepts. "Backend implementation worker" doesn't anchor the agent's thinking the way "Hardened Dockerfiles" or "Dark Glass theme" do.

**Fix**: Find the one concept that makes each skill distinct and lead with it. If there isn't one, the skill may not need to exist separately.

---

## Priority Fixes

Ordered by impact (context load waste × frequency of impact):

### P0 — Critical (block further skill work)

1. **Delete duplicate Playwright file** (`playwright-e2e-testing/playwright/SKILL.md`). It's a byte-for-byte copy. Zero effort, immediate win.

2. **Add `disable-model-invocation: true` to all 7 worker skills**. These waste context load on every single turn for zero discoverability benefit. The orchestrator dispatches them by name.

3. **Fix `testing-expert` name mismatch**. Three different names for one skill creates confusion. Pick one and update consistently.

4. **Add frontmatter to `condition-based-waiting`**. Currently invisible to the skill system.

### P1 — High (reduce sprawl and duplication)

5. **Consolidate Playwright skills**. Merge `webapp-testing` into `playwright-e2e-testing` (or delete the overlap). Keep `condition-based-waiting` as a focused reference. This removes ~3000 duplicate tokens.

6. **Deduplicate worker skills**. Either merge `backend-worker`, `frontend-worker`, and `implementation` into one skill with branches, or give each genuinely distinct methodology.

7. **Progressive disclosure for `legacy-migration-workflow`**. Move "Huge legacy surfaces," "Evidence-first dependency audit," "Token-efficient validation," and "Evidence artifact policy" behind pointers. This skill has excellent content trapped in sprawl.

8. **Progressive disclosure for `git-commit`**. Move Versioning, Changelog, Release Branches, and Troubleshooting behind pointers.

### P2 — Medium (improve quality)

9. **Remove non-standard frontmatter** from `playwright-e2e-testing`, `webapp-testing`, and `testing-expert`. The `progressive_disclosure` YAML blocks add tokens for no effect.

10. **Remove no-op content** from `git-commit` (best practices section), `tailwind-4` (generic Tailwind patterns), and `angular-core` (lifecycle section title).

11. **Expand `planner` skill** with actual methodology or merge it into the orchestrator. Currently it's an empty template.

12. **Trim `adr-skill` philosophy section** and disclose the Code ↔ ADR Linking section.

### P3 — Low (polish)

13. **Consolidate Angular sub-skill triggers** — collapse overlapping trigger words in `angular-architecture` and `angular-core`.

14. **Remove "Real-World Impact" from `condition-based-waiting`** — stale evidence that doesn't change behavior.

15. **Remove "Keywords" section from `tailwind-4`** — non-standard metadata.

16. **Fix bilingual headers in `git-commit`** — "Convenciones de Commit" should be English for consistency.
