# ywai — One command to set up your AI dev environment

## Overview

**ywai** is a CLI wrapper around [gentle-ai](https://github.com/Gentleman-Programming/gentle-ai) that adds:

- Extra skills not in gentle-ai (React 19, Angular, Tailwind 4, TypeScript, etc.)
- Project templates (AGENTS.md + REVIEW.md by project type)
- One-command install + update workflow

**What ywai does NOT do**: reimplement gentle-ai. It **delegates** to `gentle-ai install`, `gentle-ai sync`, etc.

---

## Quick Start

```bash
# Install ywai
go install -tags embedded github.com/Yoizen/dev-ai-workflow/ywai/cmd/ywai@latest

# Full install: gentle-ai + ecosystem + extra skills
ywai install

# With project type
ywai install --type react

# Specific agent
ywai install --agent opencode --type nest

# Update everything
ywai update

# Initialize a project (AGENTS.md + REVIEW.md)
ywai init react
```

---

## Commands

| Command | Description |
|---------|-------------|
| `ywai install` | Install gentle-ai + ecosystem + extra skills + optional project init |
| `ywai update` | Upgrade gentle-ai + sync + re-link skills |
| `ywai init <type>` | Copy AGENTS.md/REVIEW.md for a project type |
| `ywai skills` | List available extra skills |

### Install flags

| Flag | Description |
|------|-------------|
| `--type, -t` | Project type (auto-discovered from project-types/) |
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
│   ├── project/          # Project type initialization
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
├── project-types/        # Templates by project type (add folder = new type)
│   ├── generic/
│   ├── react/
│   ├── nest/
│   ├── nest-angular/
│   ├── nest-react/
│   ├── dotnet/
│   ├── python/
│   ├── devops/
│   └── qa-playwright/
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
