# Referencias · yz-ui (Dark Glass theme)

## Theme bundle (dentro de este skill)

Copiable a cualquier proyecto. Tokens en `palette.css`; los componentes consumen `var(--*)` únicamente.

| Archivo | Contenido |
|---------|-----------|
| `assets/theme/index.css` | Entry point (`@import` de todo) + layout helpers (`.stack`, `.row`, `.grid-2`, `.tnum`…) |
| `assets/theme/palette.css` | **Design tokens** — única fuente de verdad de color/spacing/radius/sombras/gradientes |
| `assets/theme/base.css` | Reset, fondo ambiente (3 radial glows), tipografía, scrollbar, focus ring, `.glass`, `.grad-text` |
| `assets/theme/buttons.css` | `.btn` pill variants con lift + glow |
| `assets/theme/forms.css` | `.field`, `.input/.select/.textarea`, search, segmented toggle, switch, tabs, date nativo tematizado |
| `assets/theme/table.css` | `.data-table` con header sticky blureado, row-actions on hover, paginación |
| `assets/theme/modal.css` | `.overlay` + `.modal` glass, `.form-grid`, action popup |
| `assets/theme/components.css` | Pills, tags, KPI cards, page header, alerts, progress, empty state, toasts, `.yd-select`/`.yd-date` + calendario |
| `assets/theme/shell.css` | App shell (sidebar colapsable + topbar + content), login split-screen, responsive |

## Componentes de referencia (Angular standalone)

| Componente | Ubicación | Qué demuestra |
|------------|-----------|---------------|
| YdSelect | `assets/angular/yd-select.component.ts` | Dropdown custom tematizado (reemplaza `<select>` nativo) |
| YdDate | `assets/angular/yd-date.component.ts` | Date picker custom con calendario `.yd-cal*` |

## Implementación de referencia completa

`/home/umarino/Descargas/yDeploy/ydeploy-angular` — app Angular completa que usa este sistema (dashboard, tablas, modales, login, settings). Consultar sus `features/` para ver patrones de página completos.

## Assets visuales

En `assets/` de este skill: `logo.svg`, `logo-sec-slogan.svg`, `logo-negativo.svg`, `logo-negative.svg`, `logo-footer.svg`, `logo-dorso-maneas.svg`, `icon.svg`.
