# AI Development Workflow - Project Index

## Overview

This repository contains AI-assisted development workflows and agent skills for software engineering.

| Directory | Description |
|:---|:---|
| `ywai/` | AI development workflow with SDD Orchestrator (Spec Driven Development) |
| `.agents/skills` | Global agent skills (skill-creator) |

---

## Quick Start

### Install AI Workflow (recommended)

```bash
# macOS / Linux
curl -sSL https://raw.githubusercontent.com/Yoizen/dev-ai-workflow/main/ywai/setup/setup.sh | bash -s -- --all --type=nest

# Windows
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/Yoizen/dev-ai-workflow/main/ywai/setup/quick-setup.ps1))) -All -Type nest
```

See [ywai/README.md](ywai/README.md) for full installation options.

---

## Project Structure

```
dev-ai-workflow/
├── ywai/                         # Main AI workflow
│   ├── README.md                 # User documentation
│   ├── skills/                   # AI agent skills
│   │   ├── sdd-*/                # SDD Orchestrator skills
│   │   ├── git-commit/
│   │   ├── biome/
│   │   ├── react-19/
│   │   ├── typescript/
│   │   ├── angular/
│   │   ├── dotnet/
│   │   ├── python/
│   │   └── skill-creator/
│   ├── setup/                     # Auto-setup scripts
│   │   ├── types/                # Project type configs
│   │   │   ├── generic/
│   │   │   ├── nest/
│   │   │   ├── python/
│   │   │   └── dotnet/
│   │   └── setup.sh
│   ├── commands/                 # Slash command docs
│   └── hooks/                    # Git hooks
│
└── .agents/                      # Global agent config
    └── skills/
        ├── extension-creator/
        └── skill-creator/
```

---

## Available Skills

### SDD Orchestrator (Spec Driven Development)

| Skill | Purpose |
|:---|:---|
| `sdd-init` | Bootstrap `.sdd/` structure |
| `sdd-explore` | Explore ideas before committing |
| `sdd-propose` | Create change proposal |
| `sdd-spec` | Write specifications |
| `sdd-design` | Technical design document |
| `sdd-tasks` | Break change into tasks |
| `sdd-apply` | Implement tasks |
| `sdd-verify` | Validate implementation vs specs |
| `sdd-archive` | Archive completed change |
| `sdd-onboard` | Guided end-to-end SDD walkthrough on a real codebase |
| `judgment-day` | Parallel adversarial review with two blind judges + fix/re-judge loop |

### Technology Skills

| Skill | Technology |
|:---|:---|
| `typescript` | TypeScript |
| `react-19` | React 19 |
| `tailwind-4` | Tailwind CSS 4 |
| `biome` | Biome (linter/formatter) |
| `angular/*` | Angular (core, forms, performance, architecture) |
| `dotnet` | .NET / C# |
| `python` | Python |
| `devops` | Azure Pipelines, Helm charts, Kubernetes deployments |

### Meta Skills

| Skill | Purpose |
|:---|:---|
| `skill-creator` | Create new AI agent skills |
| `extension-creator` | Create and wire new setup extensions |
| `global-agents` | Create/update global agents templates, bundles, and skills invoke sync |
| `skill-sync` | Sync skill metadata with AGENTS.md |
| `git-commit` | Conventional commits |

---

## Usage

### Agent Mode (simple tasks)

```text
> Agrega validación de email en el form de registro
```

### SDD Mode (complex features)

```bash
sdd:new feature-name     # Create proposal
sdd:ff feature-name      # Fast-forward: spec + design + tasks
/sdd-apply               # Implement tasks
git commit               # Auto-review with GA
```

---

## Documentation

- **User Guide**: [ywai/README.md](ywai/README.md)
- **SDD Commands**: [ywai/commands/](ywai/commands/)
- **Skills Reference**: [ywai/skills/](ywai/skills/)
- **Project Types**: [ywai/setup/types/](ywai/setup/types/)

---

## GitHub

- Issues: https://github.com/Yoizen/dev-ai-workflow/issues
- Repository: https://github.com/Yoizen/dev-ai-workflow
