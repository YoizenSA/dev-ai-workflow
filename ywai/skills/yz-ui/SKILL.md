---
name: yz-ui
description: "Yoizen UI design system. Trigger: Yoizen UI components, styling, colors, typography, Angular frontend polish."
license: Apache-2.0
---

## When to Use

- Creating new UI for any Yoizen frontend
- **Correcting visually poor screens** — bringing legacy UIs up to this standard
- Choosing colors, fonts, spacing, shadows; working with icons and brand assets
- Auditing a project for design-system compliance

## Scope

These are the **mandatory UI norms for every Yoizen frontend** — existing repos and new ones alike. The skill is self-contained: tokens, themes, patterns, and copyable artifacts live in `assets/`. If a project's visuals deviate, the project is wrong, not the norm — correct it with the checklist. Never imitate a legacy project's existing look.

**How to apply it to any project:** copy the canonical artifacts from `assets/`. `palette.css` (tokens, dark + light) and `base.css` go in **verbatim** (brand truth). The component CSS (`buttons/forms/table/modal/components/shell`) and the behavioral primitives (modal/anchored directives, `yd-select`, `yd-date` calendar, toasts) are copied and **wired/renamed to the project's components**. A brand-new app comes out identical — colors, dark/light, animations, tooltips, calendar, responsive — without re-deriving the design.

## References — read the one the task needs

| File | Read it when |
|------|--------------|
| [references/patterns.md](references/patterns.md) | Building or correcting a specific element. The 14 signature patterns (background, buttons, alerts, toasts, loading, modals, rail, tooltips, selects/date, shell, tables, diff) with their hard-won gotchas. The checklist below points here by number. |
| [references/theming.md](references/theming.md) | Touching the palette, adding a semantic color, or a role that looks wrong in one theme. Token families, the hue/luminosity rule, glass hierarchy, colored shadows. |
| [references/assets.md](references/assets.md) | You need the file on disk: brand logos, the CSS bundle, the TypeScript primitives. |

## Tech Stack Norms

Yoizen frontends are **Angular** (standalone components) — **never React/JSX**.

**MANDATORY: latest stable Angular major.** Check `ng version` / `package.json`; if behind, upgrading is part of the work — one major at a time, per `https://angular.dev/update-guide`. Run bare `ng update` first to see everything updatable, and bump the Angular packages **together with their peers** (`@angular/cdk`/Material, `lucide-angular`, …) in the same major step — `ng update @angular/core @angular/cli` alone leaves peers on the old major (peer-dep conflicts). **Never `--force`** (it installs incompatible peers). Commit before each major; `ng update` runs migration schematics, so review the diff and run the app after each step. New code uses zoneless change detection, signals (`input()`/`output()`/`computed()`), and native control flow (`@if`/`@for`/`@defer`). Never `*ngIf`/`*ngFor`, `@Input()`/`@Output()` decorators, or NgModules.

Detect the styling approach before writing styles:

| Approach | Detection |
|----------|-----------|
| **Pure CSS + design tokens** (default) | `:root` custom properties in `palette.css`, modular CSS per feature |
| **Tailwind CSS 4** | `@import "tailwindcss"` in `styles.css` (CSS-first) — **never** a `tailwind.config.js` (configure via `@theme`; see `assets/tailwind-theme-schema.json`) |

## Brand Palette

Source of truth: `assets/paleta-institucional.png`. Exact hexes:

| Color | Hex | Usage |
|-------|-----|-------|
| Primary Blue | `#1A66FF` | Buttons, links, primary actions |
| Secondary Purple | `#4A3ABF` | Supportive emphasis, gradients |
| Accent Orange | `#FD6421` | Critical CTAs, urgent attention |
| Yellow | `#FDBD27` | Auxiliary highlights |
| Dark | `#272A35` | Text, dark surfaces |
| Dark Navy / Darker Navy | `#00183F` / `#00122D` | Deep dark bg alternatives |

Brand gradients are tokens in `palette.css`: `--grad-full` (blue→purple→orange), `--grad-brand` (blue→purple, for buttons/highlights), `--grad-accent`, `--grad-text` (wordmark). **Reserve the full gradient for highlights/CTAs — never large surfaces. Never use `#000` as a background** (use `--yz-dark`/`--yz-dark-soft` on dark, or the light ambient gradient). Realign any off-brand blues / muddy grays to these hexes — it's part of any UI work.

## Token Rules — the short form

The system is **one set of CSS custom properties** in `:root` (dark = default) re-themed by a single `:root[data-theme="light"]` block. Components consume `var(--*)` and **never** raw hex. Non-negotiables:

- **Copy `assets/palette.css` and `assets/base.css` verbatim.** Tokens live only in `palette.css`. Never build a second, parallel token system for light.
- **One hue per role; only luminosity flips between themes.** A role that changes hue on theme switch is a bug. Validate every tint/surface pair ≥ WCAG AA in *both* themes.
- **On dark, drop chroma ~25%** vs the pure tint, or colors read neon over the dark background.
- **Never a flat background** — `base.css` carries the mandatory ambient glow. Always ship `backdrop-filter` with its `-webkit-` prefix.
- **Never tint a shadow inline** in a component — route it through a token that goes neutral in light, or it leaves a colored halo over white.
- New semantic colors get a `-soft` + `-border` pair, defined in **both** theme blocks.

The reasoning behind each of these, the full token family list, and the glass-hierarchy rules are in [references/theming.md](references/theming.md) — read it before touching the palette.

## Visual Correction Checklist

Audit a screen against this. Each item names the pattern in [references/patterns.md](references/patterns.md) that carries the fix.

1. Flat background → ambient gradient stack (#1).
2. Off-brand hexes / muddy grays / default browser blues, **a role whose hue jumps between dark↔light**, or accents so saturated they read **neon** → realign to brand tokens (Token Rules).
3. Buttons without lift/glow, abrupt color-only hover, or a soft variant washing out in light → #2.
4. Harsh `1px solid #ccc` + flat `0 1px 2px` shadow → `--panel-border`/`-strong` + `--shadow-glass` / `.glass`. A **colour-tinted shadow** that leaves a halo over white → token that goes neutral in light (theming).
5. Arbitrary paddings/margins → `--space-*` tokens.
6. No type hierarchy → title 600–700 / body / muted via `--text`/`--text-soft`/`--text-muted`/`--text-faint` + type scale.
7. Mixed corner radii → `--radius-sm/md/lg`; pills for badges and (dark) buttons.
8. Raw red/green state text → soft+border alerts/pills (#5).
9. Missing focus styles, or contrast < 4.5:1 **in either theme** → global focus ring (#4) + fix contrast.
10. No transitions / janky / `transition: all` / keyframes without a `prefers-reduced-motion` guard → 140–150ms ease, named properties, guard **every** keyframe (#2).
11. Blank divs while loading, or toolbar disabled to signal loading → spinner/skeleton, controls stay interactive (#7).
12. Emojis / mixed icon sets / hardcoded icon colors / odd sizes → Lucide 16/20/24 via `currentColor` (Iconography).
13. `backdrop-filter` without `-webkit-`, or dark theme missing `color-scheme: dark` → add both.
14. Bare overlay (no `role=dialog`/focus-trap/scroll-lock/Escape) → `[yzModal]` + `.overlay`/`.modal` (#8).
15. Collapsed footer tools as wide-short pills → contained icon-buttons below the avatar (#9).
16. Native `title`, icon-only control with no hover hint, a tooltip that's the **only** carrier of an error, or a tip **echoing already-visible text** → `[data-tip]` + `aria-label` (#10).
17. Raw `<select>`/`<input type=date>` OS chrome → themed `yd-select`/`yd-date` (#11).
18. Fixed-width layout / horizontal table scroll / sidebar not collapsing on mobile → #12.

Correct at the token level first (palette/base), then per-component — a fixed palette improves every screen at once. **The audit is done only when all 18 items have been checked against the screen** — not the first few that obviously match.

## CSS Architecture (pure-CSS projects)

Copy the canonical files from `assets/` and import them once from `styles.css`, **in this order** (later files consume the tokens):

```css
@import "./styles/palette.css";               /* tokens only — dark + light */
@import "./styles/base.css";                  /* reset, ambient bg, scrollbar, focus, glass, theme reveal */
@import "./app/shared/styles/buttons.css";
@import "./app/shared/styles/forms.css";      /* inputs, fields, yd-select, yd-cal */
@import "./app/shared/styles/table.css";      /* data-table + col-hide-* responsive */
@import "./app/shared/styles/modal.css";
@import "./app/shared/styles/components.css"; /* pills, tags, cards, headers, KPI, alerts, spinner, skeleton, empty, toasts, tooltips, diff, kv */
@import "./app/layout/shell.css";             /* reference shell — swap per app */
/* feature-specific styles stay under ./app/features/<feature>/ */
```

## Iconography

**Mandatory icon set: [Lucide](https://lucide.dev) (`lucide-angular`)** — uniform 2px-stroke outline icons. One set per app; never mix libraries or use emojis/ad-hoc SVGs when a Lucide icon exists.

```ts
import { LucideAngularModule, Search } from 'lucide-angular';
@Component({ imports: [LucideAngularModule], template: `<lucide-icon [img]="SearchIcon" [size]="20" />` })
export class MyComponent { protected readonly SearchIcon = Search; }
```

- **Sizes**: 16 inline/inputs · 20 buttons & nav (default) · 24 page headers & empty states. No other sizes.
- **Stroke**: default `stroke-width: 2`; never mix widths or outline/filled.
- **Color**: inherit `currentColor` — never hardcode. Semantic icons take the semantic token; interactive icons follow their button/link color.
- **Icon-only buttons**: circular, bordered, with hover (`.btn-icon` in `buttons.css`, or `.icon-act` for inline row actions in `components.css`) + an `aria-label`.

## Performance

- **Change detection**: `ChangeDetectionStrategy.OnPush` on every component, zoneless app. Derive view state with `computed()`; don't recompute in the template.
- **Lists**: `@for` **must** declare `track` (stable id) so rows aren't re-created.
- **Routing**: lazy-load feature routes with `loadComponent`; defer heavy/below-the-fold blocks with `@defer`.
- **Data at scale**: paginate + filter **server-side** for growing lists; **debounce** search (~300ms) before querying.
- **CSS**: prefer `transform`/`opacity` (compositor-only); keep the ambient glow a fixed `body::before` (not `background-attachment: fixed`) to avoid full repaints on scroll.
