---
name: delegate-opencode2
description: "Delegate a bounded coding task to the local opencode2 CLI, then verify it. Trigger: delegar a opencode2, opencode-admin."
---

# Delegate to opencode2

A cold agent on this machine. It gets a prompt and a repo, nothing else — everything it needs must be in the prompt or at a path the prompt names.

Load when the user asks to hand a bounded coding task to opencode2 or names an `opencode-admin/*` model. Do NOT load for a one-file edit you already understand, a quick state check, or work that depends on context only this session holds: a cold agent re-derives it and costs more than doing it yourself.

## Hard rules

- The binary is `~/.opencode/bin/opencode2`. `opencode` (v1) is a different tool.
- Always pass `--auto`. Without it the agent blocks on a permission prompt nobody can answer and the run dies.
- Never run two agents in the same git worktree. Different repos are safe in parallel.
- Never ask an agent to commit. Review first, commit yourself.
- Delegation does not transfer verification. A green suite in the agent's report is a claim, not evidence.
- Re-read the prompt for contradictory constraints before launching. The agent stops to ask, and non-interactively that is a dead run that wrote nothing.

## Routing

| Task shape | Route |
|---|---|
| 2+ non-trivial files, or a new endpoint/service | delegate |
| Cross-repo work | one agent per repo, parallel |
| Same repo, dependent steps | sequential, never parallel |
| Follow-up on work an agent just did | `--session <id>` — do not relaunch cold |

| Model | When |
|---|---|
| `opencode-admin/glm-5.3-flash` | default for coding tasks |
| `opencode-admin/grok-4.5` | the task needs real search or multi-step reasoning; flash models stall on these |
| any other | `Error: Transport` twice running — the provider is failing, not your prompt |

## Launch

Write the prompt to a file first; never inline a long prompt in the shell.

```sh
cd <repo>
nohup ~/.opencode/bin/opencode2 run --auto --standalone --format json \
  --model opencode-admin/glm-5.3-flash --title "<short>" \
  "$(cat <prompt-file>)" > <log> 2>&1 &
echo $! > <log>.pid
```

- `--standalone` runs a private server instead of joining the shared background service, so one job cannot disturb another's session.
- `--format json` gives you a parseable result and the session id, instead of scraping prose out of a log.
- `--agent <name>` picks a configured agent when you want its system prompt and permissions rather than a bare model.

Wait on **that** process, not on the program name:

```sh
while kill -0 "$(cat <log>.pid)" 2>/dev/null; do sleep 10; done
```

`pgrep opencode2` matches every opencode2 on the machine, including jobs you did not launch — it will report done while yours is still running, or hang on someone else's.

## Verify

The report is a claim. Verification is yours:

1. Run the tests yourself.
2. Mutation-test the claim: break the logic on purpose — a guard, a precedence order, an error containment — and confirm a test fails. A surviving mutation means the behavior is unpinned, not that the code is right.
3. Restore mutations with a file copy. Never `git checkout --` on uncommitted work.
4. Commit yourself, one work unit per commit.

Two measurement traps in your own review: `git stash` without `-u` leaves new files behind, so the "clean" baseline is contaminated; and a mutation that breaks compilation makes the runner execute *fewer* suites and still report green — a lower test count is not a pass.

Found a bug while reviewing? Continue the same session with `--session <id>` and pin the finding as evidence. The agent still has the context; a cold relaunch does not.

## Report back

Files changed · which claims you verified yourself · which mutations were caught and which survived · the exact test output. State plainly anything the agent reported that you could not confirm.

Prompt anatomy, worked traps, and failure modes: [prompt-recipe.md](references/prompt-recipe.md).
