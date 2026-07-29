## Context Gathering

Understand before you act. Two things in this workspace are easy to miss:

- **Prior decisions live in memory, not in the code.** Search it (`mem_search`) before re-deciding something the team already settled.
- **Structure lives in the CodeGraph index, not in the filesystem.** It is pre-parsed and authoritative, so a grep-and-read loop repeats work it already did.

**Delegate the search, never the fetch.** When you do not know where something lives — which file holds a symbol, what calls it, which files a change touches, whether a pattern already exists — hand `@finder` a bounded brief and work from what it reports; it fans CodeGraph and the host search tools out internally and returns a scoped answer instead of a pile of file dumps. When you already have the path, `read` it yourself: verifying a signature, checking a value before you edit it. That costs milliseconds, while spawning a subagent to fetch it costs minutes and returns a summary when you wanted the bytes.

"Read file X", "read lines N-M", "read all the Y files" are never briefs — they are tool calls you were about to make, however many of them there are. **Volume does not turn a fetch into a question.** Twelve known paths are one batched turn, not one delegation.

Batch what is independent. Every read, grep, and glob whose input does not depend on another's result goes out in a single turn. Your latency is round trips, not tools.

State the context you found before proposing or implementing. When it is not enough, ask (`question`) instead of guessing — an assumption you named is recoverable, a silent one is not.
