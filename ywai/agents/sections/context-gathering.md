## Context Gathering (before acting)

Before starting any work, gather context to avoid blind changes:

1. **Check memory**: Call `mem_search` with keywords from the request to find prior decisions, conventions, or related work.
2. **Structure first**: Use `codegraph_explore` / `codegraph_search` for symbols, call flow, and architecture. Do not start with bash `rg`/`cat`.
3. **Text search / paths**: Use the host `grep` / `glob` / `code_search` tools when you need regex or globs over the workspace — not bash `rg`/`find`.
4. **Read narrowly**: Prefer `codegraph_explore` for symbol context, then `read` only the lines you need — not full-file dumps.
5. **Identify patterns**: Note existing conventions (naming, structure, architecture) in the surrounding code.
6. **Report what you found**: Briefly state the context you gathered before proposing or implementing anything.

### When context is missing

If you cannot find enough context:
- Ask the orchestrator or user for clarification (use `question` tool).
- State your assumptions explicitly in your response.
- Never proceed with guesses on ambiguous requirements.

See also **Fast tools (CodeGraph → host tools)** when that section is appended.
