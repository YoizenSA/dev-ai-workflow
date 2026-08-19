# Convenciones — referencias, historial y ontología de tipos

Referencia de `diks`.

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

