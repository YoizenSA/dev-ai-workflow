---
name: code-review
description: "Review a diff for proven defects. Trigger: review this PR/branch/diff, code review, pre-merge check."
---

# Code Review

A review exists to catch what would ship broken. Its unit is the **failure scenario**: concrete inputs or state → the wrong result, crash, leak or outage. A finding with a failure scenario is a defect; without one it is a preference — drop it or mark it a nit. This one rule is what keeps a review short, credible and actionable.

Review the **diff**, not the codebase: report what this change introduces or worsens. A pre-existing problem earns at most one suggestion.

Output format, severities and routing belong to the `reviewer` agent when it runs this; otherwise use the one-line format in step 5. For Azure DevOps PRs, fetch and post through the `ado` skill (`references/workflows.md` → "Code review a PR").

## Steps

1. **Scope.** Get the full diff and the intent behind it: `git diff <base>...HEAD` (or `ado pr diff <id> --hunks`, `gh pr diff <id>`), plus the ticket, PR description or the user's ask.
   *Done when* you can state in one sentence what the change is supposed to do, and you hold the complete list of changed files.

2. **Trace.** Read every changed hunk in its surrounding code, then walk outward: callers of every changed signature, every consumer of a changed return value, config or schema. Prefer `graft_trace_calls` / `graft_find_all` over grep when available.
   *Done when* every changed file was opened and every changed public symbol has its callers checked. "Looks fine" on a file you did not open is premature.

3. **Hunt.** Run each lens in [references/lenses.md](references/lenses.md) over the traced change. For every candidate, write its failure scenario before anything else; if you cannot, it is not a defect.
   *Done when* every lens was applied to every changed file — a lens that found nothing is recorded as clean, not skipped.

4. **Verify.** Re-read the exact code path for each blocking candidate (P0/P1) and try to prove it wrong: a guard upstream, a test that already covers it, a caller that never passes that input. Run the relevant test or a read-only check when you can. Keep what survives; set confidence from what you actually observed.
   *Done when* every blocking finding was either confirmed or downgraded.

5. **Report.** Rank by what it costs to ship (data loss and auth holes first), at most 12 findings; over budget keep the most severe and close with `(+N minor findings omitted)`. One finding per line: **what — where (`file:line`) — failure scenario — fix**. Split blocking from non-blocking; formatter/linter issues are never blocking. Close with the QA angle: what a tester should exercise, one line each.
   *Done when* every finding has a location, a failure scenario and a fix — see [references/findings.md](references/findings.md) for the bar.

Never edit code, commit or post comments during a review unless the user asks; print the review first.
