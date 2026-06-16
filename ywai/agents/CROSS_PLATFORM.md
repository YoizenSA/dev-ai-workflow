# Core Agents — Cross-Platform Polish + PI.dev Installer

## Context

The 8 core agents in `agents/core/` (`orchestrator, ask, dev, qa, architect, reviewer, devops, finder`) are authored once as `AGENT.md` + `permissions.json` (+ optional `skills.txt`) and transformed by the Go loader (`internal/agents/agents.go`) into each host's native agent format.

Two problems motivate this change:

1. **PI.dev compatibility is broken at install time.** PI.dev (`pi`) is a registered agent in `KnownAgents` (`internal/agent/agent.go:130`) and receives skills, but the agent-profile install switch (`cmd/ywai/root.go:275-329`) only handles `opencode`, `kilocode`, `claude-code`, `cursor`, `vscode-copilot`. There is **no `InstallPi` function and no `case "pi"`**, so PI.dev never receives the core agents.

2. **The agent content is opencode-coupled and inconsistent.** The `orchestrator` prompt hardcodes OpenCode `background-agents` tool names (`task`, `delegate`, `delegation_read`, `delegation_list`, `todowrite`) and the `ywai-kanban` MCP as if always present. Frontmatter is inconsistent (`finder`/`ask` carry dead `tools:`/`permission:` keys the loader ignores; all 8 use `mode: all` despite 6 being subagents). Kanban trailers exist on some specialists and not others.

**Outcome:** PI.dev installs the core agents in its native format, and the agent prompts are portable (capability-based with explicit per-host adapters) and internally consistent — with no new heavyweight agent features added.

Scope: **content polish + PI.dev installer**. Depth: **cross-platform polish only** (no error-handling / verification-loop / ENGRAM feature roadmap from `IMPROVEMENTS.md`).

---

## PI.dev target format (verified from `~/.pi/agent/agents/*.md`)

```
---
name: <agent-name>
description: <one line>
tools: read, grep, glob, edit, write, bash
---

<prompt body>
```

- Frontmatter keys: `name`, `description`, `tools` (comma-separated, **lowercase**).
- No `mode`, no `temperature`, no nested `permission` block (unlike OpenCode markdown).
- Observed tool vocabulary: `read, edit, write, bash, glob, grep, webfetch`.

This mirrors the Claude format (`InstallClaude`) but with lowercase tool names and a different whitelist.

---

## Part A — PI.dev installer (Go)

### A1. `internal/agents/agents.go`

Add two functions, mirroring `InstallClaude` + `claudeToolsString` but for PI:

- **`piToolsString(perms map[string]string) string`** — renders enabled tools (value `allow` or `ask`) as a lowercase comma list, in stable order. Whitelist mapped to PI's vocabulary:
  `read, edit, write, bash, glob, grep, webfetch, websearch`.
  Fallback to `read, glob, grep` when none enabled (same safety default as `claudeToolsString`).
- **`InstallPi(agentsDir string, profiles map[string]AgentProfile, overwrite bool) error`** — writes `<name>.md` per profile into `agentsDir`. Frontmatter is `name` + `description` + `tools` (from `piToolsString`), body is `profile.Prompt`. Respect `overwrite` (skip existing unless true), following the `InstallOpenCodeMarkdown` pattern rather than the always-skip `InstallClaude` pattern, so `--overwrite-agents` works and PI's own `my-default.md`/`sdd-*.md` are never touched (core names don't collide).

Reuse the existing `stripFrontmatter` helper for the body.

### A2. `cmd/ywai/root.go`

In `installAgentProfiles` switch (line 275), add:

```go
case "pi":
    agentsDir := filepath.Join(home, ".pi", "agent", "agents")
    if err := agentprofiles.InstallPi(agentsDir, profiles, overwriteAgents); err != nil {
        fmt.Printf("  [%s] Warning: %v\n", a.Name, err)
    } else {
        fmt.Printf("  [%s] Agent profiles installed\n", a.Name)
    }
```

`overwriteAgents` already flows into this function; `pi` is already in the `agents` slice when selected. No other wiring needed.

### A3. Tests — `internal/agents/agents_test.go`

Following the existing `TestInstallOpenCodeMarkdown*` trio:
- `TestInstallPi` — writes a profile, asserts lowercase `tools:` frontmatter + `name`/`description` + body present, no `mode:`/`permission:` block.
- `TestInstallPiSkipsExisting` / `TestInstallPiOverwriteExisting` — verify the `overwrite` flag.
- `TestPiToolsString` — verify mapping (allow/ask included, deny excluded, lowercase, fallback).

---

## Part B — Cross-platform content polish (agent markdown)

Content edits flow to all hosts automatically (every installer writes `profile.Prompt` verbatim), so portability lives in the prompt text.

### B1. `core/orchestrator/AGENT.md` (the only deeply coupled agent)

Restructure delegation guidance from "OpenCode tool names" to **capability + per-host adapter**:

- Add a short **Delegation Capability Model**: abstract capabilities — *sync-delegate*, *async-delegate*, *read-async-result*, *ask-user*, *track-plan*, *track-board* — described by behavior, not tool name.
- Replace the `task` vs `delegate` prose with a **Platform Adapters** table:

  | Capability | OpenCode | Claude Code | PI.dev | Fallback |
  |---|---|---|---|---|
  | sync-delegate | `task` | `Agent`/`Task` | subagent task | `@mention` inline |
  | async-delegate | `delegate` | `Agent` (background) | subagent (background) | sequential `@mention` |
  | read-async-result | `delegation_read` | task result / `SendMessage` | subagent result | — |
  | ask-user | `question` | `AskUserQuestion` | ask inline | ask inline |
  | track-plan | `todowrite` | `TaskCreate`/`Update` | todo/inline | inline checklist |

- Gate the entire **Kanban Tracking** section behind "when the `ywai-kanban` MCP is available". Keep it, mark it optional.
- Keep untouched (already host-neutral): Delivery Flow FSM, Delegation Brief Format, Consuming Handoffs, Fan-out, Engine FSM Integration, Boundaries.

### B2. The 6 specialists + `ask` — consistency pass

Across `architect, dev, qa, reviewer, devops, finder, ask`:

- **Frontmatter cleanup:** keep only `name`, `description`, `role`, `mode`. Remove the dead `tools:` list (`finder`) and `permission:` block (`finder`, `ask`) — the loader ignores them; `permissions.json` is the single source of truth.
- **Mode correctness:** set the 6 specialists (`architect, dev, qa, reviewer, devops, finder`) to `mode: subagent` (their bodies already declare "You are a subagent"); keep `orchestrator` and `ask` as `mode: all` (primary-capable, still delegatable). Tradeoff: hosts stop exposing specialists as top-level primary pickers — the intended subagent model on Claude Code/OpenCode/PI.
- **Standardize the host-gated Kanban trailer:** the "Kanban Update" block on `architect/dev/qa/reviewer/devops` gets one consistent wording prefixed "When the orchestrator tracks a board (ywai-kanban present)…". `finder`/`ask` stay without it.
- Neutralize stray tool-name assumptions in bodies (most already capability-gated, e.g. finder's codegraph note — leave as-is since it self-checks availability).

### B3. Docs

- `README.md` — add a **Platform Compatibility** matrix (OpenCode / Claude Code / PI.dev / Cursor / VSCode-Copilot): native path + frontmatter shape + status. Update the "Portable" philosophy bullet to name PI.dev.
- `IMPROVEMENTS.md` — add a one-line "Resolved" entry noting the PI.dev installer gap is closed and the orchestrator is now capability-based.

---

## Files to change

- `internal/agents/agents.go` — add `InstallPi`, `piToolsString` (A1)
- `cmd/ywai/root.go` — add `case "pi"` (A2)
- `internal/agents/agents_test.go` — PI installer tests (A3)
- `agents/core/orchestrator/AGENT.md` — capability model + adapters + gated Kanban (B1)
- `agents/core/{architect,dev,qa,reviewer,devops,finder,ask}/AGENT.md` — frontmatter cleanup, mode, gated Kanban trailer (B2)
- `agents/README.md`, `agents/IMPROVEMENTS.md` — docs (B3)

Reuse: `stripFrontmatter`, `claudeToolsString` (shape template), `InstallOpenCodeMarkdown` (overwrite pattern), `TestInstallOpenCodeMarkdown*` (test template).

---

## Verification

1. **Unit:** `cd ywai && go test ./internal/agents/...` — new PI tests pass, existing pass.
2. **Build:** `cd ywai && go build ./...` — installer wiring compiles.
3. **Installer smoke (isolated):** drive `InstallPi` against a temp dir in the test (assert generated frontmatter is lowercase `tools:`, no `mode:`/`permission:`). Do not run `ywai install` against the real `~/.pi` during verification.
4. **Content sanity:** confirm each edited `AGENT.md` still parses — `description`/`mode` extract correctly and the body renders for Claude/OpenCode/PI installers.
5. **Cross-host shape check:** for one agent (e.g. `dev`), eyeball the three generated outputs (OpenCode `permission:` block, Claude `tools: Read,…`, PI `tools: read,…`) to confirm the same body produces three valid native formats.
