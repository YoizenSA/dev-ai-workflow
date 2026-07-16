# ywai — One command to set up your AI dev environment

Wrapper around [gentle-ai](https://github.com/Gentleman-Programming/gentle-ai) that adds extra skills, project templates, and one-command install/update.

---

## Quick Start

### Con installer

```powershell
# Windows (PowerShell)
irm https://github.com/Yoizen/dev-ai-workflow/releases/latest/download/install.ps1 | iex
```

```bash
# macOS / Linux
curl -fsSL https://github.com/Yoizen/dev-ai-workflow/releases/latest/download/install.sh | bash
```

### Con Go desde el repo clonado

```bash
cd ywai
bash scripts/prepare-embedded.sh
go install -tags embedded ./cmd/ywai
```

### Uso

```bash
ywai install                  # Install gentle-ai + ecosystem + all extra skills
ywai install --dry-run        # Preview changes without applying
ywai install --agent opencode  # Install for specific agent
ywai update                   # Self-update + sync + re-link
ywai skills                   # List extra skills
```

---

## Commands

| Command | Description |
|---------|-------------|
| `ywai install` | Install gentle-ai + ecosystem + extra skills + project init |
| `ywai update` | Self-update + upgrade + sync + re-seed + re-link + rename orchestrator |
| `ywai init <type>` | Copy AGENTS.md/REVIEW.md for a project type |
| `ywai skills` | List available extra skills |
| `ywai agents` | List detected AI agents |
| `ywai status` | Show ywai installation status |
| `ywai config` | Manage ywai configuration |
| `ywai doctor` | Run gentle-ai health check |
| `ywai skill-registry` | Refresh the project skill registry |
| `ywai serve` | Start the Control UI server (Kanban + Missions on port 5768) |
| `ywai ui` | Open Control UI in the default browser |
| `ywai daemon` | **Deprecated** — use `ywai serve` instead |

### Install flags

| Flag | Description |
|------|-------------|
| `--type, -t` | Project type (react, nest, dotnet, etc.) |
| `--agent, -a` | Specific agent (auto-detects if omitted) |
| `--dry-run` | Preview changes without applying |
| `--mcp` | Install Microsoft Learn MCP (for opencode) |
| `--ponytail` | Install ponytail (YAGNI / minimal-code): OpenCode plugin + Claude Code marketplace; default off |
| `--autostart` | Configure control server to start automatically on system boot |

### Configuration

ywai stores configuration in `~/.ywai/config.yaml`. Use the `config` command to manage it:

```bash
ywai config get                    # Show all configuration
ywai config get default_preset    # Get specific value
ywai config set default_preset minimal  # Set a value
ywai config reset                 # Reset to defaults
```

Available configuration options:
- `default_preset`: Installation preset (full-gentleman, ecosystem-only, minimal, custom)
- `default_sdd_mode`: SDD orchestrator mode (single, multi)
- `default_persona`: Agent persona (gentleman, neutral, custom)
- `default_scope`: Install scope (global, workspace)
- `default_tui`: Use TUI by default (true/false)
- `default_mcp`: Install MCP by default for opencode (true/false)
- `default_ponytail`: Install ponytail plugin by default for opencode/kilocode (true/false)
- `colored_output`: Use colored output (true/false)
- `log_level`: Logging level (debug, info, warn, error)
- `server.port`: Default port for the control server (number, default `5768`)
- `server.background`: Run server in background by default (true/false)
- `server.mcp`: Start MCP adapter by default (true/false)
- `server.autostart`: Configure autostart on system boot (true/false)

---

## Control UI

`ywai serve` unifica Kanban + Missions en un solo servidor web con una SPA (React + Vite). Incluye autostart vía systemd/launchd, health checks, y un adaptador MCP opcional.

### Quick start

```bash
# Start the server (blocking, foreground)
ywai serve

# Start in background
ywai serve --background

# Open in browser (once the server is running)
ywai ui

# Configure autostart on boot (macOS launchd / Linux systemd)
ywai install --autostart
```

### ywai serve flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port, -p` | `5768` | Port for the control server |
| `--background, -b` | `false` | Run in background (detach from terminal) |
| `--no-mcp` | `false` | Don't start the MCP adapter |
| `--mcp-only` | `false` | Run as MCP adapter only (stdio JSON-RPC, no HTTP) |

### What you get

| Endpoint | Description |
|----------|-------------|
| `http://localhost:5768/` | Kanban board — track delegations, sessions, and activities |
| `http://localhost:5768/missions/` | Missions dashboard — plan and monitor multi-feature projects |
| `http://localhost:5768/health` | Health check endpoint |
| `http://localhost:5768/memories/` | Engram memory management — search, edit, consolidate, timeline |

### Autostart

| OS | Mechanism | File |
|----|-----------|------|
| macOS | launchd | `~/Library/LaunchAgents/com.ywai.server.plist` |
| Linux | systemd | `~/.config/systemd/user/ywai-server.service` |
| Windows | Registry | `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` |

Check status: `ywai config get server.autostart`

### Deprecated

- `ywai daemon` → use `ywai serve` instead
- `ywai missions serve` → use `ywai serve` instead (both Kanban and Missions are served on the same port)

---

## Missions

`ywai missions` — Multi-agent orchestration system for large, multi-feature projects. Breaks complex work into milestones and features, delegates each feature to an `opencode` worker agent, and validates results against expected behaviors.

### Commands

| Command | Description |
|---------|-------------|
| `ywai missions start` | Start interactive mission planning |
| `ywai missions start --file plan.json` | Start mission from a JSON plan file |
| `ywai missions list` | List all missions |
| `ywai missions show <id>` | Show detailed mission information |
| `ywai missions resume <id>` | Resume a paused mission |
| `ywai missions cancel <id>` | Cancel a mission |

### Architecture

- **Package**: `ywai/internal/missions/` — self-contained module with CLI, TUI, and Web UI surfaces
- **State Machine**: Missions and features follow strict FSM transitions (`fsm.go`) — `planning → active → paused / completed / failed / validating`
- **Persistence**: JSON files stored at `~/.local/share/ywai/missions/{id}/` with atomic writes for crash safety
- **Worker Agent**: Uses `opencode` as the worker agent for feature implementation. Each feature gets a dedicated temp directory with its spec, mission context, and handoff format
- **Validation Pipeline**: Per-milestone validation runs structural scrutiny, automated user testing, and generates fix features for blocking issues
- **Dashboard**: Embedded Web UI (`internal/missions/web/ui/`) with WebSocket real-time updates, CRUD operations, and live feature log streaming
- **Plan File**: Missions can be bootstrapped from a `plan.json` file (see `PlanMission` schema in `models.go`)

### State Lifecycle

```
planning ──► active ──► paused ──► active
                  ├──► completed
                  ├──► failed ──► planning
                  └──► validating ──► completed / failed
                                    └──► active (re-enter after fixes)
```

---

## 15 Supported Agents

opencode, claude-code, cursor, windsurf, gemini-cli, vscode-copilot, codex, kilocode, kimi, qwen-code, antigravity, kiro-ide, openclaw, trae-ide, pi

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
