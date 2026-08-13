# ywai — release note

**Hacé esto:** `ywai update` → reiniciá OpenCode → `ywai doctor`

Si OpenCode no arranca (`ConfigInvalidError` en `mcp.graft`): el update reescribe el entry al formato de OpenCode (`type: local`, `command` array, `enabled`).

---

## Obligatorio — qué cambia para vos

1. **Vision-bridge ahora corre.** Las imágenes en modelos sin visión se describen. El `attachment: false` de la config gana sobre el catálogo.
2. **CodeGraph se fue. Graft es el indexador.** Los MCP `codegraph` que quedaron se limpian en install/update.
   Por qué Graft: local, $0, sin API key; grafo exacto (file:line + callers); determinístico y versionado con git.
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
| `experiment/infra-docs` + `infra-docsv2` | Agentes primarios para docs DIKS de infra |

## Skills actualizados

| Qué | Qué cambió |
|---|---|
| diagnosing-bugs | Redactar secretos + template HITL |
| tdd | Cuándo mockear + ejemplos buenos y malos |

---

## Breaking / se fue

- **CodeGraph MCP + CLI** retirados. Usá Graft.
- **gentle-ai** ya no forma parte de `ywai install` / `update`.
- Plugins viejos después del update: reiniciá OpenCode una vez. La cache se limpia; OpenCode resuelve de nuevo al arrancar.

---

## Upgrade — 3 pasos

1. `ywai update`
2. Reiniciá OpenCode (obligatorio — plugins + limpieza MCP)
3. `ywai doctor`

Opcional: pedile al agent que use el profile `inherit` (skill `ywai`) si querés que los agentes sigan el modelo de la sesión.

`inherit` = sin modelos fijos. `balanced` ahora usa **DeepSeek** (`deepseek-v4-pro` / `deepseek-v4-flash`) en lugar de Grok/Minimax.

---
