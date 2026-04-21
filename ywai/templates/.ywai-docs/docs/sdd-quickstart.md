# Quickstart de SDD — Spec Driven Development

SDD es un **workflow por fases** que te obliga a pensar antes de codear. Cada
fase produce un artifact que alimenta a la siguiente. Los sub-agentes son
dueños de cada fase; el orquestador los coordina.

## El pipeline

```
 explore → propose → spec → design → tasks → apply → verify → archive
    │         │       │       │        │        │        │         │
    ▼         ▼       ▼       ▼        ▼        ▼        ▼         ▼
 options  proposal  specs  design   tasks    code    report   archived
```

## Cuándo usar cada command

| Situación | Command |
|:---|:---|
| No tenés claro qué approach tomar | `/sdd-explore {tema}` |
| Listo para arrancar un cambio | `/sdd-new {nombre-del-cambio}` |
| Saltear exploración, ir al plan completo | `/sdd-ff {nombre-del-cambio}` |
| Perdiste el hilo de en qué fase estás | `/sdd-continue` |
| El plan está listo, a codear | `/sdd-apply` |
| Terminaste de codear, querés chequear | `/sdd-verify` |
| Verify está verde, cerrá el cambio | `/sdd-archive` |
| Primera vez usando SDD en este repo | `/sdd-onboard` |

## Flujo típico

```bash
# 1. Arrancar — crea exploración + propuesta
/sdd-new add-dark-mode

# 2. Planificar — fast-forward por spec + design + tasks
/sdd-ff add-dark-mode

# 3. Implementar — corre las tareas en orden (con soporte TDD)
/sdd-apply

# 4. Quality gate — corre tests, revisa specs, revisa design
/sdd-verify

# 5. Cerrar — mergea delta specs a los specs principales, archiva el cambio
/sdd-archive
```

## Artifacts

Cada fase produce un artifact. Según el modo de persistencia resuelto para tu
proyecto (`engram | sdd | hybrid | none`), el artifact vive en:

- **engram**: observations con `topic_key` tipo `sdd/{nombre-del-cambio}/spec`.
  Leé [engram.md](engram.md) para el protocolo STEP A/B.
- **sdd**: filesystem bajo `sdd/changes/{nombre-del-cambio}/` (proposal.md,
  spec.md, design.md, tasks.md).
- **hybrid**: los dos — el filesystem es la source of truth, engram es un
  espejo.
- **none**: se devuelve inline en la respuesta del sub-agente únicamente.

## Modos de TDD

`sdd-apply` resuelve TDD automáticamente:

| Modo | Cuándo aplica | Dónde leer |
|:---|:---|:---|
| **Standard** | `rules.apply.tdd: false` o no seteado | Step 4 de `sdd-apply` |
| **Light TDD** | `rules.apply.tdd: true` | Step 4 (RED → GREEN → REFACTOR) |
| **Strict TDD** | `rules.apply.strict_tdd: true` + test runner presente | [`tdd.md`](tdd.md) + módulo `strict-tdd.md` |

Mirá [`tdd.md`](tdd.md) para las reglas completas cuando Strict TDD está activo.

## Cómo recuperarte de errores

- **Corriste `/sdd-ff` pero la propuesta está mal** → editá `proposal.md` a mano y volvé a correr `/sdd-ff` (upsertea vía `topic_key`).
- **`sdd-apply` rompió un test** → revertí el archivo, re-correlo `/sdd-apply` — la skill relee el progreso y saltea las tareas completadas.
- **Los specs se desviaron durante la implementación** → `sdd-archive` es el lugar para mergear los delta specs a los specs canónicos.
