# ywai — One command to set up your AI dev environment

Wrapper around [gentle-ai](https://github.com/Gentleman-Programming/gentle-ai) that adds extra skills, project templates, and one-command install/update.

---

## Quick Start

### Con Go

```bash
go install github.com/Yoizen/dev-ai-workflow/ywai/cmd/ywai@latest
```

### Con installer (requiere release v0.1.0+)

```powershell
# Windows (PowerShell)
irm https://github.com/Yoizen/dev-ai-workflow/releases/latest/download/ywai_windows_amd64.zip -OutFile ywai.zip
Expand-Archive ywai.zip
```

```bash
# macOS (Apple Silicon)
curl -fsSL https://github.com/Yoizen/dev-ai-workflow/releases/latest/download/ywai_darwin_arm64.tar.gz | tar xz
sudo mv ywai /usr/local/bin/

# Linux (x86_64)
curl -fsSL https://github.com/Yoizen/dev-ai-workflow/releases/latest/download/ywai_linux_amd64.tar.gz | tar xz
sudo mv ywai /usr/local/bin/
```

### Uso

```bash
ywai install                  # Interactive wizard
ywai install --type react     # React profile
ywai install --type nest      # NestJS profile
ywai update                   # Self-update + sync + re-link
ywai init react               # Project init (AGENTS.md + REVIEW.md)
ywai skills                   # List extra skills
ywai skills --type react      # Skills for a profile
```

---

## Commands

| Command | Description |
|---------|-------------|
| `ywai install` | Install gentle-ai + ecosystem + extra skills + project init |
| `ywai update` | Self-update + upgrade + sync + re-seed + re-link + rename orchestrator |
| `ywai init <type>` | Copy AGENTS.md/REVIEW.md for a project type |
| `ywai skills` | List available extra skills |

### Install flags

| Flag | Description |
|------|-------------|
| `--type, -t` | Project type (react, nest, dotnet, etc.) |
| `--agent, -a` | Specific agent (auto-detects if omitted) |
| `--dry-run` | Preview changes without applying |

---

## 12 Supported Agents

opencode, claude-code, cursor, windsurf, gemini-cli, vscode-copilot, codex, kilocode, kimi, qwen-code, antigravity, kiro-ide

---

## Extra Skills (on top of gentle-ai)

| Skill | Technology |
|:---|:---|
| `typescript` | TypeScript strict patterns |
| `react-19` | React 19 + React Compiler |
| `tailwind-4` | Tailwind CSS 4 |
| `biome` | Biome linter/formatter |
| `angular/*` | Angular (core, forms, performance, architecture) |
| `dotnet` | .NET 9 / ASP.NET Core |
| `devops` | Azure Pipelines, Helm, Kubernetes |
| `playwright` | E2E testing |
| `git-commit` | Conventional commits |
| `yz-ui` | Yoizen UI design system |

---

## Project Types

| Type | Description | Extra Skills |
|------|-------------|-------------|
| `react` | React 19 frontend | react-19, tailwind-4, typescript, biome, playwright, git-commit |
| `nest` | NestJS backend | typescript, biome, playwright, git-commit |
| `nest-angular` | NestJS + Angular fullstack | angular, typescript, biome, playwright, git-commit |
| `nest-react` | NestJS + React fullstack | react-19, tailwind-4, typescript, biome, playwright, git-commit |
| `dotnet` | .NET / C# | dotnet, git-commit |
| `python` | Python backend | git-commit |
| `devops` | CI/CD, Docker, Helm | devops, git-commit |
| `qa-playwright` | QA / E2E testing | playwright, typescript, biome, git-commit |
| `generic` | Language-agnostic | all skills |

---

## What gentle-ai installs (via ywai)

| Component | What it does |
|-----------|-------------|
| **Engram** | Persistent cross-session memory (MCP server) |
| **SDD** | Spec-Driven Development — 11 skills + orchestrator |
| **Skills** | 21 ecosystem skills (SDD, branch-pr, issue-creation, etc.) |
| **Context7** | Latest framework docs via MCP |
| **Persona** | Agent personality injection (neutral) |
| **Permissions** | Auto-approve security defaults per agent |
| **Theme** | Kanagawa theme overlay |

**Not installed:** GGA (Gentleman Guardian Angel)

---

## Custom Agents (injected by ywai)

| Agent | What it does |
|-------|-------------|
| `sdd-orchestrator` | Renamed from `gentle-orchestrator` — SDD conductor |
| `ask` | Read-only Q&A — answers questions, never modifies code |

---

## Elegir el modo correcto

| Complejidad | Modo | Ejemplo |
|-------------|------|---------|
| Fix / tweak | Agent | "Arregla el typo en el header" |
| Feature clara | Agent | "Agrega boton de logout" |
| Feature que hay que pensar | Plan -> Agent | "Sistema de notificaciones" |
| Feature grande / multi-dia | SDD Orchestrator | "Migrar auth a OAuth2" |

---

## Comandos SDD Orchestrator

### Atajos

| Comando | Que hace |
|---------|----------|
| `sdd:new <nombre>` | Crea propuesta para un nuevo change |
| `sdd:ff <nombre>` | Fast-forward: propuesta + spec + diseno + tasks |

### Slash commands

| Comando | Que hace |
|---------|----------|
| `/sdd-init` | Inicializa el flujo SDD en el proyecto |
| `/sdd-explore` | Explora una idea antes de crear el change |
| `/sdd-propose` | Crea propuesta del change |
| `/sdd-spec` | Genera specs y requerimientos |
| `/sdd-design` | Genera diseno tecnico y decisiones de arquitectura |
| `/sdd-tasks` | Breakdown en tareas de implementacion |
| `/sdd-apply` | Implementa tareas del change activo |
| `/sdd-verify` | Verifica implementacion contra la spec |
| `/sdd-archive` | Archiva un change terminado |

---

## Ejemplo completo

```text
> /sdd-init
SDD inicializado para este repositorio.

> sdd:new login-con-google
Creando propuesta 'login-con-google'...
  -> sdd/changes/login-con-google/proposal.md

> sdd:ff login-con-google
Fast-forward: generando spec, diseno y tareas...
  -> sdd/changes/login-con-google/specs/auth/spec.md
  -> sdd/changes/login-con-google/design.md
  -> sdd/changes/login-con-google/tasks.md

> /sdd-apply
Implementando tarea 1.1: Agregar boton de login...
Implementando tarea 2.1: Configurar OAuth...
Implementando tarea 2.2: Manejar callback...
Todas las tareas completadas.
```

---

## Que modelo usar

| Tarea | Modelo recomendado | Por que |
|------|-------------------|---------|
| Planning / diseno | Opus 4.6 | Mejor razonamiento |
| Implementacion | Codex 5.3 / Sonnet 4.6 | Optimizado para codigo |
| Commits, PRs, docs | Gemini 3 Flash | Barato, suficiente para texto |
| Code review critica | Codex 5.3 | Detecta bugs sutiles |

---

## GitHub

- Issues: https://github.com/Yoizen/dev-ai-workflow/issues
- Repository: https://github.com/Yoizen/dev-ai-workflow
- Upstream: https://github.com/Gentleman-Programming/gentle-ai
