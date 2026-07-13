# Plan: Contratos tipados + review + advisor (opencode)

> Fecha: 2026-07-13 (rev 3)  
> Host: **opencode** primary  
> **Missions: IGNORAR** — fuera de scope (cero hooks, parsers o gates en `internal/missions/`)

## Decisiones cerradas

| Decisión | Estado |
|----------|--------|
| Primary host = **opencode** | ✅ |
| Roles / models / fallbacks (profiles, permissions, `role_defaults`) | ✅ Ya existe — fuera |
| **Missions** (workers, ValidationPipeline, FSM) | ❌ **Ignorar** |
| **Contratos tipados** en agents/skills | ✅ P0 |
| **Review veredicto** P0–P3 + ship/block | ✅ P0 |
| **Advisor mid-run** vía agent/workflow (no missions) | ✅ P1 |
| Harness omp (hashline, LSP, etc.) | ❌ No reimplementar |

## Boundary

| Sí | No |
|----|-----|
| `ywai/agents/**` (sections, AGENT.md, profiles) | `ywai/internal/missions/**` |
| `ywai/skills/**` (reviewer-worker, workers) | ValidationPipeline / feature gates |
| `ywai/workflows/**` (nodo advisor opcional) | Worker hooks, every_n_turns en Go |
| Contratos como **prompt contract** (+ parser util opcional solo si sirve a kanban/export) | Ship gate en missions engine |

---

## Prioridad

| Orden | Entrega | P | Dónde |
|-------|---------|---|--------|
| 1 | Handoff tipado (` ```handoff `) | P0 | sections + orchestrator + worker skills |
| 2 | Review tipado (` ```review `) + reglas de ship en **prompt** | P0 | reviewer agent + skill |
| 3 | Advisor agent + uso en workflows / delegación | P1 | `agents/core/advisor/` + workflow nodes |
| — | Missions integration | Cancelled | — |

---

## Objetivo

1. Todo specialist termina con un **bloque parseable** (handoff).
2. Reviewer termina con **veredicto + issues P0–P3**; el **orchestrator** (humano o agent) no avanza si hay `block` / P0 — por **instrucción**, no por código missions.
3. Advisor es un **agent de solo lectura** que el orchestrator (o un nodo de workflow) puede invocar mid-run.

---

## PR plan

### PR1 — Typed handoff (prompt contract)

**Contrato**

```yaml
# ```handoff
status: done | blocked | needs-decision
did: string
artifacts:
  - path: string
    kind: file | command | test
next: dev | qa | reviewer | devops | close | null
risks: string[]
findings:
  - path: string
    severity: P0 | P1 | P2 | P3
    confidence: number
    message: string
kanban:
  column: review | backlog | done
  summary: string
  detail: string
```

**Cambios**

- `agents/sections/handoff.md`, `handoff-qa.md` — fence obligatorio + ejemplo
- `agents/core/orchestrator/AGENT.md` — leer el bloque; si falta fence, pedir re-handoff o tratar como incompleto
- Skills `*-worker` — “end with ` ```handoff `”
- **Sin** package en missions; parser Go **opcional** y solo si hace falta para UI/kanban después (no bloqueante de PR1)

**Done when:** sections + orchestrator + workers documentan el mismo contrato.

---

### PR2 — Review verdict (prompt gate)

```yaml
# ```review
verdict: ship | ship-with-nits | block
summary: string
issues:
  - path: string
    severity: P0 | P1 | P2 | P3
    confidence: number
    message: string
    fix_hint: string
```

**Reglas (en orchestrator + reviewer prompts)**

| Condición | Qué hace el orchestrator |
|-----------|---------------------------|
| `verdict: block` o cualquier P0 | No mandar a devops/close; reabrir `@dev` o preguntar al user |
| `ship-with-nits` (solo P2/P3) | Puede seguir; listar nits |
| `ship` sin P0/P1 | Seguir |

**Cambios**

- `agents/core/reviewer/AGENT.md`
- `skills/reviewer-worker/SKILL.md`
- Orchestrator: “after @reviewer, parse ` ```review `; never close on block/P0”

**Done when:** el camino ask→…→reviewer→close está escrito con esas reglas (sin missions).

---

### PR3 — Advisor agent (sin missions)

**Qué**

- Profile `advisor`: read-only, no edita código, no implementa.
- El **orchestrator** (o un nodo workflow) lo invoca cuando quiera un second opinion mid-task.

```yaml
# ```advisor
level: aside | concern | block
message: string
acceptance_gap: string
```

| level | Qué hace el orchestrator |
|-------|---------------------------|
| `aside` | Registrar; seguir |
| `concern` | Reinyectar al doer en el próximo delegate |
| `block` | Parar y preguntar al user |

**Cambios**

- `agents/core/advisor/` (AGENT.md, permissions read-only, skills.txt si aplica)
- Mencionar en orchestrator: cuándo invocar `@advisor` (deriva de acceptance criteria, refactor grande, pre-close)
- Opcional: nodo en un workflow de ejemplo — **no** wiring en missions worker

**Done when:** profile instalable + orchestrator sabe cuándo/cómo usarlo.

---

## Explicitamente fuera

- **Todo `internal/missions/**`**
- Role routing nuevo
- omp primary / hashline / LSP
- ValidationPipeline ship gate en Go
- every_n_turns automático en runtime

---

## Sprints

```
PR1  Handoff contract (sections + agents + skills)
PR2  Review contract + orchestrator ship rules
PR3  Advisor agent + orchestrator usage
```

---

## Standing rule (from user)

**Any change to orchestrator contract/behavior must ship to ALL orchestrators and ALL workflows.**

How we enforce it:

| Surface | Mechanism |
|---------|-----------|
| Agent profiles (`role: orchestrator` / `planning`) | Auto-append `sections/orchestrator-contracts.md` in `loadProfile` |
| Workflow export | `export.go` always AppendSections `orchestrator-contracts` on orchestrator body |
| Worker handoffs | Shared `sections/handoff.md` / `handoff-qa.md` (subagents + workflow subAgent nodes) |
| Do **not** | Copy-paste a one-off contract into a single workflow JSON or a single AGENT.md |

## Next

Continue: advisor agent (P1) if desired; contracts + review gate landed in sections + export.
