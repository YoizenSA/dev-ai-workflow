# Flujo git y mensajes de commit

Referencia de `diks`. Leer antes de cualquier `git push` al repo de documentación.

### Flujo de sincronización con git

Antes de cualquier `git push`, el agente debe **verificar el estado del working tree y si el remoto avanzó** (puede haber commits de otra persona):

1. **Verificar estado limpio primero**: `git status`. Si hay cambios sin stagear, archivos staged o un merge a medio terminar (`MERGE_HEAD`), el `pull --rebase` va a fallar. Limpiar antes:
   - `git merge --abort` si existe `MERGE_HEAD` (merge interrumpido).
   - `git stash push --include-untracked -m "<motivo>"` para guardar cambios sueltos sin perderlos, y `git stash pop` después del rebase.
   - Si hay archivos staged que ya existen en el remoto (verificables con `git show origin/main:"<ruta>"`), descartar el staged con `git reset HEAD "<ruta>"` — nunca perder contenido sin comparar primero (usar hashes: `Get-FileHash` en Windows, `md5sum` en Linux).
2. `git fetch origin` para comparar local vs remoto.
3. Si `origin/main` tiene commits que el local no tiene (divergencia o adelanto remoto), hacer `git pull --rebase origin main` **antes** de pushear.
   - Si hay conflictos, resolverlos y continuar el rebase; no pushear forzado (`--force`) bajo ninguna circunstancia.
4. Recién después, `git push origin main`.

Regla práctica: `status limpio` → `fetch` → si el remoto avanzó, `pull --rebase` → `push`. Esto evita que se pisen cambios de otros miembros del equipo y que el rebase falle por estados sucios del working tree.

### Conventional Commits para mensajes de commit

Todos los commits generados por DIKS deben seguir el estándar **Conventional Commits**:

**Formato:** `type(scope): subject`

- `type`: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `chore`, `ci`, `build`
- `scope`: área afectada (ej. `tickets`, `auditorias`, `firewall`, `diks`, `canvas`)
- `subject`: en **imperativo** y español, máximo 50 caracteres. **Nunca en pasado** ("fixeó", "agregó").

**Reglas:**
- Una línea de subject obligatoria, separada del body por una línea en blanco.
- Body opcional explicando el **qué** y **por qué** del cambio.
- Para cambios incompatibles con versiones anteriores: footer `BREAKING CHANGE:`.
- Un solo cambio lógico por commit. No mezclar fixes con features ni refactors.
- El tipo correcto determina el versionado SemVer: `fix` → patch, `feat` → minor, `BREAKING CHANGE` → major.

**Ejemplos:**
- `feat(tickets): v2.1 - corregir StartDate y agregar OriginalEstimate`
- `fix(diks): corregir regla de autor para inferir de git config`
- `docs(auditorias): agregar nota de control A.8.7 con decision Azure Defender`

