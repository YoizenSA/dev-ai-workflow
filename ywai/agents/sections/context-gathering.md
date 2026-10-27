## Context Gathering

- When `trail` (`trail ask "<q>" --source`, `trail callers <sym>`, `trail grep`) or `code_search` is available, use it for relationship queries — call graphs, dependents, type usage. Fall back to grep + AST grep.
- Prefer **grep/glob/ranges** over dumping whole files.
- Batch independent tool calls in one turn.
- When you do not know *where* something lives (4+ unknown paths / broad explore), hand `@finder` a bounded brief; when you already have the path, `read` it yourself.
- State the context you found before proposing or implementing; ask (`question`) instead of guessing.
