# Protocolo de persistencia Engram

[Engram](https://github.com/Yoizen/engram) es un servidor de memoria local
que permite que los artifacts de SDD sobrevivan entre sesiones. Cuando tu
proyecto está configurado con `artifact store mode: engram`, cada sub-agente
SDD persiste su output como una **observation** con un `topic_key` estable.

## La regla crítica

**`mem_search` devuelve PREVIEWS de 300 caracteres, no el contenido completo.**

Si necesitás el artifact entero, TENÉS que llamar a `mem_get_observation(id)`.
Cada slash command SDD lo documenta en su body:

```
STEP A — SEARCH (get IDs only)
STEP B — RETRIEVE FULL CONTENT (mandatory)
```

Saltearse STEP B significa que el sub-agente trabaja con una preview truncada
— pérdida silenciosa de data.

## Mapa de artifacts

Cada artifact SDD usa un `topic_key` predecible:

| Fase | topic_key | Lo guarda |
|:---|:---|:---|
| Init del proyecto | `sdd/{project}/project-context` | `sdd-init` |
| Exploración | `sdd/{nombre-del-cambio}/exploration` | `sdd-explore` (cuando está atado a un cambio) |
| Proposal | `sdd/{nombre-del-cambio}/proposal` | `sdd-propose` |
| Spec | `sdd/{nombre-del-cambio}/spec` | `sdd-spec` |
| Design | `sdd/{nombre-del-cambio}/design` | `sdd-design` |
| Tasks | `sdd/{nombre-del-cambio}/tasks` | `sdd-tasks` |
| Apply progress | `sdd/{nombre-del-cambio}/apply-progress` | `sdd-apply` |
| Verify report | `sdd/{nombre-del-cambio}/verify-report` | `sdd-verify` |
| Archive report | `sdd/{nombre-del-cambio}/archive-report` | `sdd-archive` |
| Testing capabilities | `sdd/{project}/testing-capabilities` | cacheado por `sdd-init` |
| Skill registry | `skill-registry` | `skill-registry` / `skill-sync --registry` |
| Estado de onboarding | `sdd-onboard/{project}` | `sdd-onboard` |

## `topic_key` = clave de upsert

Guardar con el mismo `topic_key` **actualiza** la observation en vez de crear
un duplicado. Re-correr `/sdd-ff feature-x` después de editar la propuesta no
acumula copias — reemplaza.

## Patrón estándar de retrieval

```
STEP A — SEARCH (get IDs only):
  mem_search(query: "sdd/add-dark-mode/spec", project: "my-project")
  → devuelve [{id: 42, title: "...", preview: "...300 chars..."}]
  → guardá spec_id = 42

STEP B — RETRIEVE FULL CONTENT (mandatory):
  mem_get_observation(id: 42)
  → devuelve el contenido completo del artifact
```

## Patrón estándar de save

```
mem_save(
  title: "sdd/add-dark-mode/spec",
  topic_key: "sdd/add-dark-mode/spec",
  type: "architecture",
  project: "my-project",
  content: "{markdown completo del artifact}"
)
```

## Merge seguro de progreso

`sdd-apply` es la única fase que puede correr varias veces sobre el mismo
cambio (batch tras batch de tareas). Tiene que MERGEAR — no pisar — el
progreso:

```
1. mem_search(query: "sdd/.../apply-progress") → progress_id?
2. IF progress_id: mem_get_observation(progress_id) → estado previo
3. Implementar tareas nuevas
4. mem_save(topic_key: "sdd/.../apply-progress", content:
   "{tareas previas completadas + tareas nuevas completadas + evidence acumulada}")
```

Si guardás sin leer el progreso anterior primero, el trabajo completado de
batches previos **se pierde permanentemente** (la observation vieja se pisa
por upsert).

## Cuando Engram no está disponible

Si el proyecto corre en modo `sdd` (filesystem), los artifacts viven bajo
`sdd/changes/{nombre-del-cambio}/*.md` y el protocolo STEP A/B no aplica.
Los sub-agentes detectan el modo y switchean de convención automáticamente
vía `skills/_shared/persistence-contract.md`.
