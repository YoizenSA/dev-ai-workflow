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
| `--mcp` | Install Microsoft Learn MCP (for opencode/kilocode) |
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
│   ├── gentlai/          # gentle-ai wrapper (install, sync, upgrade, doctor)
│   ├── skills/           # Symlink extra skills to agent dirs
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
