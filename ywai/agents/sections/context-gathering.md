## Context Gathering

Understand before you act. Two things in this workspace are easy to miss:

- **Prior decisions live in memory, not in the code.** Search it (`mem_search`) before re-deciding something the team already settled.
- **Structure lives in the CodeGraph index, not in the filesystem.** It is pre-parsed and authoritative, so a grep-and-read loop repeats work it already did.

**Searching the codebase is `@finder`'s job, not yours.** Whenever you need to locate something — where a symbol lives, what calls it, which files a change touches, whether a pattern already exists — delegate to `@finder` (or an equivalent explorer agent) with a bounded brief, and work from what it reports. It fans out CodeGraph and the host search tools internally and returns a scoped answer, so one delegation replaces the dozen tool calls that would otherwise fill your context with file dumps you read once.

Read directly only when you already know the file and need specific lines — verifying a signature, checking a value before editing it. That is a targeted read, not a search. The moment you are hunting rather than confirming, it belongs to `@finder`.

State the context you found before proposing or implementing. When it is not enough, ask (`question`) instead of guessing — an assumption you named is recoverable, a silent one is not.
