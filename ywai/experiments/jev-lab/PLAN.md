# Plan: capa Jev para OpenCode 2 (v2 refinada)

Fecha: 2026-09-17
Objetivo: meter Jev como capa de *política* (ruteo + poda de tools + review + búsqueda semántica) encima de OpenCode 2 (`opencode2`), sin convertir a Jev en el modelo de chat.


---

## Cambios respecto a la versión anterior

1. **Pick deja de ser una tool que llama el LLM.** Como tool no ahorra nada: OC2 igual manda todos los schemas de tools en cada request, y encima suma un paso. Ahora vive en `ctx.session.hook("context")`, que puede borrar entradas de `event.tools` antes de cada llamada al modelo. `jev_pick_tool` queda solo como tool de debug.
2. **Los gates se aplican con `ctx.permission.hook("evaluate")`**, no solo con texto en el skill. Regla nueva: Jev solo puede *endurecer* permisos (allow → ask), nunca relajarlos.
3. **Git sale de un adapter propio a `ctx.vcs.diff()`** (con `git.ts` como fallback para tests/CLI). Persistencia y caché van a `ctx.storage`.
4. **Contradicciones resueltas:** comportamiento sin key (§3.1), qué pasa con `closeCall` en Pick (§3.6), `ROUTE_SEVERITY` renombrado a `OWNER_SEVERITY`, constantes que solo estaban en §12 ahora en la tabla, ejemplo de Find en Rust vs glob sin `.rs`, commands markdown “por si el plugin no carga” (no tenía sentido: sin plugin no hay tools).
5. **Fases reordenadas** para que coincidan con el orden de valor (review → find → pick → route). Fase 0 ahora también valida la plugin API, que es el mayor riesgo.
6. **Nombres de tools unificados** con la regla de OC2: namespace `jev` + nombre `review_diff` → `jev_review_diff`. Los ids del catálogo de route ya no llevan prefijo `jev_` para no confundirse con tools.
7. **Riesgos nuevos:** prompt injection desde el código revisado, envío de código a un tercero, poda de tools que rompe el prompt caching, latencia por paso.
8. **Criterios de done medibles** (tiempos, casos concretos) y matemática de costos corregida (requests vs preguntas).
9. Cosas marcadas con ⚠️ = no verificadas en docs; se confirman en Fase 0 (lista en §6.7).

---

## 0. Principio rector

Jev no habla. Jev decide.

- OpenCode (`plan` / `build`) lee, edita, corre tests y explica.
- Jev solo responde Choice / Score / Noul con probabilidad y confidence.
- El código aplica umbrales. Si Jev duda, se escala al humano o al modelo caro.
- Jev solo puede **endurecer** (pedir confirmación, podar tools con alta confianza). Nunca aprueba, nunca relaja un permiso.
- Si Jev falla o tarda: **fail open** en lo que es optimización (Pick), **fail closed con aviso** en lo que dice ser de Jev (Review, Find).

Nunca:
- registrar `jev-latest` como provider/modelo de chat del TUI
- pedirle a Jev que escriba un parche o argumentos de una tool
- dejar que el LLM “reescriba” la policy (umbrales viven en config)
- presentar como “finding de Jev” algo que Jev no marcó

---

## 1. Qué se construye

Un plugin local de OpenCode 2, `jev-gate`, con cuatro capacidades:

| # | Capacidad | Mecanismo OC2 | Frecuencia |
|---|---|---|---|
| 1 | **Review** — pipeline staged tipo jev-review sobre el diff o un path | tool + command | a pedido |
| 2 | **Find** — grep semántico tipo tsg | tool + command | a pedido |
| 3 | **Pick** — podar las tools que ve el modelo en cada paso (patrón Vini) | hook `context` | cada llamada al modelo |
| 4 | **Route** — ¿quién ejecuta la tarea? | tool + command (v1), hook `prompt` (v1.1) | una vez por prompt |

Más: gates vía hook `permission`, un skill, y commands registrados desde el plugin.

Fuera de alcance v1:
- dashboard HTTP de jev-review
- ruteo a Codex / Claude Code / Cursor (delegate; posible v2)
- indexación persistente tipo embeddings
- compiler/LSP como fuente de verdad (sí como contexto extra en v1.5)
- verificación de argumentos de tool calls con Jev (v1.1; el patrón está en las docs de TypeSafe)

---

## 2. Arquitectura

```
.opencode/
  plugins/jev-gate/            # auto-descubierto por OC2
    package.json
    index.ts                   # Plugin.define: registra tools, commands, hooks
    src/
      domain/
        config.ts              # umbrales, dimensiones, rubrics, deny globs
        types.ts
        patch.ts               # parse de hunks
      jev/
        client.ts              # POST /v1/systemone, timeout, usage
        questions.ts           # builders noul/choice/score (versionados)
      adapters/
        vcs.ts                 # ctx.vcs.diff / status
        git.ts                 # fallback para tests y scripts
        files.ts               # lectura + deny globs
        store.ts               # ctx.storage: último report, decisiones por sesión, caché
      review/
        workflow.ts            # screen → profile → locate → action
        judgments.ts
      find/
        segment.ts
        score.ts
      route/
        catalog.ts
        decide.ts
      pick/
        prune.ts               # lógica pura: roster + decisión → set a conservar
        decide.ts
      gates/
        permission.ts          # reglas de endurecimiento
      format.ts                # JSON → markdown compacto
  skills/
    jev-gate/SKILL.md
opencode.jsonc                 # solo si hace falta pasar options
scripts/
  spike-screen.mjs
  eval-fixtures.mjs
  eval-pick.mjs
```

Flujo de dependencias:

```
index.ts (OC2) → review | find | route | pick | gates → adapters → domain
                                                    ↘ jev/
```

- `domain` no importa SDK ni `@opencode/plugin`.
- `jev/` no importa git ni OC2.
- Solo `index.ts` y `adapters/vcs.ts`, `adapters/store.ts` tocan `@opencode/plugin`. Si la API beta cambia, se rompe en un solo lugar.

---

## 3. Decisiones de diseño (cerrar antes de codear)

### 3.1 API key y comportamiento sin key

- Env: `TYPESAFE_API_KEY`
- Fallback: `options.apiKey` en un `opencode.jsonc` **no commiteado** (o `~/.config/opencode/opencode.jsonc`)

| Capacidad | Sin key |
|---|---|
| Review | Se niega con mensaje claro. No hay “review de Jev” sin Jev. |
| Find | Se niega. |
| Pick | Hook desactivado (no poda nada). |
| Route (tool) | Fallback a `ctx.generate.text` con el mismo catálogo; resultado marcado `source: "llm"`. |
| Gates | Ignoran decisiones con `source: "llm"`. Permisos normales de OC. |

### 3.2 Unidad de trabajo

- **Review changes:** archivos fuente del diff de trabajo (tests como contexto, no como target).
- **Review path:** un archivo o dir, con cap (§3.3).
- **Find:** ventanas de 60 líneas con overlap 10, recortadas a `MAX_SEGMENT_CHARS`.
- **Route:** texto de la tarea + metadata (archivos mencionados, si pide escribir, tests fallando).
- **Pick:** último mensaje del usuario + última tool usada + roster actual (id + descripción recortada). Nunca el transcript entero.
- **Deny globs** (nunca se mandan a Jev): `**/.env*`, `**/*.pem`, `**/*.key`, `**/secrets/**`, más lo que diga config.

### 3.3 Umbrales v1

Valores de jev-review/tsg donde existen; los marcados *prov.* son provisionales hasta Fase 6.

| Const | Valor | Efecto |
|---|---|---|
| SCREEN_THRESHOLD | 0.70 | dimensión pasa a follow-up |
| MIN_LOCATION_CONFIDENCE | 0.55 | hunk válido |
| OWNER_SEVERITY | 1.50 | asignar owner (antes `ROUTE_SEVERITY`) |
| BLOCKING_SEVERITY | 2.00 | action = `request_changes` |
| FIND_THRESHOLD | 0.81 | default tsg |
| ROUTE_MARGIN | 0.12 | top1 − top2 menor → `closeCall` |
| ROUTE_MIN_CONFIDENCE | 0.60 *prov.* | confidence menor → `closeCall` |
| PICK_MIN | 0.45 *prov.* | confidence menor → no podar |
| PICK_MARGIN | 0.10 | top1 − top2 menor → conservar ambas |
| PICK_TIMEOUT_MS | 800 *prov.* | pasado eso, no podar |
| JEV_TIMEOUT_MS | 5000 | timeout por request |
| MAX_FOLLOW_UPS | 8 | señales que pasan a locate |
| MAX_PROFILES | 5 | archivos perfilados |
| CONCURRENCY | 3 | **requests** en paralelo (no preguntas) |
| MAX_SEGMENT_CHARS | 4000 | recorte por segmento |
| FIND_BATCH | 5 *prov.* | segmentos por request (medir contra 1) |
| MAX_FIND_SEGMENTS | 200 | hard stop de Find |
| CONFIRM_FILES_CODEBASE | 30 | arriba de esto, pedir confirmación |
| MAX_FILES_CODEBASE | 80 | hard stop |

Nota: Jev devuelve `confidence` además de las probabilidades. Los gates usan las dos cosas (margen y confidence), no solo el margen.

### 3.4 Dimensiones de review

Idénticas a jev-review: `correctness`, `security`, `reliability`, `compatibility`, `testGap`.

### 3.5 Catálogo de route v1

| id | Agente OC2 | Escribe | Cuándo | No le corresponde |
|---|---|---|---|---|
| `inline` | el actual | no | pregunta corta, explicar un archivo | cualquier cambio de código |
| `plan` | `plan` | no | diseño, exploración, review de enfoque | implementar |
| `build` | `build` | sí | implementar, refactor, tests | preguntas, explicar, revisar |
| `review` | `plan` + `jev_review_diff` | no | hay diff / “revisá esto” | escribir el fix |
| `find` | `plan` + `jev_find` | no | “dónde se hace X” | explicar o cambiar X |
| `human` | — | no | close call, secretos, migrations irreversibles | tareas rutinarias |

Cada opción se pasa a Jev con criteria estructurados (`what` / `not_for` / `examples`), que es el formato que recomiendan las docs de TypeSafe para Choice. Si `not_for` está vacío, esa opción gana todo (lección de delegate).

### 3.6 Pick (experimento Vini, 2026-09-17)

Datos de referencia (3 tareas, Gemini):

| Setup | Pasos | Tools | Llamadas Jev | Tokens in/out | Tiempo | Calidad |
|---|---|---|---|---|---|---|
| gemini-direct (elige tool + reasoning) | 10 | 9 | 0 | 85.407 / 3.019 | 18.6 s | 3/3 |
| jev-classifier (Jev elige, Gemini llena args, reasoning off) | 12 | 9 | 12 | 10.964 / 1.553 | 21.6 s | 3/3 |

Lectura honesta: ~8x menos input del LLM, +3 s, misma calidad **en 3 casos**, sin contar el costo de Jev. Es una señal, no una medición. Parte del ahorro viene de apagar reasoning y de un loop propio donde el LLM no ve el roster.

**Implicancia para OC2:** si Pick es una tool que el LLM llama, el LLM sigue recibiendo todos los schemas → el ahorro no aparece. Por eso Pick va en el hook `context`:

1. Antes de cada llamada al modelo (agentes `plan`/`build`, request `primary`), armar el roster desde `event.tools`.
2. Un Choice de Jev: opciones = ids del roster + `no_tool`.
3. Decidir qué conservar:
   - confidence < `PICK_MIN` o timeout/error → **no podar** (fail open)
   - `closeCall` (margen < `PICK_MARGIN`) → conservar top 2
   - si no → conservar top 1
   - `no_tool` → conservar solo `ALWAYS_KEEP`
   - siempre sumar `ALWAYS_KEEP` (arranca con `read`; se ajusta con el eval)
4. Borrar el resto de `event.tools`. Solo afecta esa llamada, no el historial.
5. Opcional: bajar reasoning con `event.options` en un hook scopeado por provider.

Jev **no** genera argumentos. El LLM los llena con lo que quede disponible.

⚠️ **Riesgo de costo:** cambiar el set de tools en cada request puede invalidar el prompt caching del provider (las tools suelen estar al principio del prefijo cacheado). La Fase 4 mide costo **con caché** contra baseline. Si la poda sale más cara, alternativa: 2–3 subsets estables (lectura / edición / review) en lugar de podar a 1–2 tools.

---

## 4. Contratos de tools

Todas las tools devuelven a OC2 un `content` corto en markdown (`summary_md` + `runId`). El JSON completo va a `ctx.storage` bajo `runs/<runId>`. Así el LLM no recibe 40 KB de JSON.

Todo resultado persistido incluye: `source` (`"jev"` | `"llm"`), `questionsVersion`, `usage`, `latencyMs`.

### 4.1 `jev_review_diff`

Input:
```ts
{
  base?: string        // default: working tree vs HEAD
  save?: boolean       // default true (a ctx.storage)
}
```

Output: `ReviewReport` de jev-review (matrix, profiles, findings, action) + `summary_md`.

Pipeline (no negociable):
1. Descubrir archivos JS/TS cambiados vía `ctx.vcs.diff` (py/go en v1.5). Aplicar deny globs.
2. **Screen:** 1 request por archivo con las 5 preguntas noul juntas (corren en paralelo del lado de Jev). Hasta `CONCURRENCY` archivos a la vez.
3. Filtrar señales ≥ `SCREEN_THRESHOLD`.
4. Profile top `MAX_PROFILES`.
5. Locate top `MAX_FOLLOW_UPS` señales, cada una en pasos separados: hunk → mechanism → severity → owner (si severity ≥ `OWNER_SEVERITY`).
6. Action = `request_changes` (alguna severity ≥ `BLOCKING_SEVERITY`) | `comment` | `clean`. No existe `approve`.
7. Guardar en `ctx.storage` la decisión de la sesión (la leen los gates).

### 4.2 `jev_review_path`

Igual que el codebase mode de jev-review. Si hay más de `CONFIRM_FILES_CODEBASE` archivos, devuelve `needsConfirmation: true` con el conteo y no corre. Cap duro en `MAX_FILES_CODEBASE`.

### 4.3 `jev_find`

Input:
```ts
{
  query: string
  root?: string        // default ctx.location.directory
  threshold?: number   // default 0.81
  glob?: string        // default **/*.{ts,tsx,js,jsx,py,rs,go}
  limit?: number       // default 20
}
```

Output:
```ts
{
  query: string
  model: "jev-latest"
  hits: Array<{ file: string; startLine: number; endLine: number; snippet: string; probability: number }>
  scannedSegments: number
  truncated: boolean   // true si se llegó a MAX_FIND_SEGMENTS
  summary_md: string
}
```

Pregunta por segmento: noul “¿`segments[i].text` contiene evidencia directa de `query`?”. Criteria `false`: coincidencias solo de nombre, comentarios que solo mencionan el término, docs/README.

Batching: `FIND_BATCH` segmentos en un mismo `state`, una pregunta por segmento apuntando a su path. Medir en Fase 3 contra 1 segmento por request (las docs recomiendan dar a cada pregunta solo el contexto que necesita; puede haber pérdida de precisión).

### 4.4 `jev_route`

Input:
```ts
{
  task: string
  wantsWrite?: boolean
  filesHint?: string[]
  failingTests?: boolean
}
```

Output:
```ts
{
  choice: "inline" | "plan" | "build" | "review" | "find" | "human"
  probabilities: Record<string, number>
  confidence: number
  closeCall: boolean     // margen < ROUTE_MARGIN || confidence < ROUTE_MIN_CONFIDENCE
  reasonCodes: string[]  // deterministas, ver abajo
  source: "jev" | "llm"
  summary_md: string
}
```

`reasonCodes` los genera el código, no Jev: `close_call`, `low_confidence`, `write_requested`, `failing_tests`, `touches_denied_path`, `llm_fallback`.

Efecto: guarda la decisión por sesión en `ctx.storage`. Si `choice` es `plan` o `build` y no hay `closeCall`, el command puede hacer `ctx.session.switchAgent`.

### 4.5 `jev_pick_tool` (debug)

Misma lógica que el hook de §3.6 pero invocable a mano para inspeccionar decisiones. No se expone al agente por defecto (`options.exposePickTool: false`).

---

## 5. Preguntas Jev

Todas en `src/jev/questions.ts` con `QUESTIONS_VERSION` para que los evals sean comparables entre cambios.

### Review screen (por archivo, 1 request)
Cinco noul con `inspect: file.patch`, criteria `true`/`false` con ejemplos, y `not_for` de estilo/naming:
- `correctness`: el patch introduce comportamiento runtime incorrecto
- `security`: debilita un boundary
- `reliability`: crash / race / leak / mal recovery
- `compatibility`: rompe un caller, protocolo o formato
- `testGap`: cambia comportamiento importante sin test dirigido

### Review locate (por señal, pasos separados)
1. Choice del hunk + `noMatch`
2. Choice del mechanism según dimensión
3. Score 0–3 de severity
4. Si severity ≥ `OWNER_SEVERITY`: Choice de owner (`security` / `api` / `runtime` / `testing` / `maintainer`)

No mezclar pasos dependientes en el mismo request (jev-review los separó a propósito). Sí se pueden correr señales distintas en paralelo.

### Route
Un Choice. Instructions: elegir quién *debe* ejecutar la tarea, no quién *podría*. Criteria = tabla §3.5.

### Pick
Un Choice. Instructions: la única tool que hay que llamar ahora; `no_tool` si falta un dato o ya se puede responder. Criteria = descripción recortada de cada tool. State = `{ goal, lastTool, roster }`.

### Find
Ver §4.3.

### Código como dato
El código va siempre dentro de `state` bajo claves explícitas (`file.patch`, `segments[i].text`) y las instrucciones lo tratan como material a evaluar. Un comentario en el código que “le hable” al revisor no debe cambiar la respuesta; se incluye como caso en el eval (§11).

---

## 6. Integración OpenCode 2

### 6.1 Plugin

Verificado en docs de OC2: `Plugin.define`, `ctx.options`, `ctx.tool.transform` con `namespace`, `ctx.command.transform`, `ctx.session.hook`, `ctx.permission.hook`, `ctx.vcs.diff`, `ctx.storage`.

```ts
import { Plugin } from "@opencode/plugin"

export default Plugin.define({
  id: "jev-gate",
  async setup(ctx) {
    const cfg = loadConfig(ctx.options, process.env)       // umbrales + apiKey
    const jev = cfg.apiKey ? createJevClient(cfg) : undefined
    const deps = {
      jev,
      vcs: vcsAdapter(ctx),                                // ctx.vcs.diff
      files: filesAdapter(ctx.location.directory, cfg.denyGlobs),
      store: storeAdapter(ctx.storage),
      cfg,
    }

    await ctx.tool.transform((editor) => {
      editor.namespace({
        name: "jev",
        description: "Señales tipadas de Jev (review, find, route). No genera código ni parches.",
      })
      editor.add({
        name: "review_diff",                               // efectivo: jev_review_diff
        description: "Review staged del diff actual con Jev. Devuelve findings con file:line.",
        input: reviewDiffSchema,                           // JSON Schema
        options: { namespace: "jev" },
        execute: async (input, tool) => {
          await tool.progress({ status: "screening" })
          const report = await reviewDiff(deps, input)
          return { content: report.summary_md }
        },
      })
      // review_path, find, route (+ pick_tool si cfg.exposePickTool)
    })

    await ctx.command.transform((editor) => {
      editor.add({
        name: "jev-review",
        description: "Review del diff con Jev",
        execute: async ({ sessionID, prompt, delivery }) => {
          await ctx.session.switchAgent({ sessionID, agent: "plan" })
          await ctx.session.prompt({
            ...prompt,
            sessionID,
            delivery,
            text: `Llamá jev_review_diff. Listá solo sus findings; no agregues findings propios como si fueran de Jev.\n\n${prompt.text}`,
          })
        },
      })
      // jev-find, jev-route
    })

    if (jev && cfg.pick.enabled) await registerPickHook(ctx, deps)   // §6.3
    await registerPermissionGate(ctx, deps)                          // §6.4
  },
})
```

### 6.2 Carga y opciones

`.opencode/plugins/jev-gate/` se carga solo. `opencode.jsonc` hace falta únicamente para pasar options:

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "plugins": [
    {
      "package": "./.opencode/plugins/jev-gate",
      "options": {
        "screenThreshold": 0.7,
        "findThreshold": 0.81,
        "maxFollowUps": 8,
        "pick": { "enabled": false, "alwaysKeep": ["read"] },
        "exposePickTool": false
      }
    }
  ]
}
```

⚠️ Confirmar en Fase 0 que declararlo acá y a la vez tenerlo en `.opencode/plugins/` no lo carga dos veces. Si pasa, mover el plugin fuera de `.opencode/plugins/` y dejar solo la entrada de config.

Pick arranca **apagado** hasta que la Fase 4 muestre que ahorra.

### 6.3 Hook de Pick

```ts
async function registerPickHook(ctx, deps) {
  await ctx.session.hook("context", async (event) => {
    if (!["plan", "build"].includes(event.agent)) return
    const roster = toRoster(event.tools, deps.cfg.pick.alwaysKeep)
    if (roster.length <= deps.cfg.pick.minRoster) return

    const decision = await pickTool(deps.jev, {
      goal: lastUserText(event.messages),       // ⚠️ shape de Message
      lastTool: lastToolName(event.messages),
      roster,
    }, { timeoutMs: deps.cfg.pickTimeoutMs }).catch(() => undefined)

    const keep = toKeep(decision, deps.cfg)      // pura, testeable; undefined = no podar
    if (!keep) return
    for (const id of Object.keys(event.tools)) if (!keep.has(id)) delete event.tools[id]
    await deps.store.logPick(event.sessionID, decision, keep)
  })
}
```

El hook `context` corre en cada llamada del loop del agente, incluidas las continuaciones después de una tool. No corre en compaction, title ni generate.

### 6.4 Gates con permisos

```ts
async function registerPermissionGate(ctx, deps) {
  await ctx.permission.hook("evaluate", async (event) => {
    if (!WRITE_ACTIONS.has(event.action)) return   // ⚠️ nombres exactos: edit, write, bash...
    if (event.effect === "deny") return
    const s = await deps.store.session(event.sessionID)
    if (!s) return

    if (s.review?.source === "jev" && s.review.action === "request_changes") {
      event.effect = "ask"
      event.message = `Jev: ${s.review.blockers} blocker(s) sin resolver en el último review`
      return
    }
    if (s.route?.source === "jev" && (s.route.closeCall || s.route.choice !== "build")) {
      event.effect = "ask"
      event.message = `Jev ruteó esta tarea a "${s.route.choice}"${s.route.closeCall ? " (close call)" : ""}`
    }
  })
}
```

Nunca se cambia `ask` → `allow`. Un `deny` explícito de config ni llega al hook.

### 6.5 Route automático (v1.1)

`ctx.session.hook("prompt")` corre una vez al admitir cada prompt del usuario. Ahí se puede llamar a Jev, guardar la decisión por sesión y dejar que el gate de §6.4 la aplique. No hay API de rechazo en ese hook, así que no se bloquea nada ahí: solo se decide y se guarda.

Mantener el hook idempotente: OC2 puede ejecutarlo más de una vez con submissions concurrentes.

Medir antes de activarlo por defecto (cuántos “ask” extra genera en una semana de uso).

### 6.6 Skill

`.opencode/skills/jev-gate/SKILL.md`:
- cuándo usar cada tool
- “no reportes findings propios como si fueran de Jev”
- “si route tiene closeCall → preguntá antes de editar”
- “si action = request_changes → listá blockers, no pases a build”
- “Jev no prueba defects; marca señales para revisar”
- “jev-latest no es un modelo de chat”

`plan` es el agente por defecto para review/find. `build` solo después de route = build y review sin blockers.

### 6.7 Preguntas abiertas para Fase 0

1. ¿Los tools de `event.tools` se identifican por el nombre efectivo (`jev_review_diff`, `read`)?
2. ¿Qué shape tiene `event.messages` para sacar el último texto de usuario y la última tool?
3. Nombres exactos de `event.action` en el hook de permisos para edición y shell.
4. ¿Se puede llamar `ctx.session.switchAgent` desde un command antes de `ctx.session.prompt` sin carreras?
5. ¿Doble carga si el plugin está en `.opencode/plugins/` y en `plugins` de config?
6. Nombre y versión del SDK JS de TypeSafe (el plan anterior decía `@typesafe-ai/sdk`; no verificado). Si hay dudas, vendorizar: es un `fetch` a `POST https://api.typesafe.ai/v1/systemone` con `{ state, model, questions }`, y OC2 corre en Bun, así que `fetch` es nativo.
7. Versión de `@opencode/plugin` que corresponde al `opencode2` instalado (pinnearla).
8. ¿Cuántos tokens ocupan los schemas de tools en un request típico? (define el techo de ahorro de Pick)

---

## 7. Reuso de código existente

Copiar y adaptar, no reescribir de memoria:

| Origen | Qué se copia | Qué se tira |
|---|---|---|
| jev-review `src/domain/*` | config, types, patch | nada |
| jev-review `src/review/*` | workflow + judgments (client inyectado) | scripts npm de CLI |
| jev-review `src/adapters/git.ts` | parse de diff (queda como fallback) | dashboard server |
| tsg (ideas, no el binario Rust) | threshold 0.81, segmentar + puntuar | engine Rust |
| delegate README | allowlist + `not_for` + close call | catálogo de 22 CLIs, hooks de Claude |
| jev-classifier (Vini) | idea: Jev elige, LLM llena args | loop propio |

Lenguaje: TypeScript ESM. Runtime: Bun (el de OC2). Dependencias: `@opencode/plugin` (pinneada) y client de TypeSafe (SDK o vendorizado, ver §6.7).

---

## 8. Formato de reporte para el TUI

`format.ts` produce:

```md
## Jev review (changes) — 12 archivos, 3 findings · run 2026-09-17T14:02
Threshold 0.70 · follow-ups 6 · ubicados 3 · action: request_changes

### Blockers
- src/auth.ts:88  security/authorization  sev 2.3
  hunk: se removió el check de role

### Comments
- src/api.ts:41  testGap/branch  sev 1.2

### Matrix (max p)
auth.ts  sec 0.91  rel 0.44  …
```

```md
## Jev find — "usa un channel?"  thresh 0.81 · 143 segmentos
1. src/engine.rs:1136  p=0.95
   let (tx, rx) = tokio::sync::watch::channel(...)
```

Si `truncated: true`, decirlo en la primera línea.

---

## 9. Fases de implementación

### Fase 0 — Spike doble (1 día)
**A. Jev**
- Repo de prueba con un diff chico (un `if` invertido + un check de auth borrado + un comentario que intenta manipular al revisor).
- `scripts/spike-screen.mjs`: llama screen contra ese diff.
- Confirmar key, latencia, `usage`, shape de answers (probabilidades + confidence).
- Done: JSON de screening en < 3 s para 2 archivos; el check de auth borrado sale con security ≥ 0.70.

**B. OC2**
- Plugin hello-world: 1 tool, 1 command, hook `context` que solo loguea (ids de tools, tamaño aprox. de schemas, shape de messages), hook `permission` que solo loguea (actions).
- Done: las 8 preguntas de §6.7 respondidas por escrito en este archivo.

### Fase 1 — Core library (1 día)
- Portar `domain` + `review` + `git.ts` fallback. Quitar dashboard/CLI.
- `questions.ts` con `QUESTIONS_VERSION`.
- Tests sin red: `patch.ts`, ranking de follow-ups dada una matrix, `toKeep` de Pick, reglas de gates.
- Armar los fixtures del eval (10 diffs) acá, no al final.
- Done: `runChangeReview(fixture)` con client mockeado.

### Fase 2 — Plugin mínimo: Review (1 día)
- `jev_review_diff` + `jev_review_path` + `/jev-review`.
- Persistencia en `ctx.storage`.
- Done: en un repo real, `/jev-review` muestra `summary_md` y el agente no agrega findings propios como si fueran de Jev.

### Fase 3 — Find (1 día)
- Segmentación, deny globs, batching, cap, glob + threshold, `/jev-find`.
- Medir `FIND_BATCH` = 1 vs 5 (precisión y tiempo).
- Done: “usa channel” / “heap allocation” devuelven líneas defendibles, no README ni comentarios.

### Fase 4 — Pick (1 día)
- Hook `context` + `toKeep` + log de decisiones. `jev_pick_tool` de debug.
- `scripts/eval-pick.mjs`: N prompts de agente con roster fijo y label de tool esperada.
- Comparar contra baseline **con prompt caching activo**: tokens in, costo total (LLM + Jev), tiempo, tareas completadas.
- Done: Jev acierta la tool en ≥ 80 % de los prompts etiquetados, ninguna tarea del set queda trabada por falta de tool, y el costo total baja. Si el costo no baja, probar subsets estables (§3.6) o dejar Pick apagado.

### Fase 5 — Route + gates (1 día)
- `jev_route`, `/jev-route`, gate de permisos (§6.4).
- Done: casos de §14; un review con blockers hace que la próxima edición pida confirmación.

### Fase 6 — Eval casero (medio día)
- 10 diffs con label humano (bug / clean / test-gap / manipulación en comentario).
- Script que compara findings vs labels y guarda resultados por `QUESTIONS_VERSION`.
- Buscar fallos vergonzosos (security por la palabra `token`), no accuracy de paper.
- Calibrar los umbrales *prov.* de §3.3.

### Fase 7 (opcional) — Delegate-lite
- Sumar `opencode2 run` vs sesión actual solo si hace falta multi-CLI.

Total: ~6.5 días.

---

## 10. Archivos a crear (en orden de fase)

| Fase | Archivos |
|---|---|
| 0 | `scripts/spike-screen.mjs`, plugin hello-world (se descarta) |
| 1 | `package.json`, `src/domain/{config,types,patch}.ts`, `src/jev/{client,questions}.ts`, `src/adapters/git.ts`, `src/review/{judgments,workflow}.ts`, fixtures |
| 2 | `src/adapters/{vcs,files,store}.ts`, `src/format.ts`, `index.ts` (review), `skills/jev-gate/SKILL.md` |
| 3 | `src/find/{segment,score}.ts`, registro en `index.ts` |
| 4 | `src/pick/{prune,decide}.ts`, hook en `index.ts`, `scripts/eval-pick.mjs` |
| 5 | `src/route/{catalog,decide}.ts`, `src/gates/permission.ts` |
| 6 | `scripts/eval-fixtures.mjs` |

---

## 11. Testing

### Sin red
- `parseHunks` con fixtures de unified diff
- ranking de follow-ups con matrix inventada
- `toKeep`: confidence baja → undefined; close call → top 2; `no_tool` → solo `ALWAYS_KEEP`
- gates: nunca `ask` → `allow`; ignora `source: "llm"`
- deny globs
- snapshots de `format.ts`

### Con red (opt-in)
- `TYPESAFE_API_KEY=... bun run eval:screen`
- Casos: refactor puro (todas p < 0.70), auth check removido (security alta), rama nueva sin test (testGap), comentario manipulador (no baja security del caso de auth)

### Integración OC2
- `opencode2 plugin list` muestra `jev-gate`
- sesión grabada: `/jev-review` sobre el propio plugin
- sesión con Pick activado: completar una tarea de 5+ pasos sin que falte una tool

---

## 12. Costos y límites

Contar **requests** y **preguntas** por separado (no está confirmado cómo se factura; loggear `usage` de cada respuesta).

| Operación | Requests | Preguntas |
|---|---|---|
| Screen, 15 archivos | 15 | 75 |
| Locate, 8 señales × hasta 4 pasos | ≤ 32 | ≤ 32 |
| Codebase, 80 archivos | 80 + ≤ 32 | 400 + ≤ 32 |
| Find, 200 segmentos, batch 5 | 40 | 200 |
| Pick | 1 por llamada al modelo | 1 |
| Route | 1 por tarea | 1 |

Reglas v1:
- changes mode es el default
- codebase mode pide confirmación arriba de 30 archivos
- Find corta en 200 segmentos y lo avisa
- cada run guarda `usage`, `latencyMs` y cantidad de requests
- rate limit: sin retry mágico; se marca el archivo como fallido y se sigue
- caché en `ctx.storage` por hash de (patch + `QUESTIONS_VERSION`) para screen y por hash de (segmento + query) para Find

---

## 13. Riesgos

| Riesgo | Mitigación |
|---|---|
| Alguien pone `jev-latest` como modelo de chat | skill + README |
| False positives por nombres (`auth`, `token`) | `not_for` + ejemplos false en cada noul; caso en el eval |
| Plugin API de OC2 beta cambia | aislada en `index.ts` y 2 adapters; versión pinneada |
| El agente ignora las tools y “revisa” en prosa | command fuerza `plan` + instrucción; gates no dependen del agente |
| Diffs no JS/TS | v1 solo JS/TS en review; py en v1.5 |
| Key en reports | store nunca escribe env; no hay dashboard |
| Close calls vendidos como certeza | `closeCall` con margen + confidence |
| Pick inventa una tool | Choice cerrado; solo ids del roster |
| Pick poda una tool necesaria | `ALWAYS_KEEP`, fail open, solo poda con confidence alta; eval de tareas completas |
| Poda de tools rompe el prompt caching y encarece | medir con caché en Fase 4; subsets estables o Pick apagado |
| Latencia por paso | timeout 800 ms y fail open |
| Prompt injection desde el código revisado | código solo en `state`; Jev solo endurece; no existe `approve`; caso en el eval |
| Código enviado a un tercero (TypeSafe) | opt-in por repo, deny globs, documentarlo en el README |
| Reports con fragmentos de código | van a `ctx.storage`, no al repo; si se exportan a archivos, gitignored por defecto |

---

## 14. Criterio de “listo para un repo de verdad”

- `/jev-review` sobre un diff de 5–15 archivos termina en < 30 s *(prov.)* y da al menos 1 finding con file:line y mechanism, o “sin señales ≥ 0.70”.
- `/jev-find "dónde se valida el token"` no devuelve segmentos que solo mencionan “token” en un comentario ni archivos de docs; termina en < 15 s *(prov.)*.
- `/jev-route implementá rate limit` → `build`.
- `/jev-route explicá este archivo` → `inline` o `plan`, nunca `build`.
- Después de un review con blockers, la siguiente edición pide confirmación con el motivo.
- Con Pick activado, sobre un roster de ≥ 5 tools, nunca aparece un id inventado y ninguna tarea del set de eval queda trabada.
- El agente no menciona un finding (ni un CVE) que Jev no marcó, atribuyéndoselo a Jev.
- Sin key: review y find se niegan con mensaje claro; el resto de OpenCode funciona igual.

---

## 15. Próximo paso concreto (hoy)

No armar el plugin entero. Hacer la Fase 0, las dos mitades:

```bash
git clone https://github.com/devagrawal09/jev-review
# A: extraer screenFile a scripts/spike-screen.mjs y correrlo contra el diff mínimo
# B: plugin hello-world en .opencode/plugins/ con hooks que solo loguean
opencode2   # abrir una sesión, llamar la tool y el command, leer los logs
```

Si las dos andan, Fase 1+2 en el repo donde uses OpenCode (Eflow u otro), no en un monorepo nuevo.

Orden de valor: **review_diff → find → pick → route**.
- Review: jev-review ya lo resolvió.
- Find: tsg lo demostró.
- Pick: prometedor (Vini, 3 casos), pero en OC2 hay que probar que ahorra con caché.
- Route: una vez por tarea (Lahfir); su valor real está en el gate de permisos.

---

## 16. Referencias

### Posts (X)

| Qué | Quién | URL |
|---|---|---|
| Lanzamiento de Jev + RLCD | Diogo Almeida (@CompleteSkeptic) | https://x.com/CompleteSkeptic/status/2099925682726002904 |
| jev-review: workflow de code review | Dev Agrawal (@devagrawal09) | https://x.com/devagrawal09/status/2100341005690298687 |
| tsg: grep semántico en Rust | Joey (@JoeTweets915) | https://x.com/JoeTweets915/status/2100076616042832296 |
| Jev rutea harness local (Claude Code / Codex / OpenCode) | Lahfir (@mdlahfir) | https://x.com/mdlahfir/status/2100314182201802811 |
| Plugin delegate + video | Lahfir (@mdlahfir) | https://x.com/mdlahfir/status/2100399709995356266 |
| “es solo un clasificador” / tokens de tool choice | Vini Lana (@oviniciuslana) | https://x.com/oviniciuslana/status/2100410010647793900 |
| Benchmark Gemini vs Jev classifier (~8x tokens) | Vini Lana (@oviniciuslana) | https://x.com/oviniciuslana/status/2100423271128715387 |
| jev-router para Claude Code | Pratyush Garg (@PratyushGa39620) | https://x.com/PratyushGa39620/status/2100363543476629513 |

### Repos

| Qué | URL |
|---|---|
| jev-review | https://github.com/devagrawal09/jev-review |
| tsg (ejemplo en typesafe-ai) | https://github.com/Twister915/typesafe-ai/tree/main/examples/tsg |
| typesafe-ai (repo de Joey) | https://github.com/Twister915/typesafe-ai |
| delegate (plugin Claude Code) | https://github.com/lahfir/claude-plugins/tree/main/delegate |
| claude-plugins (monorepo Lahfir) | https://github.com/lahfir/claude-plugins |
| jev-router | https://github.com/gargpratyush/jev-router |
| jev-mcp | https://github.com/jkudish/jev-mcp |
| typesafe-mod (hook Claude Code) | https://github.com/BeLazy167/typesafe-mod |
| OpenCode (agente) | https://github.com/sst/opencode |
| OpenCode (org Anomaly) | https://github.com/anomalyco/opencode |

### Docs y producto

| Qué | URL |
|---|---|
| TypeSafe AI | https://typesafe.ai |
| Docs TypeSafe / Jev | https://docs.typesafe.ai |
| Cómo construir con System One (decomposición, criteria estructurados, confidence) | https://docs.typesafe.ai/concepts/how-to-build-with-system-one |
| Intro System One | https://docs.typesafe.ai/concepts/system-one |
| API System One | https://docs.typesafe.ai/api |
| Quick start | https://docs.typesafe.ai/introduction/quickstart |
| Índice de docs para LLMs | https://docs.typesafe.ai/llms.txt |
| Blog lanzamiento Jev | https://typesafe.ai/blog/introducing-system-one-models-and-jev |
| Console / API keys | https://console.typesafe.ai/settings/keys |
| OpenCode docs (estable) | https://opencode.ai/docs |
| OpenCode V2 intro | https://opencode.ai/v2/docs/ |
| OpenCode V2 plugins (user) | https://opencode.ai/v2/docs/plugins/ |
| OpenCode V2 plugins (build: tools, commands, hooks, permisos, vcs, storage) | https://opencode.ai/v2/docs/build/plugins/ |
| OpenCode V2 migración de plugins V1 | https://opencode.ai/v2/docs/build/plugins/migrate-v1/ |
| OpenCode V2 skills | https://opencode.ai/v2/docs/skills/ |
| OpenCode V2 commands | https://opencode.ai/v2/docs/commands/ |
| OpenCode V2 models | https://opencode.ai/v2/docs/models |
| OpenCode V2 llms.txt | https://opencode.ai/v2/llms.txt |
| Jev en Vercel AI Gateway | https://vercel.com/changelog/typesafe-ai-jev-now-available-on-ai-gateway |

### Cuentas

| Handle | Rol |
|---|---|
| [@CompleteSkeptic](https://x.com/CompleteSkeptic) | CEO TypeSafe, anuncio Jev |
| [@typesafeai](https://x.com/typesafeai) | org |
| [@devagrawal09](https://x.com/devagrawal09) | jev-review |
| [@JoeTweets915](https://x.com/JoeTweets915) | tsg |
| [@mdlahfir](https://x.com/mdlahfir) | delegate |
| [@oviniciuslana](https://x.com/oviniciuslana) | jev-classifier / benchmark |
| [@PratyushGa39620](https://x.com/PratyushGa39620) | jev-router |
