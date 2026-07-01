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
| `memory` | Memory Specialist | Memory consolidation, deduplication, structured plans |
| `planning` | Planning | Plan mode: research → clarify → draft plan → approval gate (read-only until approved) |

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

The orchestrator uses a **capability model** with per-platform adapters. On opencode
it delegates via `task` (sync) and `delegate` (async), asks decisions with `question`,
and tracks plans with `todowrite`. On Claude Code it uses `Agent`/`Task` and
`TaskCreate`/`Update`. On PI.dev it uses subagent tools. All hosts fall back to
`@mention` routing when the native tool is unavailable.

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
├── sections/
│   ├── handoff.md          # Standard handoff format (core subagents → @orchestrator)
│   ├── handoff-qa.md       # Handoff format for qa-automation subagents (@qa-*)
│   ├── context-gathering.md # Context gathering protocol
│   └── tdd.md              # Test-driven development discipline (dev/qa roles)
```

Shared sections are appended to an agent's prompt at build time when referenced in the `sections:` frontmatter array (e.g. `sections: [handoff, context-gathering, tdd]`). A section named `foo` resolves to `sections/foo.md`; missing sections are skipped silently.

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

Configure which tools the agent can use. Valid values are `allow`, `ask`, or `deny`.
Keys follow a canonical order for consistency:

```json
{
  "read": "allow",
  "edit": "allow",
  "write": "allow",
  "bash": "allow",
  "glob": "allow",
  "grep": "allow",
  "lsp": "allow",
  "ast_grep": "allow",
  "websearch": "allow",
  "webfetch": "allow",
  "code_search": "allow",
  "task": "allow",
  "delegate": "allow",
  "question": "allow",
  "skill": "allow",
  "memory": "allow",
  "intercom": "allow",
  "mcp": "allow"
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

## Platform Compatibility

| Platform | Path | Frontmatter Shape | Status |
|---|---|---|---|
| OpenCode | `~/.config/opencode/agents/*.md` | `description`, `mode`, `permission:` block | ✅ Full support |
| Claude Code | `~/.claude/agents/*.md` | `name`, `description`, `tools:` (PascalCase) | ✅ Full support |
| PI.dev | `~/.pi/agent/agents/*.md` | `name`, `description`, `tools:` (lowercase) | ✅ Full support |
| Cursor | `~/.cursor/agents/*.md` | (same as Claude) | ✅ Full support |
| VS Code Copilot | `~/.config/Code/User/prompts/*.instructions.md` | `name`, `description`, `applyTo` | ✅ Full support |

## Philosophy

- **Focused**: Each agent has a clear, narrow role
- **Opinionated**: Strong defaults that work out of the box
- **Composable**: Agents can reference skills for domain-specific knowledge
- **Portable**: Works across opencode, claude-code, cursor, windsurf, PI.dev, etc.
