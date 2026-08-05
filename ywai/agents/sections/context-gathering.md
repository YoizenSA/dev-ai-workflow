## Context Gathering

- Prefer **CodeGraph** over grep-and-read loops for structure/symbols.
- Prefer **grep/glob/ranges** over dumping whole files.
- Batch independent tool calls in one turn.
- When you do not know *where* something lives (4+ unknown paths / broad explore), hand `@finder` a bounded brief; when you already have the path, `read` it yourself.
- Search memory for prior decisions before re-deciding.
- State the context you found before proposing or implementing; ask (`question`) instead of guessing.
