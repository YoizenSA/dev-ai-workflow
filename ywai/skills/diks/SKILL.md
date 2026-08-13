---
name: diks
description: Use when the user wants to document infrastructure work, incidents, decisions, runbooks, procedures, or concepts in the Infra documentation repository as Zettelkasten-compatible Markdown notes. Triggers on phrases like "documentar", "crear nota", "nueva página", "runbook", "procedimiento", "incidente", "postmortem", "decisión de arquitectura", or when the user asks to convert technical work into wiki documentation.
license: MIT
compatibility: opencode
metadata:
  domain: infrastructure
  language: es
  repository: Infra/wiki
  editor: Obsidian
  scope: repo
---

# DIKS — Dynamic Infrastructure Knowledge System

Sistema de asistencia para documentar conocimiento técnico de infraestructura en el repo de documentación de Infra (`Infra/wiki`) usando notas atómicas, conexiones explícitas y convenciones compatibles con Obsidian.

## Propósito

Reducir la fricción de documentar. El agente ayuda a:

1. Analizar la información técnica entregada por el usuario.
2. Estructurarla como una nota atómica siguiendo la ontología DIKS.
3. Conectarla con notas existentes mediante enlaces internos.
4. Generar el archivo `.md` en la ubicación correcta dentro de `Infra/`.
5. Presentar la nota al usuario para validación.

El humano siempre valida antes de hacer `git add / commit / push`.

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

## Cuándo usar esta skill

- El usuario dice: "documentar esto", "crear una nota", "agregar a la wiki".
- El usuario describe un cambio, incidente, procedimiento o decisión técnica.
- El usuario pide convertir logs, outputs de pipeline o conversaciones en documentación.
- El usuario menciona explícitamente DIKS, Zettelkasten, Obsidian o el repo de documentación de Infra.

## Flujo de trabajo

### 1. Entender la entrada

Antes de escribir nada, identificar:

- **Tema:** ¿de qué trata la nota?
- **Tipo:** ¿procedimiento, runbook, incidente, decisión, concepto, configuración?
- **Área:** ¿Firewall, VPNs, Zabbix, GLPI, NGINX, ArgoCD, OCI, etc.?
- **Audiencia:** ¿solo infraestructura técnica o también onboarding/didáctica?
- **Relaciones:** ¿qué notas existentes menciona o afecta?
- **Evidencia:** ¿hay imágenes, logs, comandos, URLs, diagramas?

### 2. Elegir ubicación

La estructura base es:

```text
<raíz del repo Infra/wiki>/
└── Infra/
    ├── <Área>.md
    ├── <Área>/
    │   └── <Nota>.md
    └── ...
```

Reglas:

- Si la nota pertenece a un área existente (ej. Firewall, VPNs), guardarla en `Infra/<Área>/`.
- Si el área no existe, crear `Infra/<Área>/` y `Infra/<Área>.md` como índice.

### 2b. Área especial: Auditorias

El área `Auditorias` tiene una estructura propia para documentar auditorías ISO 27001 y PCI DSS:

```text
Infra/Auditorias/
├── Auditorias.md
├── ISO-27001.md
├── ISO-27001/
│   ├── ISO-27001-<Año>.md
│   └── <Control>.md
├── PCI-DSS.md
└── PCI-DSS/
    ├── PCI-DSS-<Año>.md
    └── <Control>.md
```

Reglas del área Auditorias:

- Nota `auditoria`: una por año y normativa. Nombre: `<Normativa>-<Año>` (ej. `ISO-27001-2025`).
- Nota `control`: una por tópico/ticket, con historial por año. Nombre legible del tópico.
- Los links a Google Drive van como URL directa en la sección del año correspondiente.
- Los tickets de ClickUp se referencian con link en cada sección de año.

### 3. Nombrar el archivo

- El nombre de archivo debe coincidir con el `title` del frontmatter, sin extensión.
- Usar espacios y guiones normales: `pfSense - Firewall.md`, `VPN AT&T.md`.
- No usar caracteres reservados de Windows: `\ / : * ? " < > |`.
- Evitar tildes y eñes en nombres de archivo si es posible; usar `n` en lugar de `ñ`.
- Los enlaces internos `[[Nombre]]` usan el nombre del archivo sin extensión.

### 4. Estructura de la nota

Toda nota DIKS debe usar este template base:

```markdown
---
id: <AAAAMMDD-NNN>
title: "<Título legible>"
type: <tipo>
area: <Área>
tags: [<tag1>, <tag2>]
creado: <AAAA-MM-DD>
autor: <inicial o nombre>
pedagogico: false
---

# <Título legible>

## Idea atómica

Una sola oración que capture el conocimiento esencial de la nota.

## Contexto

¿Por qué existe esta nota? ¿Qué problema resuelve? ¿Cuándo aplica?

## Contenido

### Subsecciones según el tipo de nota

- Procedimiento: pasos numerados.
- Runbook: síntoma, diagnóstico, acción, verificación.
- Incidente: qué pasó, impacto, causa, resolución, lecciones.
- Decisión: problema, opciones evaluadas, decisión, consecuencias.
- Concepto: definición, ejemplos, relaciones.

## Relacionado con

- [[Nota existente 1]]
- [[Nota existente 2]]

## Backlinks

<!-- Se actualizan manualmente o con búsqueda en Obsidian -->

## Preguntas abiertas

- [ ] Pregunta pendiente 1
- [ ] Pregunta pendiente 2

## Referencias

- URL interna
- Documento de Drive
- Imagen adjunta
```

### Reglas del frontmatter

- `id`: fecha + número secuencial del día, por ejemplo `20250731-001`.
- `type`: uno de `procedimiento`, `runbook`, `incidente`, `postmortem`, `decision`, `concepto`, `configuracion`, `script`, `checklist`, `auditoria`, `control`.
- `area`: nombre del área tal como aparece en la carpeta.
- `tags`: lowercase, sin espacios, en español.
- `pedagogico`: `true` solo si la nota debe servir para generar material didáctico en NotebookLM.

**Regla obligatoria para `autor`:**

- El campo `autor` debe ser la inicial del **autor real de la nota**, nunca un valor heredado de una plantilla, de otra nota copiada o del ejemplo del template.
- Antes de escribir la nota, determinar el autor de una de estas formas (en orden):
  1. Si el usuario lo indica explícitamente, usar esa inicial.
  2. Inferir de la identidad de git: `git config user.name` → inicial del primer nombre y apellido.
  3. Si no se puede inferir, **preguntar al usuario** antes de generar la nota.
- Nunca dejar `autor: <inicial o nombre>` literal, ni copiar la inicial de otra nota existente.
- Al crear una nota a partir de una copia o template, verificar SIEMPRE que `autor`, `id` y `creado` no hayan quedado con valores de la nota origen.

### Reglas del frontmatter para auditoría

- `normativa`: `ISO-27001` o `PCI-DSS`.
- `anio`: año de la auditoría (solo notas `auditoria`).
- `estado` (control): `abierto` | `en_progreso` | `cerrado`.
- `estado` (auditoria): `en_progreso` | `cerrado`.
- `severidad`: `critico` | `alto` | `medio` | `bajo` (solo control).
- `control_id`: identificador de la normativa.
- `responsable`: inicial o nombre del técnico responsable del control.

### Regla de historial por año (controles)

Cada nota `control` tiene una sección `## Historial por año` con una subsección por año auditado:

```markdown
## Historial por año

### 2025
- **Documento:** [link a Google Drive](https://drive.google.com/...)
- **Resumen:** Una o dos líneas de qué trata este punto en este año.
- **Estado:** cerrado
- **Ticket ClickUp:** [link](https://app.clickup.com/...)
- **Notas:** contexto adicional

### 2024
- **Documento:** ...
```

### Reglas para enlaces internos

- Usar sintaxis `[[Nombre de la nota]]`.
- El nombre dentro de `[[...]]` debe coincidir con el nombre de archivo `.md` sin extensión.
- Buscar notas existentes antes de crear enlaces. Si no existe una nota relacionada, no inventar el enlace.
- Para imágenes usar `![descripción](/.attachments/nombre.png)`.

### Reglas para imágenes

- Guardar las imágenes en `.attachments/` (raíz del repo).
- Referenciarlas con ruta absoluta desde la raíz de la wiki.
- Si el usuario no proporciona imagen, omitir la sección o dejar placeholder comentado.

### Reglas para bloques de código

- Usar fences con lenguaje: `bash`, `powershell`, `cmd`, `sql`, `yaml`, `json`, `python`.
- Si el bloque contiene IPs privadas, URLs internas o credenciales de ejemplo, agregar advertencia.

### Reglas para advertencias

```markdown
> ⚠️ **Importante**: este procedimiento afecta servicios productivos. Validar en staging primero.
```

### Reglas para TODOs

- Usar listas de tareas de Markdown: `- [ ]`.
- Solo incluir si hay acciones pendientes reales.

## Ontología de tipos de nota

| Tipo | Cuándo usar | Secciones clave |
|---|---|---|
| `concepto` | Definir una tecnología, patrón o componente. | Definición, ejemplos, relaciones. |
| `procedimiento` | Pasos para realizar una tarea. | Objetivo, requisitos, pasos, validación. |
| `runbook` | Resolver un problema conocido. | Síntoma, diagnóstico, acción, verificación. |
| `incidente` | Registrar un evento puntual. | Qué pasó, impacto, causa, resolución, lecciones. |
| `postmortem` | Análisis formal post-incidente. | Resumen, timeline, causas raíz, acciones correctivas. |
| `decision` | Documentar una decisión de arquitectura. | Problema, opciones, decisión, consecuencias. |
| `configuracion` | Registrar configuración de un servicio. | Servicio, archivo, valor, justificación. |
| `script` | Documentar un script o comando reutilizable. | Propósito, uso, parámetros, ejemplos. |
| `checklist` | Lista de verificación para una tarea. | Contexto, items, criterio de completitud. |
| `auditoria` | Índice de una auditoría anual por normativa. | Alcance, controles auditados, tickets abiertos, estado general. |
| `control` | Tópico/ticket de auditoría con historial por año. | Contexto, historial por año, documento, resumen, estado. |

## Integración con NotebookLM

- Las notas marcadas con `pedagogico: true` pueden exportarse a un formato consumible por NotebookLM.
- NotebookLM no lee Markdown directamente desde Git. El flujo es seleccionar notas, exportarlas a Google Docs o PDF y subirlas como fuentes.
- La skill puede generar una versión resumida/didáctica de una nota técnica si el usuario lo solicita.

## Uso del MCP Gemini Notebook

El MCP `gemini-notebook-mcp` es una herramienta auxiliar. La fuente de verdad de la wiki sigue siendo Git y, para auditorías, los documentos aprobados siguen estando en Google Drive.

### Precondiciones

- Verificar que el servidor `gemini-notebook-mcp` esté habilitado en OpenCode.
- Verificar la autenticación con `nlm login --check`.
- Si está deshabilitado o no autenticado, continuar con la documentación local y pedir al usuario que lo habilite/autentique; no modificar configuración ni iniciar sesión automáticamente.
- No solicitar, copiar ni exponer cookies, tokens o archivos de sesión.

### Herramientas principales

- `notebook_list`, `notebook_get`: localizar y verificar notebooks.
- `notebook_query`: consultar contenido y pedir resúmenes comparativos.
- `source_add`: agregar una fuente únicamente después de confirmación explícita.
- `source_list_drive`, `source_sync_drive`: revisar fuentes; no sincronizar sin confirmar.
- `source_describe`, `source_get_content`: entender evidencia sin duplicarla.
- `chat_export`: exportar solo cuando el usuario lo solicite y confirme el destino.
- `server_info`: verificar la versión del servidor.

### Flujo para auditorías

1. Identificar normativa, año, tópico/control y notebook autorizado.
2. Consultar notebooks con `notebook_list` y `notebook_query`.
3. Comparar con Google Drive y la nota DIKS; tratar el resultado del modelo como referencia, no evidencia.
4. Proponer la nota `control` con resumen, estado y links a Drive y ClickUp.
5. Esperar confirmación antes de usar `source_add`, crear artefactos o modificar fuentes.
6. Guardar únicamente el resumen trazable y enlaces autorizados.

### Restricciones de seguridad

- No usar herramientas para publicar, invitar, borrar notebooks, fuentes, artefactos o notas sin solicitud explícita y validada.
- No subir credenciales, tokens, cookies, claves privadas, capturas sin revisar ni documentos fuera del alcance.
- No enviar documentos de auditoría a un notebook no autorizado.
- Mantener el MCP deshabilitado cuando no se use.

### Exposición selectiva de herramientas

Para auditorías se recomienda ocultar operaciones mutantes mediante `NOTEBOOKLM_DISABLED_GROUPS`, `NOTEBOOKLM_DISABLED_TOOLS` y `NOTEBOOKLM_ENABLED_TOOLS`. Cualquier cambio requiere reiniciar el servidor MCP.

## Ejemplos de invocación

Usuario: "Documentá que hoy configuré NAT en pfSense para la VPN de AT&T."

1. Detectar tipo y área.
2. Buscar notas relacionadas existentes.
3. Generar `Infra/Firewall/NAT-pfSense-VPN-AT&T.md`.
4. Presentar la nota para validación.

Usuario: "Documentá el ticket de ISO de la Directiva de aplicación de estándar de seguridad, cerrado en 2025."

1. Detectar tipo `control` y área `Auditorias` → `ISO-27001`.
2. Buscar si la nota existe; si existe, agregar el año al historial.
3. Si no existe, crearla y enlazarla desde `[[ISO-27001-2025]]`.
4. Presentar la nota para validación.

> ⚠️ **Importante**: si la nota de control ya existe, NO crear una duplicada. Agregar la subsección `### <Año>` dentro de `## Historial por año`.

## Ruta del repo de documentación

- **Uso desde el repo:** si el agente se ejecuta dentro de `Infra/wiki`, escribir en `Infra/` relativo al `cwd`.
- **Uso desde otro proyecto:** obtener la ruta de `DIKS_WIKI_PATH`. Si no está definida, preguntar la ruta absoluta antes de escribir.
- **IMPORTANTE:** no agregar claves custom como `diks.wikiPath` en `opencode.json`; OpenCode rechaza claves desconocidas.

## Restricciones de seguridad

- No incluir contraseñas reales, tokens ni claves privadas.
- Usar placeholders: `<PASSWORD>`, `<API_KEY>`, `<TOKEN>`.
- Revisar que capturas de pantalla no expongan credenciales.
- Mantener URLs de administración con su dominio exacto; no modificarlas.

## Formato de salida

Cuando el agente genere una nota, debe:

1. Mostrar la ruta propuesta.
2. Mostrar el contenido completo en Markdown.
3. Listar los enlaces internos creados y si las notas destino existen.
4. Esperar confirmación antes de escribir en disco.

## Convenciones de idioma

- Todo el contenido de la nota debe estar en español.
- Los tags y slugs técnicos pueden estar en inglés cuando sean nombres propios.
- Los nombres de páginas, secciones y explicaciones deben mantenerse en español.
