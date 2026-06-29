# ywai — One command to set up your AI dev environment

## Overview

**ywai** is a CLI wrapper around [gentle-ai](https://github.com/Gentleman-Programming/gentle-ai) that adds:

- Extra skills not in gentle-ai (React 19, Angular, Tailwind 4, TypeScript, etc.)
- One-command install + update workflow

**What ywai does NOT do**: reimplement gentle-ai. It **delegates** to `gentle-ai install`, `gentle-ai sync`, etc.

---

## Quick Start

```bash
# Install ywai (macOS / Linux)
curl -fsSL https://github.com/Yoizen/dev-ai-workflow/releases/latest/download/install.sh | bash

# Or install from a source checkout
cd ywai
bash scripts/prepare-embedded.sh
go install -tags embedded ./cmd/ywai

# Full install: gentle-ai + ecosystem + extra skills
ywai install

# Preview changes
ywai install --dry-run

# Specific agent with preset
ywai install --agent opencode --preset full-gentleman

# Update everything
ywai update
```

---

## Local Development

The `dev.sh` script wraps all local build/test workflows so you don't have to remember the commands.

### Subcommands

| Subcommand | What it does | When to use |
|---|---|---|
| `test` | Run all tests (`go test ./... -v`) | Before every commit |
| `test-kanban` | Run only kanban tests (`go test ./internal/kanban/... -v`) | After touching kanban code |
| `build` | Quick build WITHOUT embedded data | Fast iteration during dev |
| `build-full` | Full build WITH embedded skills/agents | Before pushing |
| `install` | Build-full + install to `$GOPATH/bin/ywai` | To test with opencode |
| `check` | Full pipeline: test → build-full → verify → install | **Before pushing to main** |
| `kanban` | Build + install + start kanban UI on port 5768 | To visually test the kanban board |
| `mcp-test` | Build + install + send test JSON-RPC to MCP daemon | After changing MCP protocol |
| `version` | Print the current dev version string | Debug |
| `help` | Show all available subcommands | Reference |

### Typical workflows

**Before every commit:**
```bash
cd ywai && bash scripts/dev.sh check
```

**Quick iteration loop (no embedded):**
```bash
cd ywai && bash scripts/dev.sh build
```

**Testing kanban changes:**
```bash
cd ywai && bash scripts/dev.sh test-kanban
```

**Visual kanban testing:**
```bash
cd ywai && bash scripts/dev.sh kanban
# Opens http://localhost:5768
```

### Notes

- The script auto-detects the project root (looks for `go.mod` with module `github.com/Yoizen/dev-ai-workflow/ywai`), so you can run it from any subdirectory
- Two build modes exist: `build` (fast, reads skills from disk) vs `build-full` (bundles skills into the binary, like production)
- After `install`, restart your AI agent (opencode, etc.) to pick up the new binary

---

## Commands

| Command | Description |
|---------|-------------|
| `ywai install` | Install gentle-ai + ecosystem + all extra skills |
| `ywai update` | Upgrade gentle-ai + sync + re-link skills |
| `ywai skills` | List available extra skills |
| `ywai agents` | List detected AI agents |
| `ywai doctor` | Run gentle-ai health check |
| `ywai skill-registry` | Refresh project skill registry |

### Install flags

| Flag | Description |
|------|-------------|
| `--agent, -a` | Specific agent (auto-detects if omitted) |
| `--dry-run` | Preview changes without applying |
| `--preset` | Install preset: `full-gentleman` (default), `ecosystem-only`, `minimal`, `custom` |
| `--scope` | Install scope: `global` (default) or `workspace` |
| `--sdd-mode` | SDD orchestrator mode: `single` or `multi` |
| `--persona` | Persona: `gentleman`, `neutral`, `custom` |
| `--mcp` | Install Microsoft Learn MCP (for opencode) |
| `--ado` | Install Azure DevOps plugin (opencode + pi) |
| `--global` | Install global skills only (skip AGENTS.md/REVIEW.md in project) |

### Update flags

| Flag | Description |
|------|-------------|
| `--sdd-mode` | SDD orchestrator mode: `single` or `multi` |
| `--strict-tdd` | Enable Strict TDD Mode for SDD agents |
| `--include-permissions` | Include permissions in sync |
| `--include-theme` | Include theme in sync |

### Skill registry flags

| Flag | Description |
|------|-------------|
| `--cwd` | Project directory (defaults to current) |

---

## Supported Agents

| Agent | ID | Detection |
|-------|----|-----------|
| OpenCode | `opencode` | Binary in PATH |
| Claude Code | `claude-code` | Binary in PATH |
| Cursor | `cursor` | Binary in PATH |
| Gemini CLI | `gemini-cli` | Binary in PATH |
| VS Code Copilot | `vscode-copilot` | Binary in PATH |
| Codex | `codex` | Binary in PATH |
| Kilo Code | `kilocode` | Binary in PATH |
| Kimi Code | `kimi` | Binary in PATH |
| Qwen Code | `qwen-code` | Binary in PATH |
| Antigravity | `antigravity` | Config dir `~/.gemini/antigravity/` |
| Kiro IDE | `kiro-ide` | Binary in PATH |
| OpenClaw | `openclaw` | Binary in PATH |
| Trae IDE | `trae-ide` | Config dir `~/.trae/` |
| Windsurf | `windsurf` | Config dir `~/.codeium/windsurf/` |
| Pi | `pi` | Binary in PATH |

---

## Project Structure

```
ywai/
├── agents/               # Pre-configured agent profiles
│   ├── ask/              # Research & Q&A
│   ├── dev/              # Implementation
│   ├── qa/               # Testing & quality
│   ├── architect/        # Design & architecture
│   ├── reviewer/         # Code review
│   └── devops/           # CI/CD & infrastructure
├── cmd/ywai/             # CLI entry point
├── internal/
│   ├── agent/            # Agent detection (15 supported agents)
│   ├── agents/           # Profile loader, installers, delegations
│   ├── control/          # Unified web server (kanban + missions + workflows)
│   ├── gentlai/          # gentle-ai wrapper (install, sync, upgrade, doctor)
│   ├── skills/           # Symlink extra skills to agent dirs
│   ├── workflows/        # Workflow Studio: model, store, validator, exporter
│   └── config/           # Paths, constants
├── skills/               # Extra skills not in gentle-ai
│   ├── angular/
│   ├── biome/
│   ├── devops/
│   ├── dotnet/
│   ├── git-commit/
│   ├── playwright/
│   ├── react-19/
│   ├── tailwind-4/
│   ├── typescript/
│   └── yz-ui/
├── go.mod
├── .goreleaser.yaml
├── AGENTS.md
└── README.md
```

---

## Workflow Studio

A visual multi-agent workflow editor (inspired by [cc-wf-studio](https://github.com/breaking-brake/cc-wf-studio)) that designs workflows on a React Flow canvas and **exports them to opencode's native primitives**.

**Where it lives:** the `/workflows` route in the control UI (`http://localhost:5768/workflows`).

**How it works:**

1. **Design** a workflow on the canvas: drag nodes (SubAgent, AskUserQuestion, Prompt, If/Else, Switch, Skill, MCP, Group) from the palette and connect them.
2. **Edit** each node's fields in the side panel (system prompt, task prompt, tools, model, options, conditions…).
3. **Validate** the graph (structural rules ported from cc-wf-studio: one start/end, no cycles, field limits, reachability).
4. **Export** to opencode — the workflow becomes real, runnable artifacts:

| Workflow element | opencode primitive | Output |
|---|---|---|
| Whole workflow (entry point) | Slash command | `~/.config/opencode/commands/<name>.md` (invoked as `/<name>`) |
| Orchestrator persona | Agent | `~/.config/opencode/agents/<name>-orchestrator.md` (Mermaid diagram + execution steps; delegates via native `task` tool) |
| `subAgent` nodes | Agents | `~/.config/opencode/agents/<name>-<slug>.md` (system prompt + permissions + task) |
| `askUserQuestion` / `ifElse` / `switch` | Routing instructions | embedded in the orchestrator's prompt body |

**Source of truth vs. generated output** (mirrors the agents profile split):
- Source (editable JSON): `~/.ywai/workflows/<name>.json`
- Generated (what opencode reads): `~/.config/opencode/{commands,agents}/`

**Import:** paste or upload a cc-wf-studio `workflow.json` — the formats are compatible and round-trip. Missing start/end nodes are added automatically.

**Backend:** `internal/workflows/` (model, store, validator, exporter, importer) + `internal/control/workflows.go` (REST API at `/api/workflows`).

---

## Available Skills

| Skill | Technology |
|:---|:---|
| `typescript` | TypeScript |
| `react-19` | React 19 |
| `tailwind-4` | Tailwind CSS 4 |
| `biome` | Biome (linter/formatter) |
| `angular/*` | Angular (core, forms, performance, architecture) |
| `dotnet` | .NET / C# |
| `devops` | Azure Pipelines, Helm charts, Kubernetes |
| `playwright` | E2E testing (browser APIs, frameworks, CI/CD) |
| `git-commit` | Conventional commits |

---

## Pre-configured Agents

Role-based agent profiles in `ywai/agents/`. Each has a system prompt (`AGENT.md`), tool permissions (`tools.json`), and linked skills (`skills.txt`).

| Agent | Role | Best For |
|:------|:-----|:---------|
| `ask` | Research & Q&A | Quick questions, explanations, research, analysis |
| `dev` | Developer | Implementation, coding, debugging, refactoring |
| `qa` | QA Engineer | Test strategy, writing tests, coverage analysis |
| `architect` | Architect | Design decisions, patterns, system architecture |
| `reviewer` | Code Reviewer | PR reviews, bug finding, security audits |
| `devops` | DevOps Engineer | CI/CD, deployments, Docker, K8s, monitoring |

### Agent Composability

```
ask → (research) → architect → (design) → dev → (implement) → qa → (test) → reviewer → (approve) → devops → (deploy)
```

---

## GitHub

- Issues: https://github.com/Yoizen/dev-ai-workflow/issues
- Repository: https://github.com/Yoizen/dev-ai-workflow
- Upstream: https://github.com/Gentleman-Programming/gentle-ai
