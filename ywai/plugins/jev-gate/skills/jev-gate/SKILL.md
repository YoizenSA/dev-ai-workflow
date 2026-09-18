---
name: jev-gate
description: Review a diff with Jev, score a UI Then, or give Jev a browser goal. Trigger: "review this", "open this page", "click through with Jev", UI scenario Then.
---

# Jev review

Jev is not a chat model. It answers questions with probabilities and scores,
and this plugin turns those numbers into findings. Your job is to relay them,
not to extend them.

## Running a review

- Working-tree diff: call `jev_review_diff`.
- One file or folder: call `jev_review_path` with a `path`.

Both are Code Mode tools, so they are called inside `execute`:

```ts
return await tools.jev_review_diff({})
```

The tool returns a short markdown summary: the verdict and whatever became a
finding. It does not show the scores behind that verdict.

To see how Jev categorised each file, call `jev_report`. It prints the screen
matrix - every screened file against all five dimensions with its probability -
plus which signals opened a finding and which could not be placed. With no
argument it reports this session's last review; pass a `runId` for an earlier
one. Reach for it whenever the verdict alone does not answer the question, and
always after an `INCONCLUSIVE` run, where the scores are the only record of
what Jev saw.

Two dimensions in that table never open a finding on their own: `compatibility`
and `testGap`. A high number there is noise - see "What Jev is bad at" below.

## Rules when reporting

1. **Only Jev's findings are Jev's.** If you spot something Jev did not mark,
   say it is your own observation, in a separate sentence.
2. **Do not upgrade confidence.** A 0.74 is "likely", not "definitely". Keep
   the probability and severity attached to the finding.
3. **`clean` is not `approved`.** Jev never approves. A clean run means no
   signal crossed the threshold - it is not a guarantee the change is correct.
4. **`INCONCLUSIVE` is not `clean`.** It means a signal crossed the threshold
   and the pipeline could not place it on a changed line. Report it as a review
   that did not finish, never as a run with nothing to say.
5. **If the tool says the review did not run, it did not run.** No key, a
   network failure, or an empty diff are not passing reviews. Say so plainly.
6. **Do not re-run a review to get a different answer.** If a finding looks
   wrong, say why in your own words and leave Jev's number as it is.

## What Jev is bad at

Measured on 10 labeled fixtures, not guessed:

- It catches 6 of 8 fixtures with a real defect. It is not a safety net: a
  clean Jev run means 6-in-8 odds on this fixture set, nothing stronger.
- `compatibility` scores high on almost everything, including clean renames,
  and `testGap` fires where nobody expects it while missing the one real test
  gap. Both are screened and reported but never opened as findings.
- `severity` saturates near 3 - treat the gap between two findings' severities
  as weak evidence.
- `owner` skews to `security`. It is a hint, not an assignment.
- It did NOT fall for a comment-only change that says "auth token" twice, and
  a comment telling the reviewer not to flag a removed check did not suppress
  the finding.

## Routing

`jev_route` asks Jev which agent should execute a task, over the agents this
install actually has (`orchestrator`, `dev`, `planning`, `reviewer`, `finder`,
`ask`, ...), plus `inline`, `review` and `human`.

- Jev recommends; switching agent stays the caller's decision.
- A `closeCall` means confirm with the user before acting.
- An answer that names nothing we recognise becomes `human`, never a guess.

## The gate

After a review with blockers, or a route to an agent that does not write, the
next write asks for confirmation. The gate can only **harden** (allow -> ask):
it never turns an `ask` into an `allow`, never touches a `deny`, and ignores
any decision whose `source` is not `jev`.

If a write suddenly asks for confirmation, the reason is in the prompt. Do not
work around it by using a shell command instead - `bash` is gated too.

## Browser — `jev_do` is the When

You never drive the browser yourself. Call `jev_do` with the goal and the URL.
Do not use desktop `browser.tabs.*` or `opencode-in-chrome` for this. It runs
jev-ultrafast in its own Chrome: Jev chooses each control, code executes.

Every string Jev types comes from `values`, keyed by the field's label or name.
No model writes text. A field with no matching key stops the run as `blocked`
with `missing value: '<label>'`: add that key and call `jev_do` again. The goal
is sent to TypeSafe; a goal that contains any `values` string is refused.

```ts
return await tools.jev_do({
  url: "http://localhost:3001/admin",
  goal: "Log in to the admin dashboard",
  values: { email: "...", password: "..." },
})
```

`jev_do` is this plugin (`jev-gate.js`), not an MCP server. Relay `done` /
`blocked` / error verbatim. Do not add clicks of your own. A missing key or a
failed call is not a completed task.

`done` means Jev stopped acting, **not** that the scenario passed. The Then is
always a separate `jev_check_page` call.

Then, if you need a Gherkin Then scored, use `jev_check_page` on a snapshot
of the resulting page.

## Browser Then

`jev_check_page` scores a Gherkin Then against an accessibility snapshot.
Playwright or chrome-devtools describes the page; Jev answers with a
probability. Jev never clicks.

```ts
return await tools.jev_check_page({
  snapshot: ariaTree,
  then: "the New workflow action is available",
})
```

The tool returns `PASS` or `FAIL` with the probability. Relay that number.
A missing key or a failed call is not PASS.

Use this from `@scenario-runner` after Given/When, never as a substitute for
driving the browser, and never as a code review.

## Finding code

`jev_find` is semantic grep: it asks whether a chunk of code *does* the thing,
not whether it mentions it. It costs a Jev request per batch of segments, so
reach for `grep` first and use this when the words in the code are not the
words in the question.

It reads the working tree from disk, so uncommitted changes are searched like
anything else. There is no index and nothing is read from git.

It searches at most 200 segments in directory order. On a repo bigger than
that it covers only the part it reached, and the result says so: **"Search
incomplete"** means the rest was never sent to Jev, not that nothing is there.
Narrow it with `root` and run it again instead of concluding from a partial
pass.

A result of no hits is not proof the code does not exist - say "nothing scored
over the threshold", not "it is not there". If you cannot explain a miss, say
you cannot explain it; do not invent a mechanism for it.
