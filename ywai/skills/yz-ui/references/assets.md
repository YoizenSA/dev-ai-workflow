# Assets — qué hay y para qué

### Brand files (official Yoizen brand kit — color decisions trace back to the palette)

| File(s) | Usage |
|------|-------|
| `logo.svg` / `logo-negativo.svg` / `logo-blanco.svg` / `logo-negro.svg` | Wordmark — light bg / dark bg / all-white / all-black |
| `logo-secundario*.svg` | Compact/square wordmark (app shells) |
| `logo-slogan*.svg` | Logo + slogan (landing/marketing) |
| `icon.svg` / `icon-blanco.svg` / `icon-negro.svg` | Isotipo — favicon, avatar, collapsed-rail brand |
| `paleta-institucional.png` / `paleta-degrade.png` | The institutional palette + gradients |

```html
<img src="/assets/logo.svg" alt="Yoizen" class="h-8 w-auto" />          <!-- header -->
<img src="/assets/logo-negativo.svg" alt="Yoizen" class="h-7 w-auto" /> <!-- dark sidebar -->
<img src="/assets/icon.svg" alt="Yoizen" class="h-9 w-9" />             <!-- compact / favicon -->
```

### CSS bundle (copy `palette.css`/`base.css` verbatim; adapt the rest)

| File | Contents |
|------|----------|
| `palette.css` | Design tokens — dark `:root` + `:root[data-theme="light"]` |
| `base.css` | Reset, ambient background, scrollbar, focus ring, `.glass`, theme reveal |
| `buttons.css` | `.btn` + variants (primary/ghost/danger/accent/outline, sizes) |
| `forms.css` | Inputs, `.field`/`.field-help`, textarea, `yd-select`, `yd-cal` |
| `table.css` | `.data-table` + `col-hide-*` responsive + skeleton rows |
| `modal.css` | `.overlay`/`.modal` glass dialog (+ `.modal-foot.split`) |
| `components.css` | Pills, tags, cards, page/section headers, KPI, alerts, spinner, skeleton, empty states, toasts, **tooltips**, code-chip, diff box + **side-by-side diff** (`.diff-split`), key-value rows |
| `shell.css` | **Reference layout** (swap/reshape per app) — sidebar/drawer, topbar, login, responsive |

### Behavioral primitives (TypeScript — wire to the project's components)

| File | Role |
|------|------|
| `yz-modal.directive.ts` | Accessible dialog: `role`/`aria-modal`, focus-trap, scroll-lock, Escape |
| `yd-anchored.directive.ts` | Popover positioning: flip/clamp vs viewport or modal (`ydConfineToModal`) |
| `popover.service.ts` | Single-open coordinator: opening one docked popover (`yd-select`/`yd-date`) closes the other |
| `yd-select.component.ts` | Themed select with search / tags |
| `yd-date.component.ts` | Themed calendar — dropdown-caption header (month/year as themed dropdowns, never native), keyboard nav |
| `yd-diff.component.ts` + `diff.ts` | Side-by-side diff (antes \| después) GitHub-style; `diff.ts` = `unifiedToRows` / `diffTexts` (LCS) |
| `toast.service.ts` + `toast-stack.component.ts` | Toast system (CSS in `components.css`) |
| `theme.service.ts` | Dark⇄light **circular reveal** (View Transitions): brand-glow edge + icon morph (CSS in `base.css`) |
| `component-template.ts` | Angular standalone starting point (signals, OnPush) |

## Contenido de `css-snippets.css`

- Gradientes de marca (fondo y texto) y grid pattern técnico
- Utilidades de color con CSS variables
- Componentes comunes: card, sidebar, input, botón primario
- Alertas semánticas (info/success/warning/error)
- Scrollbar personalizado (Webkit + Firefox)
- Animaciones: fade-in, pulse sutil
- Utilidades de layout: container, grid responsive, flex, truncate
- **Glass panel** (tema oscuro)
- **Modal / overlay** (glass dialog: scrim, card head/body/foot, animaciones — a11y en `yz-modal.directive.ts`)
- **Sidebar rail** (opcional: footer tools colapsados como icon-buttons contenidos, un paso por debajo del avatar)
- **Icon button** (botón circular solo-icono)
- **Spinner / loading inline**
- **Pill badges semánticos**
- **Sistema de toasts completo** (stack, animaciones enter/exit/progress, variantes semánticas, responsive, `prefers-reduced-motion`)
