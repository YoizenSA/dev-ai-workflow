# Modos de TDD en `sdd-apply`

`sdd-apply` resuelve uno de tres modos automáticamente. Solamente el módulo
strict carga tokens extra — los otros dos se mantienen livianos.

## Resolución de modo

```
Level 1 — Config file:
  sdd/config.yaml → rules.apply.tdd: true
                    rules.apply.strict_tdd: true   (opt-in para Strict)

Level 2 — Skills / convenciones detectadas del proyecto

Level 3 — Infra de testing presente:
  **/*.test.*, **/*.spec.*, jest.config.*, vitest.config.*, pytest.ini, ...

Level 4 — Default:
  TDD off → Standard Mode
```

Reglas de resolución:

- **Strict TDD** — `strict_tdd: true` Y hay un test runner disponible.
- **Light TDD** — TDD on pero no strict.
- **Standard** — TDD off.

## Standard Mode

Escribís código directamente desde los specs. Sin disciplina test-first.

```
FOR EACH TASK:
├── Leer tarea + spec + design + código existente
├── Escribir la implementación
├── Auto-verificar contra los scenarios del spec
└── Marcar [x]
```

## Light TDD Mode

El `tasks.md` contiene triplets `[RED]/[GREEN]/[REFACTOR]`:

```
[RED] task:
├── Leer el spec scenario
├── Escribir test que falla
├── Correr el test → TIENE que fallar
└── Marcar [x]

[GREEN] task:
├── Escribir el MÍNIMO de código para que pase
├── Correr el test → TIENE que pasar
└── Marcar [x]

[REFACTOR] task:
├── Limpiar (naming, duplicación, extract)
├── Correr el test → SIGUE pasando
└── Marcar [x]
```

## Strict TDD Mode

Carga `skills/sdd-apply/strict-tdd.md`. Agrega:

### Safety Net
Antes de modificar archivos existentes, corré los tests actuales. Si alguno falla:

- STOP y reportá como `pre-existing failure`.
- **No** "arregles" los fallos pre-existentes — pertenecen a su propio cambio.

### Triangulation
GREEN con un solo test case no alcanza. Agregá un segundo test case con
inputs distintos para forzar a la implementación a salir de los hardcodes
"fake it". Mínimo 2 test cases por comportamiento (excepto tareas puramente
estructurales — hay que justificarlo explícitamente).

### Assertion Quality Rules (patrones BANEADOS)

- `expect(true).toBe(true)` — tautología.
- `expect(result).toEqual([])` sin precondición que explique por qué está vacío.
- `expect(result).toBeDefined()` solo — hay que probar el valor.
- `for (const x of items) expect(...)` cuando `items.length === 0` — ghost loop.
- Smoke tests (`render(<X/>); expect(wrapper).toBeInTheDocument()`) — no es un test.
- Asserts sobre clases CSS (`expect(el.className).toContain(...)`) — nunca.
- Acoplamiento a detalles de implementación (`component.state`, conteos de llamadas a mocks).

### Higiene de mocks
- ≤ 3 mocks → está bien.
- 4–6 mocks → considerá extraer una función pura.
- ≥ 7 mocks → STOP. Layer equivocado. Extraé lógica o movelo a integration.

### Hard Gate
Las corridas en Strict TDD TIENEN que producir una tabla **TDD Cycle Evidence**:

```
| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| 1.1  | foo.test.ts | Unit | ✅ 5/5     | ✅ Written | ✅ Passed | ✅ 3 cases | ✅ Clean |
```

`sdd-verify` **rechaza** el cambio si esta tabla falta o está incompleta
cuando Strict TDD estaba activo.

## Activar Strict TDD

Editá `sdd/config.yaml`:

```yaml
rules:
  apply:
    tdd: true
    strict_tdd: true
    test_command: "pnpm vitest run"   # override opcional
```

O seteá el equivalente en el contexto de proyecto de engram. Una vez activo,
cada `/sdd-apply` en este proyecto va a:

1. Cargar `strict-tdd.md`.
2. Correr el Safety Net.
3. Requerir RED → GREEN → TRIANGULATE → REFACTOR por tarea.
4. Emitir la tabla de Evidence.

## Desactivar Strict TDD

Sacá `strict_tdd: true` o ponelo en `false`. `sdd-apply` vuelve a Standard o
Light automáticamente. El módulo `strict-tdd.md` no se carga, no se parsea,
no cuenta tokens.

## Referencias

- Skill: `skills/sdd-apply/SKILL.md`
- Módulo strict: `skills/sdd-apply/strict-tdd.md`
- Gate de verify: `skills/sdd-verify/SKILL.md` (sección 6: Strict TDD gate)
