---
name: diks
description: "Document infra work as Zettelkasten notes in the Infra repo. Trigger: documentar, runbook, incidente, postmortem."
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

## Cuándo usar esta skill

- El usuario dice: "documentar esto", "crear una nota", "agregar a la wiki".
- El usuario describe un cambio, incidente, procedimiento o decisión técnica.
- El usuario pide convertir logs, outputs de pipeline o conversaciones en documentación.
- El usuario menciona explícitamente DIKS, Zettelkasten, Obsidian o el repo de documentación de Infra.

## Referencias — leer la que corresponda a la tarea

| Tarea | Referencia |
|-------|------------|
| **Documentar algo (el procedimiento completo)** | [references/workflow.md](references/workflow.md) — el flujo paso a paso y ejemplos de invocación |
| **Escribir la nota** | [references/note-template.md](references/note-template.md) — idea atómica, contexto, contenido, enlaces, backlinks, preguntas abiertas |
| **Ubicar, enlazar o clasificar la nota** | [references/conventions.md](references/conventions.md) — referencias, historial por año, ontología de tipos |
| **Commitear o pushear** | [references/git-workflow.md](references/git-workflow.md) — `status` limpio → `fetch` → `pull --rebase` → `push`, y Conventional Commits |
| **Trabajar con NotebookLM** | [references/notebooklm.md](references/notebooklm.md) |

El humano siempre valida la nota antes de `git add / commit / push`.

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
