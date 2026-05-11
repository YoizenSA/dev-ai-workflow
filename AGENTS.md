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

# Specific agent
ywai install --agent opencode

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

### Install flags

| Flag | Description |
|------|-------------|
| `--agent, -a` | Specific agent (auto-detects if omitted) |
| `--dry-run` | Preview changes without applying |

---

## Project Structure

```
ywai/
├── cmd/ywai/             # CLI entry point
├── internal/
│   ├── agent/            # Agent detection (opencode, claude-code, etc.)
│   ├── gentlai/          # gentle-ai wrapper (install, sync, upgrade)
│   ├── skills/           # Symlink extra skills to agent dirs
│   ├── orchestrator/     # Orchestrator renaming (gentle-orchestrator → sdd-orchestrator)
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

## GitHub

- Issues: https://github.com/Yoizen/dev-ai-workflow/issues
- Repository: https://github.com/Yoizen/dev-ai-workflow
- Upstream: https://github.com/Gentleman-Programming/gentle-ai
