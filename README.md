# Workflow de desarrollo asistido por IA

Features:
- **Agent / Plan mode** para tareas chicas y medianas
- **SDD Orchestrator (SDD, Spec Driven Development)** para features grandes (spec + diseño + tasks + apply)
- **GA Review (GA, Guardian Agent)** para review automático en cada commit

---

## Pre-requisitos

### Común
- Un repo Git inicializado (o un proyecto donde vayas a instalarlo).
- `git` instalado y disponible en PATH.
- Acceso a GitHub (para descargar scripts desde `raw.githubusercontent.com`).

### macOS / Linux
- `bash`
- `curl`

### Windows
- PowerShell (recomendado PowerShell 5.1+ o PowerShell 7+).
- Permisos para ejecutar el comando de instalación (si tu política lo restringe, ajustá Execution Policy según tus prácticas internas).

---

## Instalación

### macOS / Linux

```bash
curl -sSL https://github.com/Yoizen/dev-ai-workflow/releases/latest/download/install.sh | bash
```

### Windows

```powershell
irm https://github.com/Yoizen/dev-ai-workflow/releases/latest/download/install.ps1 | iex
```

> El instalador descarga el binario, lo agrega al PATH y abre el wizard interactivo automáticamente.

### Otros tipos

Reemplazá `nest` por cualquiera de estos tipos:
- `nest-angular`
- `nest-react`
- `python`
- `dotnet`
- `devops`
- `generic`

### Presets (alcance de instalación)

`--preset` controla **cuánto** se instala, ortogonal a `--type` (que define **qué stack**).

| Preset | Incluye |
|--------|---------|
| `minimal` | SDD skills + `git-commit` + `skill-creator` + `skill-sync`. Sin GA, sin global agents, sin MCPs, sin hooks. |
| `standard` (default) | Comportamiento actual: bundle completo del `--type` + GA + `context7-mcp` + global agents. |
| `full` | `standard` + `engram-setup` + hooks opcionales habilitados. |

```bash
ywai --type=nest --preset=minimal    # solo SDD, sin GA
ywai --type=dotnet --preset=full     # todo
ywai --preset=standard               # default (igual que omitir el flag)
```

### Nota

> El setup instala OpenCode automáticamente si no está disponible.
> `--global-skills` configura perfiles globales de usuario para OpenCode/Copilot (no instala nada a nivel global del sistema y no crea agentes dentro del repo).
> Los agentes globales se generan desde `ywai/extensions/install-steps/global-agents/templates/` y no desde `AGENTS.md`.
> Además, cada agente global se genera con un bundle Agent-Skills definido en `ywai/extensions/install-steps/global-agents/bundles.json` (ej: `devops` -> skill `devops`).
> Los agentes globales invocan habilidades (skills) según el bundle configurado, lo que permite una mayor flexibilidad en la configuración de habilidades para cada tipo de proyecto.

### Sync inteligente para proyectos existentes (`--sync`)

Para proyectos existentes, usá `--sync` para obtener un reporte de cambios sugeridos que el LLM puede leer y aplicar selectivamente:

```bash
# Auto-detectar tipo de proyecto
ywai --sync

# Tipo específico
ywai --sync --type=nest-angular
```

**Output**: Reporte markdown con:
- Skills faltantes
- Skills actualizables
- Cambios sugeridos en AGENTS.md
- Cambios sugeridos en REVIEW.md
- Instrucciones paso a paso

**El sync NO hace cambios**, solo genera instrucciones. El LLM decide qué aplicar.

### Instalar skill específica (`--install-skill`)

Para instalar una skill individual con sus dependencias:

```bash
ywai --install-skill angular/signals
ywai --install-skill react-19
ywai --install-skill devops
```

El comando genera instrucciones de instalación incluyendo:
- Archivos a copiar
- Dependencias requeridas
- Pasos para actualizar AGENTS.md

### Uso con LLM

```text
User: "Use ywai --sync to update this repo"

LLM: [ejecuta ywai --sync]
     [lee el reporte]
     [aplica cambios selectivamente según preferencias del usuario]
```

Prompt sugerido para chats:

```text
Fetch the installation guide and follow it:
curl -s https://raw.githubusercontent.com/Yoizen/dev-ai-workflow/main/docs/guide/installation.md
```

---

## Versiones y Releases

La instalación usa GitHub Releases.

### Canal por defecto: `stable`

Instala la última release estable publicada.

### Opciones de versión

```bash
# Versión específica (incluye pre-releases)
YWAI_VERSION=v6.0.0-beta.1 curl -sSL https://github.com/Yoizen/dev-ai-workflow/releases/latest/download/install.sh | bash

# Canal latest
YWAI_CHANNEL=latest curl -sSL https://github.com/Yoizen/dev-ai-workflow/releases/latest/download/install.sh | bash
```

---

## Primer uso (SDD) en un repo

Seleccioná **Agent mode** y usá SDD Orchestrator:

```text
/sdd-init                  # Inicializa SDD en el repo (una sola vez)
sdd:new dark-mode          # Crea propuesta del change
sdd:ff dark-mode           # Fast-forward: genera spec + diseño + tareas
/sdd-apply                 # Implementa tareas pendientes
git commit                 # GA hace review automático
```

---

## Elegir el modo correcto

No siempre necesitás SDD. Usá el modo según complejidad:

### Tarea simple → Agent directo
Fixes rápidos, refactors chicos o tareas claras:

```text
[Agent mode]
> Agrega validación de email en el form de registro
```

### Tarea compleja → Plan → Agent
Cuando conviene pensar primero:

```text
[Plan mode]
> Necesito agregar autenticación con OAuth.
> Soportar Google y GitHub. Guardar sesión en cookies httpOnly.
> El usuario tiene que poder deslogearse desde cualquier página.
```

Luego:

```text
[Agent mode]
> Implementa el plan
```

### Feature grande → SDD Orchestrator
Cuando cruza múltiples archivos/sistemas o es multi-día:

```text
sdd:new sistema-de-pagos
sdd:ff sistema-de-pagos
/sdd-apply
```

SDD genera specs formales, diseño técnico, tareas, y trackea el progreso.

### Resumen

| Complejidad | Modo | Ejemplo |
|-------------|------|---------|
| Fix / tweak | Agent | "Arregla el typo en el header" |
| Feature clara | Agent | "Agrega botón de logout" |
| Feature que hay que pensar | Plan → Agent | "Sistema de notificaciones" |
| Feature grande / multi-día | SDD Orchestrator | "Migrar auth a OAuth2" |

---

## Qué modelo usar

| Tarea | Modelo recomendado | Por qué |
|------|-------------------|---------|
| Planning / diseño | **Opus 4.6** | Mejor razonamiento; piensa antes de actuar |
| Implementación (Agent) | **Codex 5.3** / **Sonnet 4.6** | Optimizado para código; rápido y preciso |
| Commits, PRs, docs | **Gemini 3 Flash** | Barato; suficiente para texto |
| Ajustes de UI/CSS | **Gemini 3.1 Pro** | Buen balance costo/calidad para visual |
| Code review básica | **Gemini 3 Flash** / **Haiku 4.5** | Económico para checks rutinarias |
| Code review crítica | **Codex 5.3** | Detecta bugs sutiles; entiende contexto |

Regla general:
- Modelo caro → pensar, planificar, revisar código crítico
- Modelo barato → ejecutar, commits, reviews rutinarias

---

## Comandos SDD Orchestrator (SDD)

### Atajos (recomendado)

| Comando | Qué hace |
|---------|----------|
| `sdd:new <nombre>` | Crea propuesta para un nuevo change (equivale a `/sdd-propose`) |
| `sdd:ff <nombre>` | Fast-forward: propuesta + spec + diseño + tasks |

### Slash commands individuales

| Comando | Qué hace |
|---------|----------|
| `/sdd-init` | Inicializa el flujo SDD en el proyecto |
| `/sdd-explore` | Explora una idea antes de crear el change |
| `/sdd-propose` | Crea propuesta del change |
| `/sdd-spec` | Genera specs y requerimientos |
| `/sdd-design` | Genera diseño técnico y decisiones de arquitectura |
| `/sdd-tasks` | Breakdown en tareas de implementación |
| `/sdd-apply` | Implementa tareas del change activo |
| `/sdd-verify` | Verifica implementación contra la spec |
| `/sdd-archive` | Archiva un change terminado |

---

## Ejemplo completo

```text
> /sdd-init
Copilot: SDD inicializado para este repositorio.

> sdd:new login-con-google
Copilot: Creando propuesta 'login-con-google'...
         → .sdd/changes/login-con-google/proposal.md

> sdd:ff login-con-google
Copilot: Fast-forward: generando spec, diseño y tareas...
         → .sdd/changes/login-con-google/specs/auth/spec.md
         → .sdd/changes/login-con-google/design.md
         → .sdd/changes/login-con-google/tasks.md

> /sdd-apply
Copilot: Implementando tarea 1.1: Agregar botón de login...
         [edita archivos]
         Implementando tarea 2.1: Configurar OAuth...
         [edita archivos]
         Implementando tarea 2.2: Manejar callback...
         [edita archivos]
         Todas las tareas completadas.

> git commit -m "feat: login con google"
GA Review: PASS
[main abc1234] feat: login con google
```

---

## Tipos de Proyecto (`--type`)

| Tipo | Descripción | Skills incluidas |
|------|-------------|-----------------|
| `nest` | NestJS backend (TypeScript, Clean Architecture) | git-commit, biome, typescript, skill-creator, skill-sync |
| `nest-angular` | NestJS + Angular fullstack | git-commit, biome, typescript, angular, tailwind-4, skill-creator, skill-sync |
| `nest-react` | NestJS + React fullstack | git-commit, biome, typescript, react-19, tailwind-4, skill-creator, skill-sync |
| `python` | Python backend / scripts (FastAPI, Django, scripts) | git-commit, skill-creator, skill-sync |
| `dotnet` | .NET / C# backend (ASP.NET Core, Clean Architecture) | git-commit, skill-creator, skill-sync |
| `devops` | DevOps / Platform workflows (CI/CD, Docker, Helm, Kubernetes) | git-commit, devops, skill-creator, skill-sync |
| `generic` | Genérico — language-agnostic | git-commit, skill-creator, skill-sync |

Cada tipo instala un `AGENTS.md` con reglas específicas del stack y un `REVIEW.md` con checklist de code review adaptado.

---

## Artefactos generados en `.ywai/`

El setup deja dos archivos en `.ywai/` del proyecto que los sub-agentes consumen para tener contexto de proyecto sin leer cada `SKILL.md` completo:

| Archivo | Qué es |
|---------|--------|
| `.ywai/sdd-models.json` | Modelo de IA recomendado por fase SDD (copia de `ywai/config/sdd-models.json`). Consumido por el orquestador SDD y por `ga review --phase=<fase>`. |
| `.ywai/skill-registry.md` | Registro compacto de las skills instaladas (5–15 bullets por skill extraídos de `## Critical Patterns` / `## Rules`). Reduce tokens en prompts de sub-agentes. |

Regenerar el skill registry manualmente:

```bash
./skills/skill-sync/assets/sync.sh --registry
```

## Sincronizar Skills con AGENTS.md

Si agregaste o modificaste skills, podés pedirle al agente que regenere la sección de Auto-invoke en tus `AGENTS.md`.

Prompt sugerido (Agent mode):

```text
Sincronizá las skills con los AGENTS.md del repo.
Usá la skill `skill-sync` y regenerá las tablas de Auto-invoke según el metadata actual de `skills/*/SKILL.md`.
```

---

## Review Automático (GA)

Cada commit pasa por review automático. Si querés skippearlo:

```bash
git commit --no-verify -m "wip: trabajo en progreso"
```

### Configurar las reglas de review

Editá `REVIEW.md` en la raíz de tu proyecto:

```markdown
# Reglas de Code Review

## TypeScript
- No usar `any`
- Usar `const` en vez de `let`

## React
- Solo componentes funcionales
- Todas las imágenes con alt

## Testing
- Toda feature nueva necesita tests
```

---

## Estructura del Proyecto

Después de instalar:

```text
mi-proyecto/
├── .ga                     # Config de GA
├── REVIEW.md               # Reglas de review
├── skills/                 # Skills de IA + SDD skills
│   ├── git-commit/
│   ├── biome/
│   ├── sdd-init/
│   ├── sdd-explore/
│   ├── sdd-propose/
│   ├── sdd-spec/
│   ├── sdd-design/
│   ├── sdd-tasks/
│   ├── sdd-apply/
│   ├── sdd-verify/
│   └── sdd-archive/
└── .vscode/
    └── settings.json
```

---

## Providers de IA

GA puede usar diferentes providers. Editá `.ga`:

```bash
PROVIDER="opencode"   # Default - OpenCode
PROVIDER="claude"     # Anthropic Claude
PROVIDER="gemini"     # Google Gemini
PROVIDER="ollama"     # Modelos locales
```

---

## Troubleshooting

**"Provider not found"**
```bash
which opencode  # Verificá que esté en PATH
```

**"Review falla siempre"**
- Simplificá tu `REVIEW.md`
- Probá con `PROVIDER="claude"`

---

## Links

- Issues: https://github.com/Yoizen/dev-ai-workflow/issues
