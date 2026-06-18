---
name: yz-ui
description: Yoizen UI design system (Dark Glass theme). Trigger: Yoizen UI components, styling, colors, typography, light/dark theme, tables, modals, dropdowns.
---

## When to Use

Use this skill when:
- Creating new UI components for Yoizen products (any framework: Angular, React, plain HTML)
- Implementing consistent styling across the application
- Choosing colors, fonts, or spacing
- Working with icons and brand assets
- Ensuring design system compliance

This file is the **core doctrine** (always read it). Deep, field-tested lessons
live in `references/` and are loaded on demand — see "Deep references" below;
pull the relevant one when the task touches it.

## The One Rule

**Tokens live ONLY in `palette.css`. Components consume `var(--*)` — never raw hex, never raw rgba triplets, never raw rem/ms/z-index.**

Every color, radius, spacing value, shadow, gradient, font-size, duration and
stacking level is a CSS custom property. If you find yourself typing a literal
inside a component, stop and either use an existing token or add a new one to the
palette. This is both a consistency rule and a maintainability rule: a theme swap
(e.g. a light variant) must touch one file only. A third-party brand's exact
colors (e.g. an OAuth button) are the only sanctioned literal exception.

**Two traps that silently break theming — the palette already solves both:**

1. **Translucent brand/semantic colors.** You can't apply alpha to a hex var (`rgba(var(--accent), .2)` is invalid). So every color that needs a faded variant is *also* published as an RGB-channel token: `--yz-primary-1-rgb: 26, 102, 255`. Use `rgba(var(--yz-primary-1-rgb), 0.18)` — never inline the `26,102,255`. This is the #1 source of un-themeable hardcoding.
2. **Text tints (the bright pastels).** On dark glass, label/value text uses the light pastel of each hue. These are **tokens** (`--tint-success`, `--tint-danger`, `--tint-info`, `--tint-accent`, `--tint-purple`, `--tint-primary`), not inline hex like `#6ee7b7`. The saturated base colors (`--success` etc.) are for dots/icons only.

Glass surface darks are likewise tokenized as channels (`--surf-1-rgb` … `--surf-12-rgb`) so a light theme can flip them centrally.

## Quick Start (any project)

Copy the theme bundle from this skill into the project and import it once:

```
assets/theme/
├── index.css       ← entry point: @import of all the rest + layout helpers
├── palette.css     ← design tokens (THE source of truth, incl. light theme block)
├── base.css        ← reset, ambient background, typography, scrollbar, focus ring, glow alphas
├── buttons.css     ← .btn variants (pill buttons with lift + glow)
├── forms.css       ← inputs, selects, date fields, search, toggles, tabs
├── table.css       ← .data-table dark glass tables + reusable cell/column primitives
├── modal.css       ← .modal glass modals (+ .modal-popovers) + action popups
├── components.css  ← pills, tags, KPI cards, page headers, alerts, toasts, tooltips, .yd-* dropdowns
└── shell.css       ← app shell (sidebar + topbar + content), login split-screen
```

- **Angular**: copy to `src/styles/` (and the shared CSS to `src/app/shared/styles/`) and reference `index.css` in `angular.json` `"styles"`.
- **React/Vite**: `import './styles/index.css'` in `main.tsx`.
- **Tailwind projects**: keep the palette as CSS vars and map them in the Tailwind theme (see "Tailwind Mapping"). Tailwind utilities then resolve to tokens, not hex.

The classes are framework-agnostic (`class=` or `className=` both work).

For the theme toggle + before-first-paint bootstrap + variable fonts, see
`references/theming.md` and copy `assets/angular/theme.service.ts`.

## Design Tokens (palette.css)

Read `assets/theme/palette.css` for the full list. The key groups:

| Group | Tokens | Notes |
|-------|--------|-------|
| Brand | `--yz-primary-1` #1a66ff, `--yz-primary-2` #4a3abf, `--yz-accent` #fd6421, `--yz-yellow` #fdbd27, `--yz-dark` #272a35, `--yz-navy` #00183f | Exact institutional hexes |
| Brand/semantic RGB channels | `--yz-primary-1-rgb`, `--yz-accent-rgb`, `--info-rgb`, `--success-rgb`, `--danger-rgb`, `--slate-rgb`, `--white-rgb`, `--black-rgb` … | For `rgba(var(--x-rgb), a)` — the only correct way to fade a token |
| Text tints | `--tint-primary`, `--tint-purple`, `--tint-accent`, `--tint-success`, `--tint-danger`, `--tint-info`, `--tint-yellow` (+ `-2`…`-5` blue steps) | Bright pastels for **text** on glass; use the token, not the hex |
| Surfaces | `--surface`, `--surface-strong`, `--surface-soft`, `--surface-hover`, `--panel-border`, `--panel-border-strong`, `--table-head-bg`, `--input-bg` | Translucent rgba — glass effect needs alpha |
| Glass surface channels | `--surf-1-rgb` … `--surf-12-rgb` (+ `--surf-success/danger/flat/input-rgb`) | Dark translucent stack behind cards/popovers/footers; flip these for light |
| Text | `--text` #f3f5fb, `--text-soft` #c4cbdd, `--text-muted` #8b93ab, `--text-faint` | 4-level hierarchy; never use pure #fff/#808080 |
| Semantic | `--success`, `--danger`, `--warning`, `--info` + each with `-soft` (bg) and `-border` variants | Always use the trio: bg=soft, border=border, text=tint token |
| Buttons | `--btn-primary-bg` (deep indigo), `--btn-danger-bg` (deep red) | Sober gradients — NOT the raw neon brand gradient |
| Radii | `--radius-sm` .55rem → `--radius-xl` 1.6rem | Generous rounding; buttons are full pills (999px) |
| Spacing | `--space-1` (.25rem) → `--space-10` (2.75rem) | cards `--space-6`, page sections `--space-6`, KPI grid gap `--space-4` |
| Shadows | `--shadow-glass`, `--shadow-lift`, `--focus-ring` | |
| Gradients | `--grad-brand`, `--grad-accent`, `--grad-full`, `--grad-text` | See contrast rules |
| Fonts | `--font-sans` (Inter), `--font-mono` (JetBrains Mono) | |
| Type scale | `--text-2xs` .6875 → `--text-3xl` 2rem (9 steps) | Every `font-size` consumes a step — never raw rem |
| Motion | `--t-fast` 120 / `--t-base` 150 / `--t-slow` 260ms, `--ease-out` | One set of durations |
| Z-index | `--z-base/sticky/drawer/fab/overlay/popover/dropdown/toast` | Stacking as a scale, not magic numbers |
| Misc | `--disabled-opacity` 0.5 | |

## Contrast Rules (learned the hard way)

1. **Gradient text on dark surfaces**: the institutional gradient (`--grad-full`) is **too dark** for text on dark backgrounds — the purple midpoint becomes illegible. Use the high-luminance `--grad-text` plus the `.grad-text` helper (it also adds a faint glow to lift the wordmark). Use this for the app name / brand title.
2. **Semantic text colors**: on dark glass, use the *bright pastel* of each hue via its token (`var(--tint-success)` …) — never the raw hex. The saturated base colors (`--success` etc.) are for dots/icons only.
3. **Text hierarchy**: titles `--text`, body/cells `--text-soft`, labels/subtitles `--text-muted`, placeholders/help `--text-faint`. Don't skip levels — that's what makes the UI feel flat or muddy.
4. **No pure black/white**: backgrounds derive from `--yz-dark` with an ambient radial glow stack; text tops out at #f3f5fb.
5. `html { color-scheme: dark; }` so native controls render dark (flips to `light` in the light scope).
6. **Active/selected state text — the white-on-translucent trap**: an active nav item/tab over a *translucent* tinted background looks crisp on dark but **vanishes on light** (white text on a pale tint). Drive it from `--nav-active-text` (white on dark, dark-hue on light). The exception: when the active background is a **solid** brand gradient (`--grad-brand`), white text is correct in both themes.

## Signature Visual Language

- **Glass panels**: `.glass` / `.card` = translucent gradient background + `backdrop-filter: blur(18-22px)` + 1px `--panel-border` + `--shadow-glass`. Everything floats over the ambient background.
- **Ambient background**: the `body::before` carries a fixed 3-layer radial glow (blue / orange / purple) over `--yz-dark`. Never a flat color.
- **Pill buttons**: full-radius (999px), `translateY(-1px)` lift on hover, inner white-ish border. Variants: `.btn-primary`, `.btn-accent`, `.btn-ghost`, `.btn-outline`, `.btn-danger`, `.btn-danger-solid`, sizes `.btn-sm`/`.btn-xs`, circular `.btn-icon`.
  - **The primary button is NOT the raw brand gradient** (reads neon). Use `--btn-primary-bg` (deep indigo, same both themes) with a *contained* `--shadow-lift` glow. Keep `--grad-brand` for *decorative* fills (progress bars, selected calendar day).
  - **The solid danger button is sober too**: `.btn-danger-solid` uses `--btn-danger-bg` (deep red), not the neon `--danger-strong`.
- **Status pills**: `.pill .pill-{success|info|primary|accent|danger|warning|muted}` with a status `.dot` (add `.pill-running` for pulsing dot).
- **KPI cards**: `.kpi-grid` (4→2→1 responsive) with `.kpi` cards; prefer the compact horizontal `.kpi-compact`. Each card gets a colour identity from a single `--kpi-rgb` triple — tinted border + corner wash + a gradient/glow icon chip.
- **Page headers**: `.page-header` with `.page-eyebrow` (orange uppercase micro-label) + `.page-title` (clamp sizing) + `.page-subtitle`. Sections use `.section-head` with the glowing `.section-tick`.
- **Tables**: `.data-table` with sticky blurred header, uppercase micro-label `th`, row hover, `.row-actions` on hover/focus-within, `.table-foot` pagination. Reusable primitives in `table.css`: `.col-hide-lg/md`, `.cell-trunc`, `.cell-stack`, `.cell-sub`, `.col-fit`, `.id-trunc`.
- **Dropdowns & calendar**: native `<select>`/`<input type=date>` can't match the theme. Use the themed `.yd-select` / `.yd-date` controls with a `.yd-pop` glass popover (Angular references in `assets/angular/`, positioned by the `ydAnchored` directive). **See `references/forms-modals.md` for the modal-popovers / flip-clamp / propagation traps.**
- **Modals**: `.overlay` (blurred fixed-dark scrim) + `.modal` (flex column: head/foot pinned, body scrolls) with `modalIn` animation; `.form-grid` 2-col forms. Add `.modal-popovers` when the body holds a themed dropdown.
- **Toasts / alerts / skeletons / empty states**: `.toast`, `.alert-{success|warning|danger|info}`, `.skeleton`/`.skel-line`, `.empty-state`, `.mini-empty`.
- **Motion**: 120–160ms ease; entrance animations use `cubic-bezier(0.16, 1, 0.3, 1)`; always honor `prefers-reduced-motion` (handled in `base.css`).
- **A11y**: global `:focus-visible` ring in `base.css`; custom scrollbar; WCAG 4.5:1 minimum for text; icon-only buttons need `aria-label`.

## Layout Helpers (index.css)

`.stack` (column, gap-4), `.row` (+`.wrap`), `.grid-2`, `.grid-equal` (cards stretch evenly), `.spacer`, `.muted`, `.soft`, `.mono`, `.tnum` (tabular numbers for metrics).

App shell: `.app-shell` grid (sidebar `--sidebar-w` 264px / collapsed 78px) + sticky `.sidebar` + `.topbar` + `.content` (clamp() horizontal padding, `--space-8`/`--space-10` vertical rhythm). Mobile (≤920px): sidebar becomes a fixed drawer with `.scrim` + `.mobile-fab`. Brand block uses `.brand-name.grad-text`.

## Code Examples

```html
<!-- Page header -->
<header class="page-header">
  <div class="page-heading">
    <span class="page-eyebrow">Sección</span>
    <h1 class="page-title">Panel principal</h1>
    <p class="page-subtitle">Resumen del estado general</p>
  </div>
  <div class="page-actions">
    <button class="btn btn-ghost">Exportar</button>
    <button class="btn btn-primary">Nuevo</button>
  </div>
</header>

<!-- KPI card (compact): one --kpi-rgb triple drives border+wash+icon chip -->
<div class="kpi kpi-compact" style="--kpi-rgb: var(--success-rgb); --kpi-icon-color: var(--tint-success);">
  <div class="kpi-icon"><svg>…</svg></div>
  <div class="kpi-meta">
    <div class="kpi-value tnum">248</div>
    <div class="kpi-label">Registros activos</div>
  </div>
</div>

<!-- Status pill -->
<span class="pill pill-success"><span class="dot"></span> Activo</span>

<!-- Field -->
<label class="field">
  <span class="field-label">Nombre</span>
  <input class="input" placeholder="Buscar…" />
  <span class="field-help">Identificador interno</span>
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
        primary: 'var(--yz-primary-1)', secondary: 'var(--yz-primary-2)',
        accent: 'var(--yz-accent)', surface: 'var(--surface)', muted: 'var(--text-muted)',
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
- Consume `var(--*)` tokens for every color, radius, spacing, shadow, font-size, duration and z-index
- Use `.grad-text` for brand titles on dark surfaces
- Use the semantic trio (`-soft` bg + `-border` + bright pastel text) for states
- Apply spacing rhythm with `--space-*` (sections gap-6, cards pad-6, grids gap-4/5)
- Keep `prefers-reduced-motion` and `:focus-visible` support intact
- Use `.tnum`/`.cell-mono` for numeric/metric columns
- Mirror every icon-only button's `data-tip` into an `aria-label`

### DON'T:
- Hardcode hex/rgba/rem/ms/z-index in components — add or consume a token instead
- Use `--grad-full` for text (illegible on dark) — that's `--grad-text`'s job
- Use saturated semantic base colors for text (they're for dots/icons)
- Use `--yz-primary-1` as active-state text on dark — it sinks in; use `--nav-active-text` (never a literal `#fff`, which vanishes on the pale active tint in light)
- Forget `.modal-popovers` on any modal with a `yd-select`/`yd-date`, or `stopPropagation` on the modal panel — see `references/forms-modals.md`
- Use native `<select>`/`<input type=date>` unthemed — use the `.yd-*` pattern (`grep -rn '<select' src/` must be zero)
- Ship native `title` tooltips — every hover text uses `data-tip` (+ `aria-label`)
- Use pure black or flat backgrounds — ambient glow over `--yz-dark`
- Skip text-hierarchy levels (title→soft→muted→faint)

## Deep references (load on demand)

Pull the matching file into context when the task touches it — they hold the
full field-tested detail that doesn't need to be in working memory all the time:

- **`references/theming.md`** — light-theme variant (the inversions that matter), theme-toggle bootstrap (pre-paint, View-Transitions reveal), variable fonts. Read before building the light theme or the toggle.
- **`references/tables.md`** — tables, lists, dashboards, server-side pagination, grouped catalogs, deep-link filtering, pill→variant mapping. Read before building any data view.
- **`references/forms-modals.md`** — `data-tip` tooltips, themed `yd-select`/`yd-date` inside modals (the modal-popovers / flip-clamp / propagation traps), destructive/confirmation dialogs. Read before adding tooltips, dropdowns-in-modals or a delete dialog.
- **`references/performance.md`** — zoneless/OnPush/signals, lazy routes, skeletons, a11y, paint cost of `backdrop-filter`. Read when scaffolding a project or doing a perf pass.
- **`references/docs.md`** — index of the theme bundle, Angular references and brand assets.

## Resources

- **Theme bundle (copy-paste ready)**: `assets/theme/`
- **Themed select/date + positioning + theme toggle (Angular)**: `assets/angular/` (`yd-select`, `yd-date`, `yd-anchored` directive, `popover.service`, `theme.service`)
- **React component template (token-consuming)**: `assets/component-template.tsx`
- **Brand assets**: `assets/*.svg`
