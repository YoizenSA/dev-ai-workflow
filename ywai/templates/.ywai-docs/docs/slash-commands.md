# Referencia de slash commands

Cada `/sdd-*` se instala para todos los agentes soportados (Claude, OpenCode,
GitHub Copilot, Gemini). Comparten una estructura común:

```
---
description: <qué hace>
agent: sdd-orchestrator
subtask: true          # commands que invocan un solo sub-agente
---

If the native `<skill>` sub-agent is available, delegate this command to it.
Otherwise, locate and read the skill file from the FIRST existing path below.

CONTEXT: ...
TASK: ...
ENGRAM PERSISTENCE: ...
```

Esto significa que cada command **degrada gracefully**: si el agente no tiene
un sub-agente nativo, cae al skill file y lo sigue inline.

## Catálogo

| Command | Para qué | Sub-agente |
|:---|:---|:---|
| `/sdd-init` | Detecta stack, inicializa el backend de persistencia | `sdd-init` |
| `/sdd-onboard` | Walkthrough guiado del ciclo completo sobre tu repo | `sdd-onboard` |
| `/sdd-explore {tema}` | Compara opciones, recomienda un enfoque | `sdd-explore` |
| `/sdd-new {nombre}` | Exploración → propuesta | orquesta `sdd-explore` + `sdd-propose` |
| `/sdd-ff {nombre}` | Fast-forward: proposal → spec → design → tasks | orquesta 4 sub-agentes |
| `/sdd-continue` | Detecta la fase siguiente y la corre | orquestador |
| `/sdd-apply` | Implementa las tareas pendientes (con soporte TDD) | `sdd-apply` |
| `/sdd-verify` | Quality gate: tests + compliance de specs + Strict TDD gate | `sdd-verify` |
| `/sdd-archive` | Syncea delta specs, archiva el cambio | `sdd-archive` |

## Dónde viven en disco

Se instalan en los agentes que habilitaste durante el setup:

| Agente | Ubicación |
|:---|:---|
| GitHub Copilot (prompts del proyecto) | `.github/prompts/sdd-*.md` |
| OpenCode (commands globales) | `~/.config/opencode/command/sdd-*.md` |
| OpenCode (skills globales) | `~/.config/opencode/skills/sdd-*/SKILL.md` |
| GitHub Copilot (agentes globales) | `~/.copilot/agents/sdd-*.md` |
| Claude (commands globales) | `~/.claude/commands/sdd-*.md` *(si activaste la extensión Claude)* |

## Argumentos

Los commands que toman argumento usan `{argument}` como placeholder (la
mayoría de los agentes lo substituyen con lo que escribas después del slash
command, ej. `/sdd-new add-dark-mode` → `{argument}` = `add-dark-mode`).

Placeholders contextuales que el agente completa en runtime:

- `{workdir}` — directorio de trabajo actual.
- `{project}` — basename del directorio de trabajo (se usa como project key de engram).
- `{argument}` — lo que escribiste después del slash command.

## Agregar un nuevo slash command

1. Creá `slash-commands/sdd-foo.md` con el frontmatter común + body.
2. Volvé a correr el instalador (el install-step de `slash-commands` toma todo `*.md`).
3. Opcionalmente agregá una skill con el mismo nombre en `skills/sdd-foo/SKILL.md` para que los agentes con sub-agente nativo puedan delegar.
