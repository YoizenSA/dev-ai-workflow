## Jev gate

Jev is a classifier, not a chat model: it answers with probabilities and
scores. The `jev_*` tools turn those numbers into findings. Relay them; do not
extend them.

### Tools

- `jev_review_diff` — review the working-tree diff.
- `jev_review_path` — the same pipeline over a file or directory.
- `jev_find` — semantic grep: where something is *done*, not where it is named.
  Reach for `grep` first; use this when the words in the code are not the words
  in the question.
- `jev_route` — which agent should execute a task.

They are Code Mode tools, so call them inside `execute`:
`return await tools.jev_review_diff({})`.

### Rules when reporting a review

1. **Only Jev's findings are Jev's.** Anything you noticed yourself is your own
   observation, said in a separate sentence.
2. **Do not upgrade confidence.** A 0.84 is "likely", not "definitely". Keep the
   probability and severity attached to the finding.
3. **`clean` is not `approved`.** Jev never approves. On its own fixture set it
   catches 6 of 8 diffs with a real defect, so a clean run is not a safety net
   and must never be reported as one.
4. **If the tool says it did not run, it did not run.** No key, a network
   failure or an empty diff are not passing reviews. Say so plainly.
5. **Do not re-run to get a different answer.** If a finding looks wrong, say
   why in your own words and leave Jev's number as it is.

### Known weaknesses, measured

`compatibility` and `testGap` are screened and reported but never opened as
findings: the first scores high on clean renames, the second fires where
nobody expects it and missed the one real test gap. `severity` saturates near
3, so the gap between two severities is weak evidence. `owner` skews to
`security` — a hint, not an assignment.

### The gate

After a review with blockers, or a route to an agent that does not write, the
next write asks for confirmation. The reason is in the prompt. Do not work
around it with a shell command: `bash` is gated too.
