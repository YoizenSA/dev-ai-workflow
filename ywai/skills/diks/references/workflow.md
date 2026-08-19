# Flujo de trabajo y ejemplos de invocación

Referencia de `diks`. El procedimiento completo para convertir información
técnica en una nota atómica.

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

