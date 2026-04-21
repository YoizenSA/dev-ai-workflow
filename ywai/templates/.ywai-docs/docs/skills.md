# Skills y auto-invoke

Las skills son archivos de instrucciones modulares (`SKILL.md`) que los
agentes cargan cuando matchea una trigger phrase o un scope. Viven bajo
`skills/` en tu proyecto y/o bajo el directorio global de cada agente.

## Anatomía de una skill

```yaml
---
name: skill-name
description: >
  Descripción corta + triggers.
  Trigger: keyword1, keyword2, ...
license: Apache-2.0
metadata:
  author: Yoizen
  version: "1.0"
  scope: [root, backend, angular]   # qué AGENTS.md reciben la fila de auto-invoke
  auto_invoke:
    - "keyword 1"
    - "keyword 2"
allowed-tools: [Read, Edit, Write, Glob, Grep, Bash]
---

## When to Use

...

## Critical Patterns / Rules

- Regla accionable 1
- Regla accionable 2
```

## Cómo funciona auto-invoke

Hay dos capas:

1. **Tablas de auto-invoke en `AGENTS.md`** — generadas por `skill-sync`.
   Cualquier skill con `metadata.scope` y `metadata.auto_invoke` aparece en
   el `AGENTS.md` correspondiente. Los agentes leen `AGENTS.md` al arrancar
   y saben qué skills cargar para qué triggers.

2. **Registry compacto** (`.ywai/skill-registry.md`) — generado por
   `skill-sync --registry` o la skill `skill-registry`. En vez de cargar
   cada `SKILL.md`, los delegadores leen este archivo único e inyectan las
   compact rules que matcheen (5–15 líneas cada una) en los prompts de los
   sub-agentes. Ahorra ~40% de tokens del prompt en proyectos con muchas
   skills.

## Regenerar

```bash
# Reconstruir las tablas de auto-invoke en los AGENTS.md
bash skills/skill-sync/assets/sync.sh

# Reconstruir .ywai/skill-registry.md (compact rules)
bash skills/skill-sync/assets/sync.sh --registry

# Dry-run primero
bash skills/skill-sync/assets/sync.sh --dry-run
```

Corré estos comandos después de agregar, sacar o editar la metadata de una skill.

## Valores de scope

| Scope | `AGENTS.md` actualizado |
|:---|:---|
| `root` | `/AGENTS.md` |
| `copilot` | `.github/copilot-instructions.md` |
| `<custom>` | Subdirectorio con el nombre del scope (ej. `backend/AGENTS.md`) |

Una skill puede declarar múltiples scopes: `scope: [root, backend, api]`.

## Crear una nueva skill

1. Corré la skill `skill-creator` (`/skill-creator` o simplemente describí lo que querés).
2. Crea `skills/<nueva-skill>/SKILL.md` a partir del template.
3. Completá `## Critical Patterns` / `## Rules` con bullets accionables.
4. Corré `bash skills/skill-sync/assets/sync.sh` + `--registry` para propagar.

## Listar skills instaladas

Revisá `.ywai/skill-registry.md` para ver el catálogo actual + compact rules.
Si el archivo no existe, corré:

```bash
bash skills/skill-sync/assets/sync.sh --registry
```

## Tips

- Mantené `description` corta; los detalles van en `## When to Use`.
- Poné las reglas **accionables** en `## Critical Patterns` — esas pasan al registry compacto.
- Saltéate `sdd-*`, `_shared`, `skill-registry` y `skill-sync` al escanear — son skills de workflow, no de coding.
