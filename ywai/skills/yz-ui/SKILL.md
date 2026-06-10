---
name: yz-ui
description: Yoizen UI design system standards. Trigger: Yoizen UI components, styling, colors, typography, visual polish or correction of any Yoizen Angular frontend.
license: Apache-2.0
---

## When to Use

Use this skill when:
- Creating new UI for any Yoizen frontend
- **Correcting visually poor screens** — bringing legacy UIs up to this standard
- Choosing colors, fonts, spacing, or shadows
- Working with icons and brand assets
- Auditing a project for design system compliance

## Scope

These are the **mandatory UI norms for every Yoizen frontend** — existing repos and new ones alike. The skill is self-contained: everything needed (tokens, themes, patterns, assets) is defined here. If a project's current visuals deviate from these norms, the project is wrong, not the norm — correct it using the checklist below. Never imitate a legacy project's existing look.

## Tech Stack Norms

Yoizen frontends are **Angular** (standalone components).

**MANDATORY: latest stable Angular major.** Every project must run the latest stable Angular release. Check with `ng version` (or `package.json`); if the project is behind, upgrading via `ng update @angular/core @angular/cli` is part of the work — do it before (or alongside) UI changes, one major at a time, following the official update guide (`https://angular.dev/update-guide`). New projects always start on the latest major with zoneless change detection, signals (`input()`/`output()`/`computed()`), and native control flow (`@if`/`@for`/`@defer`). Never use `*ngIf`/`*ngFor`, `@Input()`/`@Output()` decorators, or NgModules in new code.

Styling follows one of two approaches — detect which one the project uses before writing styles:

| Approach | Detection | Notes |
|----------|-----------|-------|
| **Pure CSS + design tokens** | `:root` custom properties in a `palette.css`, modular CSS per feature | Default for projects without Tailwind |
| **Tailwind CSS 4 + component classes** | `@import "tailwindcss"` in `styles.css` (CSS-first, no `tailwind.config.js`) | Preferred for new projects |

**NEVER** create a `tailwind.config.js` — Tailwind 4 is configured in CSS via `@theme`/custom properties.

## Brand Palette (mandatory in all projects)

Source of truth: `assets/Colores Institucionales - Yoizen.png`.

| Color | Hex | Usage |
|-------|-----|-------|
| Primary Blue | `#1A66FF` | Buttons, links, primary actions |
| Secondary Purple | `#4A3ABF` | Supportive emphasis, gradients |
| Accent Orange | `#FD6421` | Critical CTAs, urgent attention |
| Yellow | `#FDBD27` | Auxiliary highlights |
| Dark | `#272A35` | Text, dark surfaces |
| Dark Navy | `#00183F` | Deep dark bg alternative |
| Darker Navy | `#00122D` | Deepest dark bg |
| Black | `#000000` | Pure black (use sparingly) |

**Brand Gradients (Gradient 1 & 2 from palette):**
```css
/* Gradient 1 — blue → orange (horizontal, bright) */
background: linear-gradient(90deg, #1A66FF 0%, #4A3ABF 55%, #FD6421 100%);

/* Gradient 2 — dark version (blue/purple → orange/black) */
background: linear-gradient(135deg, #1A66FF 0%, #4A3ABF 50%, #FD6421 80%, #000000 100%);

/* Brand accent (UI buttons, highlights) */
background: linear-gradient(135deg, #1A66FF, #4A3ABF);
```

All theme tokens below derive from this palette. When a project's colors drift from it (off-brand blues, muddy grays), realigning them to these hexes is part of any UI work.

## Two Canonical Themes

Pick per product; both share the brand palette and token structure. These tokens ARE the standard — copy them as-is into new projects.

### Dark glass theme

Dark base with ambient brand glows and glassmorphism panels.

```css
:root {
  --yoizen-primary-1: #1a66ff;
  --yoizen-primary-2: #4a3abf;
  --yoizen-accent: #fd6421;
  --yoizen-dark: #272a35;
  --yoizen-dark-soft: #1d2029;
  --yoizen-light: #ffffff;
}

body {
  font-family: Inter, "Segoe UI", sans-serif;
  color: var(--yoizen-light);
  background:
    radial-gradient(circle at top left, rgba(26, 102, 255, 0.35), transparent 55%),
    radial-gradient(circle at bottom right, rgba(253, 100, 33, 0.4), transparent 55%),
    var(--yoizen-dark);
}

.glass-panel {
  border-radius: 1rem;
  border: 1px solid rgba(148, 163, 184, 0.35);
  background: linear-gradient(135deg, rgba(15, 23, 42, 0.92), rgba(15, 23, 42, 0.82));
  backdrop-filter: blur(22px);
}
```

### Light theme

Soft gradient background with ambient brand tints, white surfaces.

```css
:root {
  --bg-top: #f2f6ff;
  --bg-bottom: #fff6f0;
  --surface: #ffffff;
  --surface-soft: #f5f7ff;
  --surface-hover: #e9efff;
  --text: #1f2433;
  --text-muted: #5c6478;
  --border: #d8e0f0;
  --border-strong: #b6c2dd;

  --primary: #1a66ff;
  --primary-strong: #0e4fd6;
  --secondary: #4a3abf;
  --secondary-light: #6f5ce8;
  --secondary-soft: #eeeaff;
  --accent: #fd6421;
  --accent-strong: #df4f12;

  --danger: #d92a2a;
  --danger-soft: #fdecec;
  --danger-border: #f3b1b1;
  --info-soft: #edf3ff;
  --info-border: #bbd0ff;
  --success-soft: #effaf4;
  --success-border: #a0dfba;
  --warning-soft: #fff7eb;
  --warning-border: #ffd494;

  --radius-sm: 0.5rem;
  --radius-md: 0.75rem;
  --radius-lg: 1rem;
  --space-1: 0.25rem;  --space-2: 0.5rem;  --space-3: 0.75rem;
  --space-4: 1rem;     --space-6: 1.5rem;  --space-8: 2rem;
  --shadow-soft: 0 12px 28px rgba(12, 28, 67, 0.14);
  --focus-ring: 0 0 0 3px rgba(26, 102, 255, 0.28);
  --overlay: rgba(14, 20, 35, 0.5);
}

body {
  font-family: Inter, "Segoe UI", sans-serif;
  line-height: 1.45;
  color: var(--text);
  background:
    radial-gradient(circle at 8% 10%, rgba(26, 102, 255, 0.12), transparent 42%),
    radial-gradient(circle at 92% 4%, rgba(74, 58, 191, 0.12), transparent 38%),
    linear-gradient(180deg, var(--bg-top), var(--bg-bottom));
}
```

## Signature Visual Patterns

These are the patterns that give Yoizen screens their look. Apply them when building or correcting UI.

### 1. Ambient background

Never use flat backgrounds. Layer 2 radial gradients (brand blue + purple/orange at opposite corners, ~0.12 alpha light / ~0.35 alpha dark) over a base gradient or dark color. See both themes above.

### 2. Buttons with lift

All buttons share: `font-weight: 600`, smooth transition (~140–150ms ease), `translateY(-1px)` on hover, reset on `:active`, `cursor: not-allowed; opacity: 0.6–0.65` when disabled.

Light theme (rounded rect + colored glow):

```css
.btn {
  padding: var(--space-2) var(--space-4);
  font: inherit; font-weight: 600;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: all 140ms ease;
}
.btn-primary {
  color: #fff;
  background: var(--primary);
  border-color: var(--primary-strong);
  box-shadow: 0 8px 18px rgba(26, 102, 255, 0.24);
}
.btn-primary:hover:not(:disabled) {
  background: var(--primary-strong);
  transform: translateY(-1px);
}
```

Dark glass theme (gradient pill + strong glow):

```css
.btn-primary {
  border-radius: 9999px;
  background: linear-gradient(135deg, var(--yoizen-primary-1), var(--yoizen-primary-2));
  color: #fff; font-weight: 600;
  padding: 0.65rem 1.45rem;
  border: 1px solid rgba(255, 255, 255, 0.28);
  box-shadow: 0 16px 40px rgba(26, 102, 255, 0.45);
  transition: transform 0.15s ease-out, box-shadow 0.15s ease-out, filter 0.15s ease-out;
}
.btn-primary:hover { filter: brightness(1.05); transform: translateY(-1px); }
.btn-primary:active { transform: translateY(0); }
.btn-primary:disabled { cursor: not-allowed; opacity: 0.65; filter: saturate(0.7); }
```

### 3. Outline variants via custom property overrides

One `.outline` rule parameterized per variant — keeps CSS DRY:

```css
.btn.outline {
  color: var(--btn-outline-color, var(--text));
  background: var(--btn-background, transparent);
  border-color: var(--btn-outline-border, var(--border-strong));
  box-shadow: none;
}
.btn.outline:hover:not(:disabled) {
  color: var(--btn-outline-hover-color, #fff);
  background: var(--btn-outline-hover-bg, var(--primary));
}
.btn-secondary.outline {
  --btn-outline-color: var(--secondary);
  --btn-outline-border: var(--secondary);
  --btn-background: var(--secondary-soft);
  --btn-outline-hover-bg: var(--secondary);
}
```

### 4. Global focus ring

```css
a:focus-visible, button:focus-visible,
input:focus-visible, select:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
}
```

### 5. Semantic alerts and pill badges

Soft background + matching border + readable text, per semantic color (`--danger-soft`/`--danger-border`, etc.). Badges are uppercase pills:

```css
.pill-badge {
  border-radius: 9999px;
  padding: 0.15rem 0.75rem;
  font-size: 0.7rem;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  border: 1px solid rgba(148, 163, 184, 0.5);
}
.pill-success { background: rgba(22, 163, 74, 0.2); border-color: rgba(74, 222, 128, 0.45); color: rgba(187, 247, 208, 0.95); }
.pill-danger  { background: rgba(220, 38, 38, 0.2); border-color: rgba(248, 113, 113, 0.45); color: rgba(254, 202, 202, 0.95); }
```

### 6. Toasts with motion

Enter with spring-like cubic-bezier, exit fade-up, linear progress bar; semantic border/icon tints. Always honor reduced motion:

```css
.toast-card { animation: toast-enter 0.44s cubic-bezier(0.16, 1, 0.3, 1); }

@media (prefers-reduced-motion: reduce) {
  .toast-card, .toast-closing, .toast-progress { animation: none; }
}
```

Full toast implementation (stack, body, icon, progress, semantic variants): `assets/css-snippets.css`.

## Visual Correction Checklist

When asked to fix or polish an existing screen, audit against this list and correct every miss:

1. **Background** — flat solid color? Replace with the theme's ambient gradient stack.
2. **Palette drift** — off-brand hexes, muddy grays, default browser blues? Realign to brand tokens.
3. **Buttons** — no hover lift, no glow, abrupt color-only hover? Apply pattern #2; add `:active` and `:disabled` states.
4. **Borders & shadows** — harsh `1px solid #ccc` + `box-shadow: 0 1px 2px`? Use `--border`/`--border-strong` and `--shadow-soft` (or glass-panel in dark).
5. **Spacing** — arbitrary paddings/margins? Normalize to `--space-*` tokens; consistent rhythm beats more whitespace.
6. **Typography hierarchy** — everything same size/weight? Establish title (600–700), body, and muted levels using `--text`/`--text-muted`.
7. **Radius consistency** — mixed 2px/4px/10px corners? Normalize to `--radius-sm/md/lg`; pills for badges and (dark theme) buttons.
8. **Semantic states** — raw red/green text for errors/success? Use the soft+border alert/badge patterns.
9. **Focus & a11y** — missing focus styles, contrast under 4.5:1? Add the global focus ring; fix contrast.
10. **Motion** — no transitions, or janky long ones? 140–150ms ease for hovers, spring cubic-bezier for entrances, `prefers-reduced-motion` guard.
11. **Empty/loading states** — blank divs while loading? Add spinners or skeletons on `--surface-soft` (see `.spinner` in `assets/css-snippets.css`).
12. **Icons** — emojis, mixed sets, inconsistent sizes, hardcoded colors? Migrate to Lucide per the Iconography norms.

Correct at the token level first (palette/base files), then per-component — a fixed palette improves every screen at once.

## CSS Architecture (pure-CSS projects)

Keep styles modular — one file per concern, imported from `styles.css`:

```css
@import "./styles/palette.css";   /* tokens only */
@import "./styles/base.css";      /* reset, body, typography */
@import "./app/shared/styles/buttons.css";
@import "./app/shared/styles/forms.css";
@import "./app/shared/styles/table.css";
@import "./app/shared/styles/modal.css";
@import "./app/shared/styles/alerts.css";
@import "./app/shared/styles/focus.css";
/* feature styles under ./app/features/<feature>/styles/ */
```

Rules:
- Tokens live **only** in `palette.css`; components consume `var(--*)`, never raw hex
- Shared component styles in `app/shared/styles/`, feature-specific in `app/features/<feature>/styles/`
- New semantic colors get a `-soft` + `-border` pair

## Brand Assets

All SVGs in `assets/` are the **official Yoizen brand files** (sourced from the brand kit). The palette is the logo's palette — any color decision must trace back to it.

### Logos primarios (wordmark horizontal)

| File | Background | Usage |
|------|-----------|-------|
| `logo.svg` | Light / white | Header, landing pages |
| `logo-negativo.svg` | Dark | Dark backgrounds (white text) |
| `logo-blanco.svg` | Any dark | All-white version |
| `logo-negro.svg` | Light | All-black version |

### Logo secundario (formato cuadrado / compacto)

| File | Usage |
|------|-------|
| `logo-secundario.svg` | Compact spaces, app shells |
| `logo-secundario-blanco.svg` | Compact on dark |
| `logo-secundario-negro.svg` | Compact on light |

### Logo + Slogan

| File | Usage |
|------|-------|
| `logo-slogan.svg` | Landing pages, marketing |
| `logo-slogan-negativo.svg` | Dark backgrounds |
| `logo-slogan-blanco.svg` | All-white on dark |

### Isotipo (símbolo solo — para favicon, ícono de app, marca pequeña)

| File | Usage |
|------|-------|
| `icon.svg` | Favicon, avatar, sidebar brand icon (color) |
| `icon-blanco.svg` | White isotipo on dark |
| `icon-negro.svg` | Black isotipo on light |

```html
<!-- Header: logo completo -->
<img src="/assets/logo.svg" alt="Yoizen" class="h-8 w-auto" />

<!-- Sidebar dark: logo negativo -->
<img src="/assets/logo-negativo.svg" alt="Yoizen" class="h-7 w-auto" />

<!-- Sidebar compact o favicon -->
<img src="/assets/icon.svg" alt="Yoizen" class="h-9 w-9" />
```

### CSS & templates

| File | Usage |
|------|-------|
| `css-snippets.css` | Gradients, glass panel, spinner, pills, toasts, scrollbar, layout |
| `component-template.ts` | Angular standalone template (signals, OnPush) |

## Iconography

**Mandatory icon set: [Lucide](https://lucide.dev) (`lucide-angular`).** Uniform 2px stroke outline icons that sit well on both themes. One set per app — never mix libraries.

```bash
npm install lucide-angular
```

```ts
import { LucideAngularModule, Search, Trash2, Pencil } from 'lucide-angular';

@Component({
  imports: [LucideAngularModule],
  template: `<lucide-icon [img]="SearchIcon" [size]="20" />`,
})
export class MyComponent {
  protected readonly SearchIcon = Search;
}
```

Norms:
- **Sizes**: 16px inline with text / inside inputs, 20px buttons and nav (default), 24px page headers and empty states. Never other sizes.
- **Stroke**: keep the default `stroke-width: 2`; never mix stroke widths or outline/filled styles.
- **Color**: icons inherit `currentColor` — never hardcode icon colors. Semantic icons take the semantic token (`--danger`, success/warning equivalents); interactive icons follow their button/link color.
- **Spacing**: icon-to-label gap `0.4–0.5rem` (see `.btn` gap and `.loading-inline`).
- **Icon-only buttons**: circular, with border and hover state (see `.icon-button` in `assets/css-snippets.css`) and an `aria-label`.
- **NEVER** use emojis, mixed icon sets, or ad-hoc inline SVGs when a Lucide icon exists.

## Best Practices

### DO:
- Verify the project is on the latest stable Angular major (`ng version`) — upgrade with `ng update` if it's behind
- Use signals, zoneless change detection, and native control flow (`@if`/`@for`/`@defer`) in all new code
- Detect the project's styling approach (pure CSS tokens vs Tailwind 4) before writing styles
- Use existing tokens (`var(--primary)`, `var(--space-4)`) — extend `palette.css` if one is missing
- Add ambient radial-gradient backgrounds instead of flat colors
- Give buttons lift on hover (`translateY(-1px)`) and a colored glow shadow
- Use `:focus-visible` + `--focus-ring` token for accessibility
- Add `@media (prefers-reduced-motion: reduce)` for any animation
- Maintain WCAG contrast (4.5:1 for body text)
- Use Lucide for all UI icons, sized 16/20/24, colored via `currentColor`

### DON'T:
- Leave a project on an outdated Angular major — latest stable is mandatory
- Write `*ngIf`/`*ngFor`, `@Input()`/`@Output()` decorators, or NgModules in new code
- Imitate a legacy project's current visuals — correct them toward this standard
- Hardcode colors outside the palette
- Create `tailwind.config.js` in Tailwind 4 projects
- Write React/JSX examples — these projects are Angular
- Use pure black `#000000` backgrounds — use `#272a35`/`#1d2029` (dark) or the light gradient
- Put feature styles in shared files or tokens outside `palette.css`
- Use the full brand gradient on large surfaces — reserve it for highlights/CTAs
- Use emojis as icons, mix icon libraries, or hardcode icon colors
