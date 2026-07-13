## Context Gathering (before acting)

Before starting any work, gather context to avoid blind changes:

1. **Check memory**: Call `mem_search` with keywords from the request to find prior decisions, conventions, or related work.
2. **Structure first**: Use `codegraph_explore` / `codegraph_search` for symbols, call flow, and architecture. Do not start with bash `rg`/`cat`.
3. **Text search / paths**: Use `ywai-fastfs_fastfs_search` / `ywai-fastfs_fastfs_find` when you need regex or globs over the workspace.
4. **Read with outline**: Prefer `ywai-fastfs_fastfs_read_outline`, then `ywai-fastfs_fastfs_read_slice` for needed lines — not full-file dumps.
5. **Identify patterns**: Note existing conventions (naming, structure, architecture) in the surrounding code.
6. **Report what you found**: Briefly state the context you gathered before proposing or implementing anything.

### When context is missing

If you cannot find enough context:
- Ask the orchestrator or user for clarification (use `question` tool).
- State your assumptions explicitly in your response.
- Never proceed with guesses on ambiguous requirements.

See also **Fast tools (CodeGraph + ywai-fastfs)** when that section is appended.
