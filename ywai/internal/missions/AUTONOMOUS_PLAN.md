# Plan — Missions autónomo ("dale un Goal y shippea solo")

## Context

`ywai/internal/missions` ya tiene el esqueleto del orquestador: `Goal → Plan (opencode) →
Mission → RunMission` (loop secuencial de features → validación en milestone). FSM, store
atómico, recovery, retry. Es plomería sólida pero **no es autónomo de verdad** todavía.

Tres gaps verificados bloquean la visión "ship while you sleep":

1. **El worker no toca el repo.** `worker.go` setea `cmd.Dir = contextDir` (un tmpdir vacío
   con solo markdowns); `executeViaAPI` no setea `SessionCreateOpts.Directory`. `mission.Project`
   se guarda como nombre pero nunca se resuelve a un path ni se le pasa al worker. **El worker no
   puede implementar nada real porque no ve el código.**
2. **No hay modo autónomo end-to-end.** El planning pide aprobación `y/n` (`promptApproval`).
   No existe `Goal → plan → approve → run` sin intervención.
3. **La validación es honesta pero vacía.** Tras quitar los `ValidationPassed` truchos, todo queda
   `pending`/`manual`: no corre build, ni tests, ni E2E. No hay loop run→observe→fix→verify.

Infra ya disponible que vamos a reusar:
- `ProjectStore` (`projects.go`) resuelve `name → Path` (repo real en disco).
- `SessionCreateOpts.Directory`/`.Workspace` (`session_api.go`) para apuntar al worktree en API mode.
- `ServicesManifest`/`ServiceDef` (`models.go`) con `Healthcheck`, `Start`, comandos nombrados.
- `MissionArtifacts`, `ValidationContract`, handoffs y logs persistidos.

**Decisión de arquitectura: aislamiento por `git worktree` por feature**
(rama `ywai/<mission>/<feature>`, merge a rama de integración al pasar verificación).

## Objetivo final

```
ywai missions auto "Construí X" --project myrepo --yes
        │
        ├─ plan (opencode)  ─────────────────────────────────┐
        ├─ approve (auto)                                     │
        └─ RunMission ──► para cada feature (DAG, en paralelo):
                            1. worktree aislado del repo real
                            2. worker opencode trabaja sobre el código
                            3. verify: build + test (+ E2E) en el worktree
                            4. ¿falla? self-correct: reintento con el error inyectado
                            5. ¿pasa N corridas limpias? merge a integración
                          ► milestone validation ► mission completed
                          ► REPORT.md con evidencia (handoffs, logs, screenshots)
```

## Decisiones secundarias (defaults, ajustables)

- **Fuente de comandos de verificación**: `services.yaml > commands` (ej. `build`, `test`) primero;
  fallback auto-detect (`go.mod` → `go build ./... && go test ./...`; `package.json` → script `test`).
- **Cómo aterrizan los cambios**: feature verde → merge de su rama a `ywai/<mission>/integration`.
  La rama de integración queda para que el usuario abra PR o mergee a `main`. **Nunca push automático
  a main** (alineado con la safety de la inspiración).
- **Base ref**: HEAD actual del repo del proyecto (flag `--base` para override).
- **Paralelismo inicial**: `MaxParallel = 1` (secuencial) en Fase 2; el scheduler DAG se diseña
  desde el principio para `>1` y se activa en Fase 6.
- **Clean streak**: por feature, verify exige 1 corrida limpia (Fase 3). El "3 corridas limpias
  seguidas" estilo E2E es por milestone y entra en Fase 5.

---

## Cambios al modelo de datos (transversal)

`models.go` — `Feature`, agregar:
```go
WorktreePath string      `json:"worktreePath,omitempty"` // worktree activo de esta feature
Branch       string      `json:"branch,omitempty"`       // ywai/<mission>/<feature>
LastError    string      `json:"lastError,omitempty"`    // para self-correction
VerifyRuns   []VerifyRun `json:"verifyRuns,omitempty"`   // historial de verificación
```

Nuevo tipo (`verify.go`):
```go
type VerifyRun struct {
    Passed   bool         `json:"passed"`
    Commands []CommandRun `json:"commands"` // reusa CommandRun de models.go
    RunAt    time.Time    `json:"runAt"`
    Output   string       `json:"output,omitempty"`
}
type VerifyResult struct {
    Passed   bool
    Runs     []VerifyRun
    Combined string
}
```

`missions.go` — `EngineConfig`, agregar:
```go
MaxParallel       int          // default 1
VerifyCleanStreak int          // default 1 por feature
RepoResolver      RepoResolver // interfaz para resolver Project → path (default: ProjectStore)
```

---

## FASE 1 — Worker sobre worktree del repo real  (P0, desbloquea todo)

**Meta**: el worker ejecuta opencode con el código real delante, en un worktree aislado.

### Archivos nuevos
- `workspace.go` — `WorkspaceManager`:
  ```go
  type WorkspaceManager struct {
      baseDir string    // ~/.local/share/ywai/missions/<id>/worktrees
      git     GitRunner // wrapper exec.Command para testeo
  }
  func (wm *WorkspaceManager) CreateWorktree(repoPath, missionID, featureID, baseRef string) (worktreePath, branch string, err error)
  func (wm *WorkspaceManager) RemoveWorktree(repoPath, worktreePath string, deleteBranch bool) error
  func (wm *WorkspaceManager) MergeToIntegration(repoPath, missionID, featureBranch string) error
  ```
  Comandos git: `git -C <repo> worktree add -b ywai/<id>/<feat> <wtPath> <baseRef>`,
  `git -C <repo> worktree remove --force <wtPath>`, merge con `--no-ff`.
  `GitRunner` interface (`Run(ctx, dir, args...) (stdout, stderr, err)`) para mockear en tests.

- `repo.go` — resolución Project → path:
  ```go
  type RepoResolver interface { Resolve(projectName string) (repoPath string, err error) }
  // ProjectStoreResolver implementa RepoResolver sobre ProjectStore.
  ```
  Error claro si `Project == ""` o no está registrado: `ErrNoRepo` ("autonomous run needs a
  registered project; add it with `ywai missions project add <name> <path>`").

### Archivos modificados
- `worker.go`:
  - `WorkerManager` gana `workspace *WorkspaceManager` y recibe el `repoPath` por ejecución.
  - `PrepareContext(mission, feature, worktreePath)`: en vez de tmpdir, escribe los markdowns en
    `<worktreePath>/.ywai/` (feature.md, mission.md, AGENTS.md, SKILL.md, services.yaml). El repo
    real ya está en el worktree.
  - `SpawnWorker`: `cmd.Dir = worktreePath` (no más tmpdir).
  - `executeViaAPI`: `SessionCreateOpts{..., Directory: worktreePath}`.
  - `ExecuteFeature(mission, featureID, repoPath)`: nueva firma; orquesta create-worktree →
    prepare → spawn → (Fase 3: verify) → merge-on-success → remove-worktree.
- `missions.go`:
  - `NewEngine` resuelve `RepoResolver` (default `ProjectStoreResolver`).
  - `RunMission`: resuelve `repoPath` una vez al inicio (falla temprano con `ErrNoRepo`),
    lo pasa a `ExecuteFeature`.
- `cli/commands.go`: nuevo subcomando `project` (`add`/`list`/`rm`) que envuelve `ProjectStore`.

### Tests (Fase 1)
- `workspace_test.go`: repo git temporal real (init + commit), `CreateWorktree` crea dir y rama;
  `RemoveWorktree` limpia; `MergeToIntegration` integra un commit (`t.TempDir()`).
- `repo_test.go`: `Resolve` ok / `ErrNoRepo`.
- `worker_test.go`: mock `GitRunner` + `cmdCreator`; assert `cmd.Dir == worktreePath` y que
  `executeViaAPI` manda `Directory`.

**PR boundary**: Fase 1 es un PR. Verificable: `ywai missions project add`, `start`, `run` ya corre
con el repo real.

---

## FASE 2 — Comando autónomo  (P0)

**Meta**: `Goal → plan → approve → run` sin prompts, CLI y web.

### Archivos modificados/nuevos
- `planning.go`: helper no interactivo
  ```go
  type AutoPlanOpts struct { Project, Model, Agent, BaseRef string; AutoApprove bool }
  func PlanAndApprove(store *MissionsStore, goal string, opts AutoPlanOpts) (*Mission, error)
  ```
  Reusa `GeneratePlanWithOpencode` + `CreateMissionFromPlan` + `ApprovePlan`. Sin prompts.
- `cli/commands.go`: `newAutoCmd()` → `missions auto "<goal>"` flags `--project --model --agent
  --base --yes --max-retries --timeout --max-parallel`. Flujo: `PlanAndApprove` → `RecoverEngine`
  → `engine.RunMission`. Stream de eventos a stdout (patrón ya en `runRun`).
- `web/handlers.go` + `web/server.go`: `POST /api/missions/auto` con `{goal, project, model, agent}`
  → `PlanAndApprove` → `RunMission` en goroutine (patrón ya usado en el handler `RunMission`)
  → responde `{missionId}` y streamea por el hub/WebSocket.
- `control/web` (React): botón "Auto mission" en `CreateMissionModal.tsx`/`Missions.tsx`.

### Tests (Fase 2)
- `planning_test.go`: `PlanAndApprove` deja la misión en `active` sin tocar stdin.
- `cli/commands_test.go`: `auto` con opencode mockeado → misión activa.
- `web/handlers_test.go`: `POST /api/missions/auto` → 200 + missionId; goal vacío → 400.

**PR boundary**: Fase 2 es un PR. Verificable: `ywai missions auto "..." --project x --yes` corre
de punta a punta (todavía sin gate de verificación real).

---

## FASE 3 — Gate de verificación real  (P1)

**Meta**: una feature solo se completa si el worker entregó handoff **y** la verificación pasa.

### Archivos nuevos
- `verify.go` — `Verifier` pluggable:
  ```go
  type Verifier interface { Verify(ctx, worktreePath string, mission *Mission, feature *Feature) (VerifyResult, error) }
  type CommandVerifier struct { cmdCreator func(...) *exec.Cmd }
  func DetectVerifyCommands(worktreePath string) []string // go/npm/etc.
  ```
  Lee `services.yaml > commands.build` y `.test`; si no hay, auto-detecta. Cada comando es un
  `CommandRun`. `Passed = todos exit 0`.

### Archivos modificados
- `worker.go` `ExecuteFeature`: tras worker OK, antes de `CompleteFeature`, corre `verifier.Verify`.
  Falla → `FailFeature` (cuenta como retry) + persiste `VerifyRun` + `LastError`. Pasa →
  `MergeToIntegration` → `CompleteFeature`.
- `missions.go`: `Engine` construye el `Verifier` y lo pasa a workers.
- `validation.go`: el pipeline de milestone reusa `VerifyResult` como evidencia real en vez de los
  `pending` actuales.

### Tests (Fase 3)
- `verify_test.go`: repo go temporal que compila/falla → `Passed` correcto; `DetectVerifyCommands`
  por tipo de repo; timeout mata el proceso.
- `worker_test.go`: feature con verify-fail no llega a `completed` ni mergea; verify-pass mergea.

**PR boundary**: Fase 3 es un PR. Verificable: tests rotos → feature NO completa; tests verdes →
completa y aparece en la rama de integración.

---

## FASE 4 — Self-correction (reintento con feedback)  (P1)

**Meta**: el fallo informa el próximo intento, no se repite a ciegas.

### Cambios
- `worker.go` `PrepareContext`: si `feature.LastError != ""` o hay `verifyRuns` fallidas, agrega a
  `feature.md` una sección `## Previous attempt failed — fix this` con el error y el output de verify.
- `queue.go`/`store.go`: `FailFeature` persiste `LastError` (del handoff/verify).
- `missions.go` `RunMission`: al reencolar para retry, conserva `LastError`.

### Tests (Fase 4)
- `worker_test.go`: tras un fallo, el `feature.md` del segundo intento contiene la sección de error.
- `store_test.go`: `LastError` persiste y se limpia al completar.

**PR boundary**: Fase 4 es un PR pequeño sobre Fase 3.

---

## FASE 5 — Clean-streak + E2E pluggable  (P2, el diferencial de la inspiración)

**Meta**: por milestone, exigir "N corridas limpias seguidas" (default 3), con E2E enchufable.

### Cambios
- `validation.go`: loop de clean-streak en `RunValidation` — corre el `Verifier` (y E2E si está
  configurado) hasta `VerifyCleanStreak` pasadas consecutivas; cualquier fallo resetea el contador.
- `verify.go`: segundo `Verifier` — `E2EVerifier` (interface estable; implementación inicial puede
  invocar `agent-browser`/`tuistory` cuando exista). El contrato ya está en `ValidationContract.Tool`.
- Config: `VerifyCleanStreak` en `EngineConfig`/`ValidationConfig`, flag `--clean-streak`.

### Tests
- `validation_test.go`: streak resetea ante fallo; completa al alcanzar N; E2E mock.

**PR boundary**: Fase 5 es un PR. E2E real (browser) queda como extensión documentada.

---

## FASE 6 — Scheduler paralelo (DAG) + port blocks  (P2)

**Meta**: correr features en paralelo respetando `Preconditions`, con puertos aislados.

### Cambios
- `queue.go`: `ReadyFeatures(mission) []*Feature` = pending con todas las `Preconditions`
  completadas (el DAG ya está en el modelo: `Feature.Preconditions`).
- `missions.go` `RunMission`: reemplazar el loop secuencial por un pool con semáforo `MaxParallel`;
  cada feature en su worktree (aislado desde Fase 1). Recolectar resultados, recomputar ready set,
  repetir hasta drenar el DAG.
- `workspace.go`: `AllocatePortBlock() (base int, release func())` — rango por feature, inyectado
  como env `YWAI_PORT_BASE` al worker.
- Merge serializado a integración (lock) aunque las features corran en paralelo.

### Tests
- `queue_test.go`: `ReadyFeatures` respeta preconditions; ciclo detectado → error.
- `missions_test.go`: con `MaxParallel=2` y mocks, dos features concurrentes mergean sin pisarse.

**PR boundary**: Fase 6 es un PR (el más delicado: concurrencia + merge serializado).

---

## FASE 7 — Return Ready (evidencia)  (P3)

**Meta**: entregable con evidencia, como la inspiración.

### Cambios
- `report.go`: genera `missions/<id>/report/REPORT.md` con handoffs, `VerifyRun`s, logs y links a
  screenshots (cuando E2E los produzca). Se arma al completar la misión en `RunMission`.
- `web`/React: vista "Report" por misión.

---

## Estrategia de entrega

7 fases = 7 PRs encadenados (cada uno < ~400 líneas, verificable solo). Orden estricto: la Fase 1
desbloquea todo; Fases 2–4 dan el "auto + verify + self-correct" mínimo usable; 5–7 son el
diferencial (clean-streak/E2E, paralelismo, evidencia). Sugerido `feature-branch-chain` con rama
tracker `feat/missions-autonomous` y PRs hijos apuntando al anterior.

Recomendado ejecutar vía SDD (`/sdd-new missions-autonomous`) para spec+design por fase.

## Verificación global (end-to-end)

1. `ywai missions project add demo /ruta/a/repo-go-de-prueba`
2. `ywai missions auto "agregá un endpoint /health que devuelva 200 ok" --project demo --yes`
3. Observar: plan generado → worktrees creados (`git -C repo worktree list`) → opencode corre en el
   worktree → `go build && go test` como gate → merge a `ywai/<id>/integration` → misión `completed`.
4. `git -C repo log ywai/<id>/integration` muestra los commits de las features.
5. `ywai missions show <id>` → features `completed` con `verifyRuns` verdes; `REPORT.md` generado.
6. Casos borde: repo no registrado → `ErrNoRepo` claro; tests rotos → feature NO completa y
   reintenta con el error inyectado; cancelación a mitad → worktrees limpiados por `RecoverEngine`.

Cada fase: `gofmt -l` vacío, `go vet` limpio, `go test ./internal/missions/...` verde.
