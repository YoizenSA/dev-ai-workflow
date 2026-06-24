### Delegation Rules

Core principle: **does this inflate my context without need?** If yes -> delegate. If no -> do it inline.

| Action                                                     | Inline | Delegate                     |
| ---------------------------------------------------------- | ------ | ---------------------------- |
| Read to decide/verify (1-3 files)                          | Yes    | No                           |
| Read to explore/understand (4+ files)                      | No     | Yes                          |
| Read as preparation for writing                            | No     | Yes, together with the write |
| Write atomic (one file, mechanical, you already know what) | Yes    | No                           |
| Write with analysis (multiple files, new logic)            | No     | Yes                          |
| Bash for state (git, gh)                                   | Yes    | No                           |
| Bash for execution (test, install, external tooling)       | No     | Yes                          |

Use OpenCode's native `task` tool for delegated work.

Delegation is not optional once complexity appears. If a task crosses a trigger below, use the smallest useful sub-agent workflow instead of continuing as a monolithic executor.

#### Mandatory Delegation Triggers

These gates are **non-skippable hard gates**, not recommendations. Do not skip them, do not weaken them, and do not replace delegation-required gates with inline execution. Tool unavailability is not a waiver; document it, stop the blocked delegated work, and perform the closest fresh-context audit only where the fired rule calls for review/audit.

Semantic guard: **delegate** means using OpenCode's native `task` tool to invoke a configured sub-agent. Running local scripts, Python, or Bash inline is execution, not delegation.

1. **4-file rule**: if understanding requires reading 4+ files, delegate a narrow exploration/mapping task. If delegation tooling is unavailable, document the blocker and stop the exploration instead of reading everything inline.
2. **Multi-file write rule**: if implementation will touch 2+ non-trivial files, delegate one writer. If delegation tooling is unavailable, document the blocker and stop the implementation; a fresh review is required after delegated implementation, not a substitute for delegation.
3. **PR rule**: before commit, push, or PR after code changes, run a fresh-context review unless the diff is trivial docs/text.
4. **Incident rule**: after wrong `cwd`, accidental repo/worktree mutation, merge recovery, confusing test command, or environment workaround, stop and run a fresh audit before continuing.
5. **Long-session rule**: after roughly 20 tool calls, 5 exploratory file reads, or 2 non-mechanical edits without delegation and growing complexity, pause and delegate the remaining work instead of silently continuing monolithically. If delegation tooling is unavailable, document the blocker and stop the complex work.
6. **Fresh review rule**: use fresh context for adversarial review of diffs, conflicts, PR readiness, and incidents; use continuity/forked context only for implementation work that needs inherited state.
