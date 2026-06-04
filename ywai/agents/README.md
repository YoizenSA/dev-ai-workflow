# ywai Agents

Pre-configured agent profiles for different roles. Each agent has a focused system prompt and tool configuration.

## Available Agents

| Agent | Role | Best For |
|-------|------|----------|
| `orchestrator` | Technical Lead | Multi-step goals: plan → test/implement → review → ship via delegation |
| `ask` | Research & Q&A | Quick questions, explanations, research, analysis |
| `finder` | Codebase Explorer | Search, navigate, and explore files and code (read-only) |
| `dev` | Developer | Implementation, coding, debugging, refactoring |
| `qa` | QA Engineer | Testing, test strategy, quality assurance |
| `architect` | Architect | Design decisions, patterns, system design |
| `reviewer` | Code Reviewer | PR reviews, code quality, security audits |
| `devops` | DevOps Engineer | CI/CD, deployments, infrastructure, monitoring |

## Delegation Flow

The `orchestrator` is a `primary` agent that owns a goal and delegates to the
specialist subagents, collecting a standard **handoff** from each before deciding
the next step.

```mermaid
graph TD
    U[User] -->|goal| O[orchestrator]

    O -->|PLAN| A[architect]
    A -->|handoff| O

    O -->|¿TDD?| Q{TDD?}
    Q -->|yes| QA1[qa: write failing tests]
    QA1 -->|handoff| O
    O -->|IMPLEMENT| D1[dev: make tests pass]
    D1 -->|handoff| O
    O -->|VALIDATE| QA2[qa: run + extend coverage]
    QA2 -->|handoff| O

    Q -->|no| D2[dev: implement feature]
    D2 -->|handoff| O
    O -->|TEST| QA3[qa: add tests after]
    QA3 -->|handoff| O

    O -->|REVIEW| R[reviewer]
    R -->|approve| O
    R -->|request changes| D3[dev: fix]
    D3 -->|handoff| R

    O -->|DEPLOY?| DO[devops]
    DO -->|handoff| O

    O -->|CLOSE| U[summary]

    %% Fan-out annotation
    O -.->|fan-out: parallel @dev slices| D1
    O -.->|fan-out: parallel @dev slices| D2
    O -.->|fan-out: parallel @dev slices| D3

    %% Statusline plugin
    SL[sub-agent-statusline plugin]
    SL -.->|visibility: running/completed/failed| O
    SL -.->|visibility: running/completed/failed| A
    SL -.->|visibility: running/completed/failed| D1
    SL -.->|visibility: running/completed/failed| D2
    SL -.->|visibility: running/completed/failed| D3
    SL -.->|visibility: running/completed/failed| QA1
    SL -.->|visibility: running/completed/failed| QA2
    SL -.->|visibility: running/completed/failed| QA3
    SL -.->|visibility: running/completed/failed| R
    SL -.->|visibility: running/completed/failed| DO
```

**Key points:**
- The orchestrator owns the goal and decides the next step from each handoff.
- TDD branch: `@qa` writes failing tests → `@dev` makes them pass → `@qa` validates.
- Fan-out: the orchestrator can spawn multiple `@dev` (or `@qa`) in parallel for disjoint workstreams.
- Each subagent ends with a `## Handoff (report back to @orchestrator)` block.
- The `sub-agent-statusline` plugin (installed automatically with `ywai install`) gives real-time visibility into running/completed/failed subagents, elapsed time, and token/context usage.

On opencode the orchestrator delegates with two tools from the `background-agents`
plugin: synchronous **`task`** for the sequential spine (each phase needs the
previous handoff) and asynchronous **`delegate`** for fan-out / parallel work
(read results with `delegation_read` when a `<task-notification>` arrives — never
poll). It asks branching decisions with `question` and tracks the plan with
`todowrite`. On other agents it falls back to `@mention` routing.

## Config Format

Each agent directory contains:

```
agents/
├── ask/
│   ├── AGENT.md        # System prompt (required)
│   ├── permissions.json # Tool permissions (optional)
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
mode: all
---
```

### permissions.json (optional)

Configure which tools the agent can use. Valid values are `allow`, `ask`, or `deny`:

```json
{
  "read": "allow",
  "edit": "allow",
  "write": "allow",
  "bash": "allow",
  "glob": "allow",
  "grep": "allow"
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
