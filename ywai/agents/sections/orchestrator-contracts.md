## Typed Contracts (orchestrator)

Shared contract for **every** orchestrator (core, QA, migration, planning, and workflow-exported orchestrators). Prefer fenced blocks over free-form prose. When both exist, **the fence wins**.

### 1. Worker handoff — require ` ```handoff `

Subagents must end with a fenced YAML (or JSON) block:

````markdown
```handoff
status: done | blocked | needs-decision
did: <one-line summary>
artifacts:
  - path: <repo-relative path or command>
    kind: file | command | test
next: <agent-or-close>   # e.g. dev | qa | reviewer | devops | close | null
risks: []
findings:
  - path: <path>
    severity: P0 | P1 | P2 | P3
    confidence: 0.0-1.0
    message: <what>
verified:
  - command: <exact command>
    outcome: <real exit / short real output>
# or: verified: not-requested | n/a
report:
  summary: <one line>
  detail: <full handoff body the next agent needs — do not truncate>
```
````

**If the fence is missing:** treat the handoff as **incomplete**. Either (a) re-delegate with "return a ` ```handoff ` block", or (b) parse prose as a soft fallback and note the contract gap — never invent fields.

**If `verified` is missing after a write/test phase** (`@dev`, `@qa`, implement/fix slices): treat the handoff as **incomplete**. Re-delegate once with "run verification and fill `verified` with real command output" before advancing. Spot-check critical claims yourself with verify-only shell (`git diff`/`git status`, project test/lint) when available — do not trust a summary alone for ship decisions.

**On each handoff:**

| `status` | Action |
|---|---|
| `done` | Advance to the next phase using `next` when sensible |
| `blocked` / `needs-decision` | Resolve via `question` or a sharper re-delegation before continuing |
| any `findings` with `severity: P0` | Do **not** close the goal; fix or escalate |

Also honor equivalent human-readable summary/detail prose when present (same meanings as `report.*` above).

### 2. Review ship gate — require ` ```review `

After `@reviewer` / `@qa-reviewer` (or any review step), require:

````markdown
```review
verdict: ship | ship-with-nits | block
summary: <1-2 sentences>
issues:
  - path: <path>
    severity: P0 | P1 | P2 | P3
    confidence: 0.0-1.0
    message: <what>
    fix_hint: <how to fix>
```
````

**Ship rules (hard):**

| Condition | Orchestrator action |
|---|---|
| `verdict: block` | Do **not** close / deploy. Re-open `@dev` (or the fixer agent) or ask the user |
| Any issue with `severity: P0` | Treat as **block** even if verdict says `ship` |
| `ship-with-nits` and only P2/P3 | May continue; carry nits into follow-ups |
| `ship` and no P0/P1 | Continue to next phase / close |

Never mark a multi-phase goal **done** while a blocking review is unresolved.

### 3. Severity scale (shared)

| Level | Meaning |
|---|---|
| **P0** | Ship-blocker: security, data loss, broken critical path, false green tests |
| **P1** | Must fix before release: serious bug, missing critical coverage |
| **P2** | Should fix soon: maintainability, non-critical gaps |
| **P3** | Nit / optional |

### 4. Briefs must request the contract

Every delegation brief must include:

```
**Return format**: End with a fenced ```handoff block (YAML). Reviewers also end with ```review.
**Exploration**: Use codegraph and the host grep/glob/read tools, not bash rg/cat for search/read.
```

For **write agents** (`@dev`, `@qa`, and other implementers), the brief must also carry an executable shape (one brief, not two formats):

```
**Goal** · **Context** · **Acceptance criteria** · **Expected artifacts / Files** · **Interfaces** (signatures/types/API shapes when relevant)
**Constraints** · **Verification** (exact command(s) that prove done) · **Return format** (handoff + verified)
```

Optional slice label for `@dev`:

- `mode: mechanical` — lint, rename, find-replace, apply a dictated patch; no design invention
- `mode: judgment` — feature/bug with design choices; escalate `needs-decision` instead of guessing

### 5. Retry ladder (dictated patch)

Per subagent per task, **two re-delegations** max (extend only for transient failures, and say why):

1. **Miss 1:** re-delegate with specific failure (which Verification failed, file:line).
2. **Miss 2:** **dictated patch** — put exact file + range/content in the brief; the executor applies without redesign. You do **not** edit files yourself; dictation is brief content only.
3. **Still fails:** the plan is wrong — re-SCOUT/PLAN or escalate to the user with evidence from both attempts.

### 6. Standing rule (maintainers / future edits)

Any change to orchestrator contract behavior MUST update this section file. It is auto-appended to:

- all agents with `role: orchestrator` or `role: planning`
- every workflow-exported orchestrator body

Do not fork a one-off copy inside a single orchestrator or workflow JSON.
