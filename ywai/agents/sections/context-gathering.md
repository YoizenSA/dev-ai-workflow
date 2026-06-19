## Context Gathering (before acting)

Before starting any work, gather context to avoid blind changes:

1. **Check memory**: Call `mem_search` with keywords from the request to find prior decisions, conventions, or related work.
2. **Read related files**: Identify and read files affected by or related to the task. Don't guess — verify.
3. **Identify patterns**: Note existing conventions (naming, structure, architecture) in the surrounding code.
4. **Report what you found**: Briefly state the context you gathered before proposing or implementing anything.

### When context is missing

If you cannot find enough context:
- Ask the orchestrator or user for clarification (use `question` tool).
- State your assumptions explicitly in your response.
- Never proceed with guesses on ambiguous requirements.
