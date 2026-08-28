# NotebookLM y el MCP Gemini Notebook

Referencia de `diks`. Leer solo si la tarea toca NotebookLM.

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

