# Referencias · yz-ui (Dark Glass theme)

Índice de recursos del skill. `SKILL.md` es la doctrina central (siempre se
lee); las guías profundas de abajo se cargan **on demand** según la tarea.

## Guías profundas (cargar según la tarea)

| Archivo | Cuándo leerlo |
|---------|---------------|
| `references/theming.md` | Tema claro (las inversiones que importan) + bootstrap del toggle (pre-paint, reveal con View Transitions) + fuentes variables |
| `references/tables.md` | Tablas, listas, dashboards, paginación server-side, catálogos agrupados, filtrado por deep-link, mapeo valor→variante de pill |
| `references/forms-modals.md` | Tooltips `data-tip`, `yd-select`/`yd-date` dentro de modales (trampas modal-popovers / flip-clamp / propagación), modales de confirmación/borrado |
| `references/performance.md` | Zoneless/OnPush/signals, rutas lazy, skeletons, a11y, costo de `backdrop-filter` |

## Theme bundle (dentro de este skill)

Copiable a cualquier proyecto. Tokens en `palette.css`; los componentes consumen `var(--*)` únicamente.

| Archivo | Contenido |
|---------|-----------|
| `assets/theme/index.css` | Entry point (`@import` de todo) + layout helpers (`.stack`, `.row`, `.grid-2`, `.tnum`…) |
| `assets/theme/palette.css` | **Design tokens** — única fuente de verdad de color/spacing/radius/sombras/gradientes + bloque del tema claro |
| `assets/theme/base.css` | Reset, fondo ambiente (3 radial glows + alfas de glow del tema claro), tipografía, scrollbar, focus ring, `.glass`, `.grad-text`, reveal de tema |
| `assets/theme/buttons.css` | `.btn` pill variants con lift + glow |
| `assets/theme/forms.css` | `.field`, `.input/.select/.textarea`, search, segmented toggle, switch, tabs, `.filter-inline`, date nativo tematizado |
| `assets/theme/table.css` | `.data-table` con header sticky blureado, row-actions on hover, paginación + primitivas reutilizables (`.col-hide-*`, `.cell-trunc`, `.cell-stack`, `.cell-sub`, `.col-fit`, `.id-trunc`) |
| `assets/theme/modal.css` | `.overlay` + `.modal` glass (flex column con head/foot fijos), `.modal-popovers`, `.form-grid`, action popup |
| `assets/theme/components.css` | Pills, tags, KPI cards (`.kpi`/`.kpi-compact`), page header, alerts, progress, spinner, skeleton, empty state (`.mini-empty`), toasts, `.del-name`, tooltips `data-tip`, `.yd-select`/`.yd-date` + calendario + buscador |
| `assets/theme/shell.css` | App shell (sidebar colapsable + topbar + content), login split-screen, responsive |

## Componentes de referencia (Angular standalone)

| Archivo | Qué demuestra |
|---------|---------------|
| `assets/angular/yd-select.component.ts` | Dropdown custom tematizado (reemplaza `<select>`), con buscador automático (>7 opciones) y prefijo de label para filtros |
| `assets/angular/yd-date.component.ts` | Date picker custom con calendario `.yd-cal*` |
| `assets/angular/yd-anchored.directive.ts` | Posiciona el popover docked: flip + clamp contra el viewport |
| `assets/angular/popover.service.ts` | Coordina popovers (sólo uno abierto a la vez) — dependencia de `yd-select`/`yd-date` |
| `assets/angular/theme.service.ts` | Toggle dark⇄light con reveal circular (View Transitions API), persistencia y degradación |

## Template de componente (React)

`assets/component-template.tsx` — ejemplo que consume las **clases y tokens** del
sistema (`.card`, `.btn`, `var(--*)`), no utilidades Tailwind ad-hoc.

## Assets visuales

En `assets/` de este skill: `logo.svg`, `logo-sec-slogan.svg`, `logo-negativo.svg`, `logo-negative.svg`, `logo-footer.svg`, `logo-dorso-maneas.svg`, `icon.svg`.
