# ywai — release note

**Hacé esto:** `ywai update` → reiniciá OpenCode → `ywai doctor`

`ywai update` ya no rehace lo instalado: Engram/ado/skills/`engram setup` se saltan si no cambiaron.

**Graft ahora es Trail.** `ywai install`/`update` instala `@trailhq/trail-code` (CLI `trail`), desinstala `@nanonets/graft` y reescribe el MCP `graft` como `trail` (`trail mcp`) en OpenCode, Claude Code y pi. Los permisos pasan de `graft_*` a `trail_*`. El grafo sigue en `<repo>/graft`; `.trail/learnings/` se commitea.

**Los learnings van a Engram, no a Trail.** `ywai install`/`update` saca el hook `Stop` de trail de `~/.claude/settings.json` (el que bloquea el turno pidiendo `trail note`; el resto de los hooks de trail quedan) y escribe la sección gestionada `Trail learnings → Engram` en `~/.config/opencode/AGENTS.md` y `~/.claude/CLAUDE.md`. Corre en cada update porque `trail init`/upgrade vuelve a agregar el hook; si ya tenías la sección a mano, se reemplaza por la gestionada.

Si OpenCode no arranca (`ConfigInvalidError` en `mcp.trail`): el update reescribe el entry al formato de OpenCode (`type: local`, `command` array).

---

## Obligatorio — qué cambia para vos

1. **Vision-bridge ahora corre.** Las imágenes en modelos sin visión se describen. El `attachment: false` de la config gana sobre el catálogo.
2. **CodeGraph se fue. Trail (ex Graft) es el indexador.** Los MCP `codegraph` que quedaron se limpian en install/update.
   Por qué Trail: local, $0, sin API key; grafo exacto (file:line + callers); determinístico y versionado con git.
3. **`ywai` instala y configura Engram.** Ya no toca gentle-ai. Baja el binario de Gentleman-Programming/engram y escribe el MCP en opencode, pi, omp y claude-code (si los tenés). En opencode y pi también corre `engram setup`.
4. **`ywai update` limpia la cache de plugins de OpenCode** (statusline, ponytail, ADO/quota retirados) para resolver la última versión publicada.
5. **`ywai install` arranca el control server** si está apagado (`serve --background --no-update`). `--dry-run` no lo arranca.

---

## Skills / agentes nuevos

| Qué | Para qué |
|---|---|
| `ywai` | Skill para agents: prenden/apagan MCP, cambian profile, habilitan grupos. El humano no corre esa CLI |
| `learn-ywai` | Slash command + skill. Enseña ywai con la docs oficial embebida |
| `diks` | Notas de infra en `Infra/wiki` (Obsidian/Zettelkasten) |
| `i-have-adhd` | Estilo de salida: acción primero, pasos numerados, sin recap |

## Skills actualizados

| Qué | Qué cambió |
|---|---|
| diagnosing-bugs | Redactar secretos + template HITL |
| tdd | Cuándo mockear + ejemplos buenos y malos |

---

## Breaking / se fue

- **CodeGraph MCP + CLI** retirados. Usá Trail.
- **gentle-ai** ya no forma parte de `ywai install` / `update`.
- Plugins viejos después del update: reiniciá OpenCode una vez. La cache se limpia; OpenCode resuelve de nuevo al arrancar.

---

## Upgrade — 3 pasos

1. `ywai update`
2. Reiniciá OpenCode (obligatorio — plugins + limpieza MCP)
3. `ywai doctor`

Opcional: pedile al agent que use el profile `inherit` (skill `learn-ywai`) si querés que los agentes sigan el modelo de la sesión.

`inherit` = sin modelos fijos. `balanced` ahora usa **DeepSeek** (`deepseek-v4-pro` / `deepseek-v4-flash`) en lugar de Grok/Minimax.

---
