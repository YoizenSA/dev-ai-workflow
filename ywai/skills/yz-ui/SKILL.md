---
name: yz-ui
description: Yoizen UI design system (Dark Glass theme). Trigger: Yoizen UI components, styling, colors, typography.
license: Apache-2.0
---

## When to Use

Use this skill when:
- Creating new UI components for Yoizen products (any framework: Angular, React, plain HTML)
- Implementing consistent styling across the application
- Choosing colors, fonts, or spacing
- Working with icons and brand assets
- Ensuring design system compliance

## The One Rule

**Tokens live ONLY in `palette.css`. Components consume `var(--*)` — never raw hex.**

Every color, radius, spacing value, shadow and gradient is a CSS custom property. If you find yourself typing a hex code inside a component, stop and either use an existing token or add a new one to the palette. This is both a consistency rule and a performance/maintainability rule (theme changes touch one file).

## Quick Start (any project)

Copy the theme bundle from this skill into the project and import it once:

```
assets/theme/
├── index.css       ← entry point: @import of all the rest + layout helpers
├── palette.css     ← design tokens (THE source of truth)
├── base.css        ← reset, ambient background, typography, scrollbar, focus ring
├── buttons.css     ← .btn variants (pill buttons with lift + glow)
├── forms.css       ← inputs, selects, date fields, search, toggles, tabs
├── table.css       ← .data-table dark glass tables
├── modal.css       ← .modal glass modals + action popups
├── components.css  ← pills, tags, KPI cards, page headers, alerts, toasts, calendar/select dropdown
└── shell.css       ← app shell (sidebar + topbar + content), login split-screen
```

- **Angular**: copy to `src/styles/` and reference `index.css` in `angular.json` `"styles"`.
- **React/Vite**: `import './styles/index.css'` in `main.tsx`.
- **Tailwind projects**: keep the palette as CSS vars and map them in the Tailwind theme (see "Tailwind Mapping" below). Tailwind utilities then resolve to tokens, not hex.

The classes are framework-agnostic (`class=` or `className=` both work).

## Design Tokens (palette.css)

Read `assets/theme/palette.css` for the full list. The key groups:

| Group | Tokens | Notes |
|-------|--------|-------|
| Brand | `--yz-primary-1` #1a66ff, `--yz-primary-2` #4a3abf, `--yz-accent` #fd6421, `--yz-yellow` #fdbd27, `--yz-dark` #272a35, `--yz-navy` #00183f | Exact institutional hexes |
| Surfaces | `--surface`, `--surface-strong`, `--surface-soft`, `--surface-hover`, `--panel-border`, `--panel-border-strong` | Translucent rgba — glass effect needs alpha |
| Text | `--text` #f3f5fb, `--text-soft` #c4cbdd, `--text-muted` #8b93ab, `--text-faint` #5f6781 | 4-level hierarchy; never use pure #fff/#808080 |
| Semantic | `--success`, `--danger`, `--warning`, `--info` + each with `-soft` (bg) and `-border` variants | Always use the trio together: bg=soft, border=border, text=bright pastel |
| Radii | `--radius-sm` .55rem → `--radius-xl` 1.6rem | Generous rounding; buttons are full pills (999px) |
| Spacing | `--space-1` (.25rem) → `--space-10` (2.75rem) | Use for gaps/padding between dashboard sections: cards `--space-6`, page sections `--space-6`, KPI grid gap `--space-4` |
| Shadows | `--shadow-glass`, `--shadow-lift`, `--focus-ring` | |
| Gradients | `--grad-brand`, `--grad-accent`, `--grad-full`, `--grad-text` | See contrast rules |
| Fonts | `--font-sans` (Inter), `--font-mono` (JetBrains Mono) | |

## Contrast Rules (learned the hard way)

1. **Gradient text on dark surfaces**: the institutional gradient (`--grad-full`, #1a66ff→#4a3abf→#fd6421) is **too dark** for text on dark backgrounds — the purple midpoint becomes illegible. Use the high-luminance variant `--grad-text` (#6ea3ff→#b3a6ff→#ff9e6b) plus the `.grad-text` helper class, which also adds a faint `drop-shadow` glow to lift the wordmark off the sidebar. Use this for the app name / brand title.
2. **Semantic text colors**: on dark glass, use the *bright pastel* of each hue for text (`#6ee7b7` success, `#fca5a5` danger, `#93c5fd` info, `#fdba74` accent, `#c4b5fd` purple) — the saturated base colors (`#34d399` etc.) are for dots/icons only.
3. **Text hierarchy**: titles `--text`, body/cells `--text-soft`, labels/subtitles `--text-muted`, placeholders/help `--text-faint`. Don't skip levels — that's what makes the UI feel flat or muddy.
4. **No pure black/white**: backgrounds derive from `--yz-dark` #272a35 with an ambient radial glow stack (see `base.css` body); text tops out at #f3f5fb.
5. `html { color-scheme: dark; }` so native controls render dark.

## Signature Visual Language

- **Glass panels**: `.glass` / `.card` = translucent gradient background + `backdrop-filter: blur(18-22px)` + 1px `--panel-border` + `--shadow-glass`. Everything floats over the ambient background.
- **Ambient background**: the `body` carries a fixed 3-layer radial glow (blue / orange / purple) over `--yz-dark`. Never a flat color.
- **Pill buttons**: full-radius (999px), gradient fills, `translateY(-1px)` lift + glow on hover, inner 1px white-ish border. Variants: `.btn-primary`, `.btn-accent`, `.btn-ghost`, `.btn-outline`, `.btn-danger`, `.btn-danger-solid`, sizes `.btn-sm`/`.btn-xs`, circular `.btn-icon`.
- **Status pills**: `.pill .pill-{success|info|primary|accent|danger|warning|muted}` with a status `.dot` (add `.pill-running` for pulsing dot).
- **KPI cards**: `.kpi-grid` (4 cols → 2 → 1 responsive) with `.kpi` cards: corner radial glow via `--kpi-glow`, hover lift, big `.kpi-value` with tabular numbers.
- **Page headers**: `.page-header` with `.page-eyebrow` (orange uppercase micro-label) + `.page-title` (clamp sizing) + `.page-subtitle`. Sections use `.section-head` with the glowing `.section-tick` bar.
- **Dropdowns & calendar**: native `<select>`/`<input type=date>` can't match the theme. Use the themed custom controls: `.yd-select` / `.yd-date` triggers with a `.yd-pop` glass popover; calendar grid classes `.yd-cal*` (selected day gets `--grad-brand` + glow). Reference Angular implementations in `assets/angular/yd-select.component.ts` and `yd-date.component.ts` — port the pattern to your framework. As fallback, `forms.css` still themes native controls (`color-scheme: dark`, tinted picker indicator, dark `option`).
- **Tables**: `.data-table` with sticky blurred header, uppercase micro-label `th`, row hover, `.row-actions` revealed on hover/focus-within, `.table-foot` pagination.
- **Modals**: `.overlay` (blurred scrim) + `.modal` with spring-ish `modalIn` animation; `.form-grid` 2-col forms with `.span-2`.
- **Motion**: 120–160ms ease transitions; entrance animations use `cubic-bezier(0.16, 1, 0.3, 1)`; always honor `prefers-reduced-motion` (handled in `base.css`).
- **A11y**: global `:focus-visible` ring (`--focus-ring`) in `base.css`; custom scrollbar; WCAG 4.5:1 minimum for text.

## Layout Helpers (index.css)

`.stack` (column, gap-4), `.row` (+`.wrap`), `.grid-2`, `.grid-equal` (cards stretch evenly), `.spacer`, `.muted`, `.soft`, `.mono`, `.tnum` (tabular numbers for metrics).

App shell: `.app-shell` grid (sidebar `--sidebar-w` 264px / collapsed 78px) + sticky `.sidebar` + `.topbar` + `.content` (uses `clamp()` horizontal padding and `--space-8`/`--space-10` vertical rhythm). Mobile (≤920px): sidebar becomes a fixed drawer with `.scrim` + `.mobile-fab`. Brand block uses `.brand-name.grad-text` for the app title.

## Code Examples

```html
<!-- Page header -->
<header class="page-header">
  <div class="page-heading">
    <span class="page-eyebrow">Operaciones</span>
    <h1 class="page-title">Dashboard</h1>
    <p class="page-subtitle">Estado general de despliegues</p>
  </div>
  <div class="page-actions">
    <button class="btn btn-ghost">Exportar</button>
    <button class="btn btn-primary">Nuevo deploy</button>
  </div>
</header>

<!-- KPI card -->
<div class="kpi" style="--kpi-glow: rgba(52,211,153,0.30); --kpi-icon-bg: rgba(16,185,129,0.16); --kpi-icon-color: #6ee7b7;">
  <div class="kpi-top">
    <div class="kpi-icon"><svg>…</svg></div>
    <span class="kpi-delta up">▲ 12%</span>
  </div>
  <div class="kpi-value tnum">248</div>
  <div class="kpi-label">Deploys exitosos</div>
</div>

<!-- Status pill -->
<span class="pill pill-success"><span class="dot"></span> Activo</span>

<!-- Field -->
<label class="field">
  <span class="field-label">Cliente</span>
  <input class="input" placeholder="Buscar…" />
  <span class="field-help">Nombre o código interno</span>
</label>
```

Custom component CSS (when the bundle doesn't cover it):

```css
.my-widget {
  background: var(--surface-soft);
  border: 1px solid var(--panel-border);
  border-radius: var(--radius-md);
  padding: var(--space-4);
  color: var(--text-soft);
}
.my-widget:hover { background: var(--surface-hover); border-color: var(--panel-border-strong); }
```

## Tailwind Mapping (optional)

Keep `palette.css` loaded and map tokens so utilities resolve to vars:

```javascript
// tailwind.config.js
export default {
  theme: {
    extend: {
      colors: {
        primary: 'var(--yz-primary-1)',
        secondary: 'var(--yz-primary-2)',
        accent: 'var(--yz-accent)',
        surface: 'var(--surface)',
        muted: 'var(--text-muted)',
      },
      borderRadius: { sm: 'var(--radius-sm)', md: 'var(--radius-md)', lg: 'var(--radius-lg)' },
      fontFamily: { sans: 'var(--font-sans)', mono: 'var(--font-mono)' },
    },
  },
};
```

## Brand Assets

See `assets/*.svg` in this skill (logo, variantes negativas/secundarias/con slogan, icon). Copy what the project needs into its `public/`/`assets/` folder.

```html
<img src="/logo.svg" alt="Yoizen" class="h-8 w-auto" />
```

## Best Practices

### DO:
- Consume `var(--*)` tokens for every color, radius, spacing, shadow
- Use `.grad-text` (the bright gradient) for brand titles on dark surfaces
- Use the semantic trio (`-soft` bg + `-border` + bright pastel text) for states
- Apply spacing rhythm with `--space-*` (sections gap-6, cards pad-6, grids gap-4/5)
- Keep `prefers-reduced-motion` and `:focus-visible` support intact
- Use `.tnum`/`.cell-mono` for numeric/metric columns

### DON'T:
- Hardcode hex/rgba in components — add a token instead
- Use `--grad-full` for text (illegible on dark) — that's `--grad-text`'s job
- Use saturated semantic base colors for text (they're for dots/icons)
- Use native select/date dropdowns unthemed — use the `.yd-*` pattern
- Use pure black or flat backgrounds — ambient glow over `--yz-dark`
- Skip text-hierarchy levels (title→soft→muted→faint)

## Resources

- **Theme bundle (copy-paste ready)**: `assets/theme/` in this skill
- **Themed select/date reference (Angular)**: `assets/angular/`
- **Reference implementation**: `/home/umarino/Descargas/yDeploy/ydeploy-angular` (full app using this system)
- **Brand assets**: `assets/*.svg`
