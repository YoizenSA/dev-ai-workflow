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
| `test-ui` | Run only control-server UI tests (`go test ./internal/kanban/... ./internal/control/... -v`) | After touching UI/server code |
| `build` | Quick build WITHOUT embedded data | Fast iteration during dev |
| `build-full` | Full build WITH embedded skills/agents | Before pushing |
| `install` | Build-full + install to `$GOPATH/bin/ywai` | To test with opencode |
| `check` | Full pipeline: test → build-full → verify → install | **Before pushing to main** |
| `ui` | Build + install + start the control UI on port 5768 | To visually test the UI |
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

**Testing UI changes:**
```bash
cd ywai && bash scripts/dev.sh test-ui
```

**Visual UI testing:**
```bash
cd ywai && bash scripts/dev.sh ui
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
| `ywai update` | Upgrade gentle-ai + re-seed + re-link skills |
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
| `--mcp` | Install Microsoft Learn MCP (for opencode) |
| `--ponytail` | Install ponytail (YAGNI / minimal-code): OpenCode/kilocode plugin array + Claude Code marketplace; default off |
| `--global` | Install global skills only (skip AGENTS.md/REVIEW.md in project) |

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
| Oh My Pi (OMP) | `omp` | Binary `omp` in PATH or `~/.omp/agent/` |

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
│   ├── control/          # Unified web server (config API + missions + workflows)
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

A visual multi-agent workflow editor that designs workflows on a React Flow canvas and **exports them to opencode's native primitives**.

**Where it lives:** the `/workflows` route in the control UI (`http://localhost:5768/workflows`).

**How it works:**

1. **Design** a workflow on the canvas: drag nodes (SubAgent, AskUserQuestion, Prompt, If/Else, Switch, Skill, MCP, Group) from the palette and connect them.
2. **Edit** each node's fields in the side panel (system prompt, task prompt, tools, model, options, conditions…).
3. **Validate** the graph (structural rules: one start/end, no cycles, field limits, reachability).
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

**Import:** paste or upload a `workflow.json` — the format round-trips on re-export. Missing start/end nodes are added automatically.

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

<!-- graft:start -->
## Graft — repo context graph

This repo is indexed in `graft/`: small linked markdown nodes that explain each
system and carry exact file:line spans, kept in sync with the code through git.

For ANY task here — understanding how something works, finding where code lives,
or scoping a change — get context from the graph before grepping or opening
source files. Re-ask freely (it's cheap) and reuse literal identifiers you
already have (symbol, error string, file name) as the query. New to this repo?
Run `graft map` first — a token-budgeted orientation (dir clusters, hubs,
hotspots), no LLM, no key.

- Run `graft ask "<your question>" --source` → ranked nodes with the relevant
  code spans inlined (each hit's ≤8-line crux by default; `--full` for whole
  definitions when the crux isn't enough). Match the tool to the task shape:
  for understanding or editing, the top node IS the answer — cite its
  `covers:` file:line spans and edit straight from `--source`. For
  exhaustive tasks ("every occurrence / every caller of this pattern"), ranked
  results are top-N, not complete — run `graft grep "<literal>"` instead
  (exhaustive over indexed files, grouped by enclosing symbol), falling back
  to raw `grep -rn` only for unindexed files.
- `graft skeleton <file>` → every definition's signature + span, ~10× cheaper
  than reading the file; use it to skim an API surface.
- `graft callers <symbol>` gives precomputed, exact edges — who calls this.
  Add `--direction out` for what it calls, or `--depth N` to walk
  transitively for the full blast radius. For structural questions, skip
  ranking and use this directly.
- Or browse: `graft/INDEX.md` lists every node; follow the links.
- Monorepos and folders of multiple repos rank fairly across sub-projects —
  hits carry `[scope/]` labels naming which one they're from. Narrow with
  `graft ask "<task>" --in <scope>/` once you know where you're working.

If a returned span is truncated ("+N more lines"), open the file at that exact
range before finalizing. Only open source files when a node genuinely lacks a
needed detail, and then at the exact file:line the node points to — never
re-read whole files.

After big code changes, refresh the graph with `graft build` (deterministic,
no API key, $0).
<!-- graft:end -->
