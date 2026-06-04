# ywai Agents

Pre-configured agent profiles for different roles. Each agent has a focused system prompt and tool configuration.

## Available Agents

| Agent | Role | Best For |
|-------|------|----------|
| `orchestrator` | Technical Lead | Multi-step goals: plan → test/implement → review → ship via delegation |
| `ask` | Research & Q&A | Quick questions, explanations, research, analysis |
| `dev` | Developer | Implementation, coding, debugging, refactoring |
| `qa` | QA Engineer | Testing, test strategy, quality assurance |
| `architect` | Architect | Design decisions, patterns, system design |
| `reviewer` | Code Reviewer | PR reviews, code quality, security audits |
| `devops` | DevOps Engineer | CI/CD, deployments, infrastructure, monitoring |

## Delegation Flow

The `orchestrator` is a `primary` agent that owns a goal and delegates to the
specialist subagents, collecting a standard **handoff** from each before deciding
the next step.

```
User → @orchestrator (goal)
  1. PLAN      → @architect   (design / plan, ADR if needed)
  2. ¿TDD?     → orchestrator asks the user (question tool)
       yes → @qa writes failing tests → @dev makes them pass → @qa validates
       no  → @dev implements → @qa adds tests after
  3. REVIEW    → @reviewer   (approve / request changes → back to @dev)
  4. DEPLOY    → @devops      (optional: CI/CD, container, deploy)
  5. CLOSE     → orchestrator summarizes delivered work
```

On opencode the orchestrator delegates asynchronously with the native
`delegate` / `delegation_list` / `delegation_read` tools, asks branching
decisions with `question`, and tracks the plan with `todowrite`. On other
agents it falls back to `@mention` routing. Each subagent ends its turn with a
`## Handoff (report back to @orchestrator)` block (status, did, artifacts, next
suggested, notes/risks).

## Config Format

Each agent directory contains:

```
agents/
├── ask/
│   ├── AGENT.md        # System prompt (required)
│   ├── tools.json      # Allowed tools (optional)
│   └── skills.txt      # Linked skills (optional)
├── dev/
│   └── ...
```

### AGENT.md

The main system prompt. Uses the same SKILL.md frontmatter format:

```yaml
---
name: dev
description: Implementation-focused developer agent
role: developer
tools: [Read, Edit, Write, Bash, Glob, Grep]
---
```

### tools.json (optional)

Override which tools the agent can use:

```json
{
  "allowed": ["Read", "Edit", "Write", "Bash", "Glob", "Grep"],
  "denied": []
}
```

### skills.txt (optional)

Skills to link when this agent is active (one per line):

```
typescript
react-19
tailwind-4
```

## Usage with ywai

```bash
# Install with a specific agent profile
ywai install --agent opencode --profile dev

# Or use the agent prompt directly
cat ywai/agents/dev/AGENT.md
```

## Philosophy

- **Focused**: Each agent has a clear, narrow role
- **Opinionated**: Strong defaults that work out of the box
- **Composable**: Agents can reference skills for domain-specific knowledge
- **Portable**: Works across opencode, claude-code, cursor, windsurf, etc.
