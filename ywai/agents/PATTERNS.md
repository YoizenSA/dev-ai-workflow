# ywai Patterns Roadmap

> Plan de integración de patrones dinámicos en el workflow de agents de ywai.

## Contexto

Claude Code dynamic workflows introducen patrones como adversarial verification, tournament, fan-out-and-synthesize, y loop-until-done. Este documento propone integrar los patrones más útiles dentro del framework ywai, manteniendo la compatibilidad cross-platform y el pipeline opinionado.

---

## Prioridades

| Prioridad | Patrón | Impacto | Complejidad |
|-----------|--------|---------|---------------|
| **Alta** | Adversarial Verification | Previene self-preferential bias en reviews | Media |
| **Alta** | Root-Cause Investigation | Mejora debugging estructurado | Media |
| **Media** | Deep Research Workflow | Amplia capacidad de research del agent ask | Media |
| **Media** | Tournament Pattern | Mejora decisiones de diseño/naming | Alta |
| **Media** | Loop Until Done | Potencia debugging iterativo | Media |
| **Media** | Classify-and-Act Routing | Optimiza delegación dinámica | Alta |
| **Baja** | Worktree Isolation | Permite massive parallelism sin conflictos | Alta |
| **Baja** | Generate-and-Filter | Facilita brainstorming estructurado | Media |
| **Baja** | Token Budgets | Controla costos en workflows pesados | Baja |
| **Baja** | Documentación de Híbridos | Guía de uso de patrones dinámicos | Baja |

---

## Fase 1: Patrones Adversariales

### 1.1 Adversarial Verification en Reviewer Phase

**Problema actual**: El reviewer a veces presenta self-preferential bias; aprueba su propio trabajo o no cuestiona suficientemente.

**Solución propuesta**:

```
orchestrator --> reviewer (approval review)
orchestrator --> skeptical-reviewer (adversarial review)
    si conflicto --> orchestrator decide o pide aclaración
```

**Implementación**:
- Crear `agents/reviewer/skeptical/AGENT.md` con persona de "skeptic"
- Modificar `WORKFLOW.md` para incluir review paralelo adversarial
- El orchestrator compara handoffs y decide

**Archivos a modificar**:
- `agents/reviewer/` (nuevo subdirectorio `skeptical/`)
- `agents/WORKFLOW.md` (sección Review Cycle)
- `agents/README.md` (documentar nuevo pattern)

**Trade-off**: +50-100% tokens en review phase. Activar vía flag `--adversarial-review`.

---

### 1.2 Root-Cause Investigation

**Problema actual**: Debugging sufre de self-preferential bias cuando se hace en un solo context window.

**Solución propuesta**:

```
orchestrator --> hipótesis-generator-logs
orchestrator --> hipótesis-generator-files
orchestrator --> hipótesis-generator-data
    (cada uno genera hipótesis desde evidence disjunta)
    --> panel-de-verifiers (uno por hipótesis)
    --> synthesis-agent --> handoff
```

**Implementación**:
- Crear skill `skills/root-cause-investigation/` con workflow JavaScript/Go
- Integrar en `agents/dev/` como modo especial `--investigate`
- Pattern: generar hipótesis independientes + adversarial verify + síntesis

**Archivos a crear**:
- `skills/root-cause-investigation/SKILL.md`
- `skills/root-cause-investigation/workflow.js` (o equivalente Go)

**Trade-off**: 3-5x tokens vs debugging tradicional. Usar solo para bugs complejos.

---

## Fase 2: Patrones de Decisión y Exploración

### 2.1 Deep Research Workflow para Ask

**Problema actual**: El agent `ask` hace research en un solo context window; no puede profundizar en múltiples fuentes.

**Solución propuesta**:

```
ask (research mode) --> fan-out: N web searches
    --> fetch sources
    --> adversarial verification de claims
    --> synthesis --> cited report
```

**Implementación**:
- Crear skill `skills/deep-research/`
- Agregar flag `--deep` al agent `ask`
- Output: report con citas y verificación de factual claims

**Archivos a crear**:
- `skills/deep-research/SKILL.md`
- `skills/deep-research/workflow.md`

**Trade-off**: 5-10x tokens vs research simple. Usar para investigación profunda.

---

### 2.2 Tournament Pattern

**Problema actual**: Decisiones de diseño (naming, arquitectura) se hacen con un solo approach.

**Solución propuesta**:

```
orchestrator --> spawn N agents con diferentes approaches
    --> judging-agent: pairwise comparison
    --> tournament hasta winner
    --> winner se convierte en la decisión
```

**Implementación**:
- Agregar flag `--tournament` al orchestrator
- Spawn N agents (diferentes personas/prompts)
- Judging agent con rubric pairwise
- Integrar en fase PLAN del architect

**Archivos a modificar**:
- `agents/orchestrator/AGENT.md` (agregar tournament delegation)
- `agents/architect/AGENT.md` (soportar modo tournament)

**Trade-off**: Nx tokens (donde N = número de competidores). Usar para decisiónes críticas.

---

### 2.3 Classify-and-Act Routing

**Problema actual**: El orchestrator decide manualmente el siguiente step; no hay routing dinámico basado en el tipo de task.

**Solución propuesta**:

```
user-goal --> classifier-agent
    classifier retorna: task-type + recommended-pipeline + model-selection
    orchestrator delega según clasificación
```

**Implementación**:
- Crear `agents/classifier/AGENT.md`
- El orchestrator delega primero a classifier antes de cualquier otro agent
- Classifier puede sugerir: model (sonnet vs opus), pipeline (tdd vs direct), parallelism level

**Archivos a crear**:
- `agents/classifier/AGENT.md`
- `agents/classifier/tools.json`

**Trade-off**: +1 step inicial. Mejora la calidad de delegación.

---

## Fase 3: Patrones de Escalado e Iteración

### 3.1 Loop Until Done

**Problema actual**: Fan-out usa número fijo de agents; no hay iteración hasta condición de parada.

**Solución propuesta**:

```
while (no stop-condition):
    orchestrator --> spawn agents para investigar
    if (no new findings / no more errors):
        break
    else:
        continue loop
```

**Implementación**:
- Agregar pattern en `agents/orchestrator/AGENT.md`
- Stop conditions configurables: `no-new-findings`, `error-count-zero`, `max-iterations`
- Integrar con `/loop` (si el host lo soporta)

**Archivos a modificar**:
- `agents/orchestrator/AGENT.md` (sección "Iterative Patterns")
- `agents/WORKFLOW.md` (documentar loop pattern)

**Trade-off**: Tokens variables (hasta condición de parada). Usar para triage y debugging iterativo.

---

### 3.2 Worktree Isolation

**Problema actual**: Fan-out paralelo escribe en el mismo workspace; riesgo de conflictos.

**Solución propuesta**:

```
orchestrator --> delegate --worktree agent-1
orchestrator --> delegate --worktree agent-2
    (cada agent en su propio git worktree)
    --> merge results cuando todos terminan
```

**Implementación**:
- Investigar soporte de worktree isolation en opencode/otros hosts
- Agregar flag `--worktree` a `delegate` (si el host lo soporta)
- Prevención de cross-contamination en massive parallelism

**Archivos a investigar**:
- Capacidades de `background-agents` plugin (opencode)
- Alternativa: ramas temporales + merge

**Trade-off**: + overhead de worktrees. Usar para tasks con riesgo de conflicto.

---

## Fase 4: Utilidades y Documentación

### 4.1 Generate-and-Filter

**Problema actual**: No hay workflow creativo explícito para brainstorming.

**Solución propuesta**:

```
orchestrator --> generate-agent: N ideas
    --> filter-agent: apply rubric
    --> dedupe-agent: remove duplicates
    --> return top N ideas
```

**Implementación**:
- Crear skill `skills/brainstorm/`
- Integrar en `agents/architect/` para ideación
- Rubric configurable por el usuario

**Archivos a crear**:
- `skills/brainstorm/SKILL.md`

**Trade-off**: 2-3x tokens vs brainstorming simple. Usar cuando la calidad de ideas es crítica.

---

### 4.2 Token Budgets

**Problema actual**: Sin control de token usage; workflows dinámicos pueden ser costosos.

**Solución propuesta**:

```
ywai install --token-budget 10000
# o
orchestrator: "use max 10k tokens for this task"
```

**Implementación**:
- Agregar config en `ywai/.config/token-budgets.yaml`
- Orchestrator rastrea usage y alerta al acercarse al límite
- Graceful degradation: si se acerca al budget, reduce parallelism

**Archivos a crear**:
- `ywai/.config/token-budgets.yaml` (template)

**Trade-off**: Mínimo. Mejora control de costos.

---

### 4.3 Documentación de Patrones Híbridos

**Implementación**:
- Este archivo (`PATTERNS.md`) como guía central
- Agregar sección en `agents/README.md` con "When to use dynamic patterns"
- Ejemplos de prompts para trigger cada pattern
- Tabla comparativa: static vs dynamic para cada caso de uso

---

## Roadmap Sugerido

### Sprint 1 (Semanas 1-2)
- [ ] Adversarial verification en reviewer phase (#1.1)
- [ ] Root-cause investigation skill (#1.2)

### Sprint 2 (Semanas 3-5)
- [ ] Deep research workflow para ask (#2.1)
- [ ] Tournament pattern (#2.2)

### Sprint 3 (Semanas 6-8)
- [ ] Loop until done (#3.1)
- [ ] Classify-and-act routing (#2.3)

### Sprint 4 (Semanas 9-11)
- [ ] Worktree isolation (#3.2)
- [ ] Token budgets (#4.2)
- [ ] Generate-and-filter (#4.1)
- [ ] Documentación final (#4.3)

---

## Principios de Diseño

1. **Opt-in**: Todos los patrones dinámicos son opt-in vía flags (`--adversarial-review`, `--tournament`, `--deep`)
2. **Backward compatible**: El pipeline estático default no cambia
3. **Cross-platform**: Los que dependen de features específicas de host (worktree) son graceful degradation
4. **Token-aware**: Cada pattern documenta su multiplicador de tokens
5. **Composable**: Los patterns se pueden combinar (tournament + adversarial review)

---

## Decisiones Pendientes

| Decisión | Opciones | Impacto |
|----------|----------|---------|
| Workflow language | JavaScript (Claude-style) vs Go native vs Markdown-based | Portabilidad vs complejidad |
| Host support | ¿Qué hosts soportan `delegate` async? | Alcance de fan-out patterns |
| Token tracking | ¿Cómo rastrear usage cross-agent? | Implementación de budgets |
| Model routing | ¿Soportar model selection (sonnet vs opus)? | Complejidad del classifier |

---

*Documento vivo. Actualizar a medida que se implementen los patrones.*
