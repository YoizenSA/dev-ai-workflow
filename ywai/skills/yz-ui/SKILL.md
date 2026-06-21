---
name: yz-ui
description: Yoizen UI design system standards. Trigger: Yoizen UI components, styling, colors, typography, visual polish or correction of any Yoizen Angular frontend.
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

## Design Tokens — one set, two themes

The design system is **one set of CSS custom properties** in `:root` (dark = default, `color-scheme: dark`) and **re-themed** by a single `:root[data-theme="light"]` block that redefines only what must change (surfaces dark→light, text→dark, glows/shadows softened, `--white-rgb`/`--black-rgb` flipped; translucent semantics need higher alpha on white). Components consume `var(--*)` and **never** raw hex. **Do not build a second, parallel token system for light.**

**Hue stays constant across themes — only luminosity flips.** A role must read as *the same colour* in dark and light: fix its OKLCH **hue** (keep chroma even), then take a high-L tint for dark (text on dark) and a low-L one for light (text on light). Deriving each theme's tint independently — a washed pastel in dark, a saturated tone in light — makes the hue *jump* on theme switch; that is the bug this rule prevents. Anchor hues to the brand (blue 262°, purple 290°, accent 47°, success 162°, danger 22°) and validate every tint/surface pair ≥ **WCAG AA** in *both* themes. One exception: pure brand yellow is illegible as text on white, so `warning` unifies to a **golden amber ~H75** in both themes and the pure yellow stays for dots/icons only. Keep the accent **orange** at H47 in dark too (not a peachy H66) so it doesn't drift to terracotta in light.

**On dark, also drop chroma ~25% vs a pure tint.** Over a dark background a light+saturated colour *glows* and reads neon — even at equal/lower chroma than its light counterpart, because the high luminosity over near-black is what brightens it. Lowering the dark chroma matches the sobriety the light tints already have (dark text on white). The biggest offender is the **brand gradient**: `--grad-brand` must be a **sober low-chroma** indigo→violet (`#4d78ca/#5d55a9`), *not* the raw neon `--yz-primary-1/2` — otherwise every bar/fill that uses it (dashboard bars, progress, toasts, avatar, segmented, calendar-day) reignites the neon you removed from the tints. Validate the de-saturated set still clears AA (it does: chroma barely moves luminance).

**Copy `assets/palette.css` verbatim** — it is the canonical token file (dark + light, ~120 tokens). Families:

```css
--yz-primary-1/2; --yz-accent; --yz-yellow; --yz-dark; --yz-light;   /* brand */
--*-rgb  /* channels for rgba(var(--x-rgb), a): brand, --surf-1..12-rgb, --white-rgb/--black-rgb (flip) */
--surface; --surface-soft; --surface-hover; --panel-border; --panel-border-strong; --input-bg;
--text; --text-soft; --text-muted; --text-faint;                     /* hierarchy */
--success/--danger/--warning/--info (+ each -soft, -border);  --tint-*;  /* coloured text on glass */
--btn-primary-bg; --btn-danger-bg; --grad-brand/-accent/-text; --shadow-glass; --glass-highlight; --shadow-lift; --focus-ring;
--radius-sm/md/lg/xl; --space-1..10; --text-2xs..3xl; --t-fast/base/slow + --ease-out; --z-overlay/popover/dropdown/toast;
```

**`assets/base.css` (copy verbatim)** carries the mandatory ambient background — a *fixed* 3-layer radial glow (brand blue + accent + purple over `--yz-dark`, `--glow-1..3`: dark ~0.20, light ~0.18 — **sober and present in both, never neon**) — plus the custom scrollbar, the global `:focus-visible` ring, the `.glass` primitive, and the **theme switch as a circular reveal (View Transitions API)** — driver in `assets/theme.service.ts` (~380ms `--ease-out` clip-path circle from the click origin), with the **sun/moon icon morphing** on its own snapshot (`view-transition-name`, rotate+fade — give the icon `display:inline-flex` so an inline box doesn't drop the name). Guards: keyboard origin via `event.detail === 0`, rapid-toggle via `skipTransition()`; instant cut on no-support / reduced-motion. **A colored glow on the wipe *edge* is NOT achievable** — `clip-path` clips the snapshot's own `drop-shadow` (proven), and the VT layer renders above any overlay you could inject; keep the reveal geometric and put the brand moment in the icon. **Never a flat background.** Always ship `backdrop-filter` with its `-webkit-` prefix, or glass silently degrades to a flat panel on Safari/iOS.

**Glass hierarchy (dark).** `--yz-dark` is a **deep** base (`#131722`) so glass cards lift *above* it — a card lighter than its background reads as elevated (the inverse — a card darker than the page — looks sunken). Borders carry their weight too: `--panel-border` sits ~0.30 alpha so the contour is visible, not a ghost. `--glass-highlight` (an `inset 0 1px 0` lit top edge, **dark only** — nulled in light) gives the lit-glass look; pair it with `--shadow-glass` on cards/modals/popovers. In light, lift comes from the material `--shadow-glass`, not the highlight.

**Coloured shadows must go neutral in light.** A `box-shadow` tinted with a brand/semantic colour leaves a coloured **halo over white**. Route them through tokens — `--shadow-accent` / `--shadow-danger` / `--shadow-glow-primary` — that carry colour in dark and a **neutral grey** shadow in light; and `--shadow-card` (= highlight in dark, `--shadow-glass` in light) for surface lift, so `.card`/`.kpi` need no per-component `data-theme` override. **Never tint a shadow inline** in a component — a class selector can't override an inline style, so its light variant is unreachable.

## Signature Visual Patterns

Apply these when building or correcting UI. Full CSS for each lives in the named `assets/` file.

**1. Ambient background** — never flat; the layered radial brand glow from `assets/base.css` (pattern is mandatory in both themes).

**2. Buttons with lift** — `font-weight: 600`, ~140–150ms transition, `translateY(-1px)` on hover (guard with `@media (hover: hover)`), reset on `:active`, `cursor: not-allowed; opacity ~0.55` disabled. Pill radius in both themes. **Name transitioned properties — never `transition: all`** (it animates disabled/theme flips + layout). **Hover must hold contrast in BOTH themes:** a soft/tinted variant (e.g. `.btn-danger`) whose hover puts white text on a *translucent* fill washes out on light — land the hover on a **solid** fill (add a `:root[data-theme="light"] .btn-x:hover` override if the base was tuned for dark). Full set (primary/ghost/danger/danger-solid/accent/outline + `.btn-sm`/`.btn-xs`): `assets/buttons.css`.

**3. Outline variants via custom-property overrides** — one `.btn-outline` rule parameterized with `--btn-outline-color/-border/-hover-bg`; each variant only sets those vars. Keeps CSS DRY. See `assets/buttons.css`.

**4. Global focus ring** — `:focus-visible { box-shadow: var(--focus-ring) }` on links/buttons/inputs/selects/textareas (in `assets/base.css`).

**5. Semantic alerts & pills** — soft bg + matching border + readable text per semantic (`--x-soft`/`--x-border`). Pills are rounded with a status dot; tags are compact mono chips. `assets/components.css`. **Interactive/copyable chip inside a table row** (click-to-copy a path/URL): the hover must **not** reuse `--surface-hover` — the row already lands on it at hover, so the chip stacks into a flat muddy grey box. Use a **brand tint** (`--info-soft` + `--info-border`) so it reads as an *action* distinct from the neutral row hover; reveal a small copy icon on hover via **`opacity` (keep it in flow), not `display`**, so layout doesn't jump; and keep the tooltip a **short static label** ("Copiar …"), never the live (possibly huge) value.

**6. Toasts with motion** — top-stacked glass cards: spring cubic-bezier enter, fade-up exit, linear progress bar, semantic border/icon tints; full-width bottom on mobile; honor `prefers-reduced-motion`. CSS in `assets/components.css`; driver `assets/toast.service.ts` + `toast-stack.component.ts` (adapt the API). Norms baked into those assets: **(a) duration scales with severity + message length** (`base + ~45ms/char`, clamped per type, ~3–10s band) — never one fixed timeout; **errors linger longest** and the progress bar is synced to the real duration via a `--toast-dur` custom prop (a hardcoded bar duration *lies* once durations vary). **(b) Accessibility:** the stack is a live region — error toasts get `role="alert"`/`aria-live="assertive"`, everything else `role="status"`/`polite`; plus a **keyboard-reachable close button** (click-the-body-only isn't focusable) and **pause-on-hover/focus** (`animation-play-state: paused` on the bar + the service stops the timer). **(c) Semantics:** four types `success|error|warning|info` — *successful destructive actions are `success`/neutral, never `error`* (red = the op failed, only); user-correctable input problems are `warning` (amber), not `error`; icons `check-circle-2/circle-alert/triangle-alert/info` (info is `info`, **not** `activity`); copy confirmations use a `clipboard-check` icon (`toast.copied()`).

**7. Loading & disabled states — never flash controls on data load.** Controls carry a 140–150ms transition, so toggling `disabled` **replays it** — binding `disabled` (or a reactive form's `disable()`/`enable()`) to a loading flag makes the toolbar visibly flicker on every load.

```ts
effect(() => { isLoading() ? form.disable() : form.enable(); });  // ✗ flickers
<app-select [disabled]="isLoading()" />                            // ✗
<app-select />                                                     // ✓ stays interactive
```

- Keep **filters/read controls interactive while loading**; show loading in the content area (spinner/skeleton on `--surface-soft`, `aria-busy`).
- Reserve `disabled` for real **action** states the user initiates and that don't flip on navigation (`isSaving`, validating, `!canExport`). A control that enables **once** when data first arrives is fine.

**8. Modals & overlays (glass dialog).** Structure: `.overlay` (scrim) → `.modal` with `.modal-head` / `.modal-body` (scrolls) / `.modal-foot` (`.modal-foot.split` pushes a destructive action left). Drive open/close with a **signal** + `@if` — never leave a hidden modal in the DOM. CSS in `assets/modal.css`. Accessibility is **mandatory** — wire the **`[yzModal]` directive** (`assets/yz-modal.directive.ts`) on every modal:

- `role="dialog"` + `aria-modal="true"` + `aria-labelledby` → title.
- Close on Escape / overlay click / close-button (`aria-label`); `stopPropagation` on the card.
- **Focus trap** (cycle Tab/Shift+Tab, restore focus to trigger on close) + **scroll-lock** the body.
- Entrances (`overlay-in`, `modal-in`) behind `prefers-reduced-motion`.
- **Anchored dropdowns prefer opening DOWNWARD; the modal stays centered.** `ydAnchored` measures the space below the trigger against the **viewport** (not the modal), so the menu drops down — even spilling below the modal's edge/footer — rather than flipping up and covering content (a flipped-up menu hiding the field above it is the bug this prevents). It only flips up (`yd-menu-up`) when the space below is under the usable minimum *and* above offers more; with many options it clamps `--yd-pop-maxh` and scrolls internally. `ydConfineToModal` now only caps the *top* bound to the modal (for that rare upward flip). **Never top-anchor the overlay to make room** — the modal is always centered (no `:has(yd-date)` special-case); the fixed-height date calendar flips on its own when it doesn't fit below. Reference: `assets/yd-anchored.directive.ts`.
- **Type-to-confirm for destructive actions.** Show the exact token to type as its own prominent, **selectable** `.confirm-token` block (mono, `user-select: all`, `overflow-x` for long ids) — *not* a chip buried in the sentence. Give the input **live-match feedback**: wrap it in `.confirm-input-wrap` toggling `.is-match` when the typed value equals the token → success border + a `.confirm-check` ✓ fades in. Keep the confirm button `disabled` until it matches. `.confirm-*` in `assets/forms.css`. (Gotcha seen twice: an `inline-flex`/content-sized control as a direct child of a flex **column** like `.field`/`.cell-stack` stretches full-width via `align-self: stretch` — add `align-self: flex-start` to hug content.)

**9. Collapsible sidebar — the collapsed rail (when present).** Hide labels, center icons, keep the label as a tooltip (`data-tip-pos="right"`) + `aria-label`; swap wordmark→isotipo (`icon.svg`); stack footer tools (theme/collapse) as **contained icon-buttons a step below the user chip** (e.g. ~53×38 in a ~78px rail, ~20px icon) — never wide-short pills. Preserve focus order in both states. CSS in `assets/shell.css`.

**10. Tooltips (CSS-only, `[data-tip]`).** Themed glass tooltip replacing native `title` — pure CSS via `::after`/`::before`, no JS. CSS in `assets/components.css`.

```html
<button class="icon-act" [attr.data-tip]="'Eliminar'" aria-label="Eliminar"> … </button>
<a class="nav-link" [attr.data-tip]="collapsed() ? label : null" data-tip-pos="right"> … </a>
```

- Default position **above, centered**; `data-tip-pos="right"`/`"left"` for sides. Action clusters at a row's right edge (`.row-actions`, `.fr-acts`…) auto-open **left** to avoid horizontal scroll. `pointer-events: none` (never steals clicks); `data-tip=""`/`null` = no tooltip. **Never put `opacity` on the element that owns a `[data-tip]`** — the `::after` inherits it, the tooltip renders semi-transparent and the content behind bleeds through (looks "broken"). Dim the icon/content *inside* instead, or hide the control (e.g. a disabled-for-this-user action that doesn't apply — don't show it greyed with a tip).
- **The tooltip is a visual reinforcement — never the sole channel for information.** It fires on `:hover` **and** `:focus-visible` (keyboard-reachable), but touch and screen readers never see it, so whatever it carries must also live in the visible content or an `aria-label`: **(a)** icon-only controls always keep an `aria-label`; **(b)** when the tip reveals info not otherwise present (an error detail, a field's help text), **duplicate it into `aria-label` and make the host focusable** (`tabindex="0"`) — don't bury an error in hover; **(c)** if the same text is already visible beside the control, **drop the tooltip** — reserve it for *extra* info (a chip reading "secret" whose tip adds "· sealed with kubeseal"), never to echo a label. This is the anti-abuse rule: a tooltip earns its place by adding value, not by restating what's on screen.

**11. Themed selects & date pickers (never native).** Native `<select>`/`<input type=date>` show OS chrome that breaks the glass look. Use `assets/yd-select.component.ts` (signal select: glass popover, search above a threshold, `tags` mode) and `assets/yd-date.component.ts` (themed **calendar**: month grid, prev/next, today/clear). Both position via `assets/yd-anchored.directive.ts` (see #8). Adapt the components' I/O but keep the markup contract (`.yd-select` / `.yd-cal`) so `assets/forms.css` + `components.css` apply. The calendar comes out identical because look **and** logic ship together. **Selected option (`.yd-select-opt.sel`) is minimal:** a faint brand-blue fill + a `✓` — no side bar, no glow (an over-decorated selected row reads loud, not premium).

**12. App shell & responsive layout (reference, not mandatory).** The design system is **layout-agnostic** — tokens/base/components assume *no* particular nav. The shell (`assets/shell.css`: collapsible left sidebar + mobile drawer) is **one reference layout — reshape or replace it**. A top-nav app keeps every token/component/primitive and just builds a different chassis with the same techniques:

- **Desktop collapse:** a `.collapsed` class on a CSS-grid shell flips `grid-template-columns` from `var(--sidebar-w) 1fr` to `var(--sidebar-w-collapsed) 1fr`; `.collapsed .x` rules drive the rail (pattern #9). Toggled by a signal.
- **Mobile drawer:** below the seam the grid drops to one column; the sidebar goes `position: fixed` + `transform: translateX(-105%)`, sliding in via `.open`, with a blurred **scrim** and a floating **FAB (☰)** (z: fab < scrim < drawer). **The same off-canvas pattern works for a top nav** — just anchor it to the top.
- **Breakpoint seams** (desktop-first): KPI/grids 3→2→1, master-detail editors stack, `form-grid`→1col, tighter padding; tables drop columns with `col-hide-md`/`col-hide-lg` (keep identifier + primary action) instead of horizontal scroll; toasts full-width bottom; modals cap `90vh`. Guard hover lifts with `@media (hover: hover)`.

**13. Data tables (`.data-table`, `assets/table.css`).** A few hard-won rules:

- **`border-collapse: separate; border-spacing: 0`** — *not* `collapse`. A `position: sticky` header with `collapse` doesn't paint its border continuously (it looks **serrated/stepped** under the titles, worse on fixed-width columns from sub-pixel rounding).
- **No `backdrop-filter` on a sticky `th`** — the blur over a sticky cell glitches on repaint (the line "breaks" until you refresh). Give the header an **opaque** `--table-head-bg` instead; it covers the scroll without blur.
- **Header reads by typography + a luminosity step, not a heavy bar or hard line**: `text-soft` + bold + uppercase, on a `--table-head-bg` that is a clear step off the rows (a touch *darker* in dark, a defined grey in light — if it's near-identical it "gets lost in the data"). One **dominant** hairline header↔body; row lines are far lighter or absent.
- **Borderless rows** (Linear/Stripe): rows separate by **hover** (a bit more present without lines) + airy padding; the only line is the header's. Maximum data-ink.
- **A table that fills a card**: the wrap must clip to the **card's** radius (`.card > .table-wrap:first/last-child { border-radius: var(--radius-lg) }`), else the opaque header keeps a square corner ("clipped border").
- **Don't over-shrink columns** of short content to `width:1%` — one flex column then hoards the space and the rest **clump to one edge**. Let columns distribute naturally (the identifier column absorbs slack).
- **Column alignment = a scan edge that matches the header.** Text/identifiers/categorical → **left**; numbers → **right** (digits line up). **Status pills/badges → left too** (`col-min`: `width:1%` + default left), *not* centered: centered variable-width pills zig-zag both edges and read messy. Reserve center only for **fixed-width or icon-only** status columns. This is the GitHub/Linear/Stripe audit-log convention — a clean left edge down the column.

## Visual Correction Checklist

Audit a screen against this; each item points at the pattern with the fix.

1. Flat background → ambient gradient stack (#1).
2. Off-brand hexes / muddy grays / default browser blues, **a role whose hue jumps between dark↔light**, or accents so saturated they read **neon** → realign to brand tokens; one hue per role, vary only luminosity (Design Tokens).
3. Buttons without lift/glow, abrupt color-only hover, or a soft variant washing out in light → #2.
4. Harsh `1px solid #ccc` + flat `0 1px 2px` shadow → `--panel-border`/`-strong` + `--shadow-glass` / `.glass`. A **colour-tinted shadow** (or one inlined in a component) that leaves a halo over white → token that goes **neutral in light** (Glass hierarchy).
5. Arbitrary paddings/margins → `--space-*` tokens.
6. No type hierarchy → title 600–700 / body / muted via `--text`/`--text-soft`/`--text-muted`/`--text-faint` + type scale.
7. Mixed corner radii → `--radius-sm/md/lg`; pills for badges and (dark) buttons.
8. Raw red/green state text → soft+border alerts/pills (#5).
9. Missing focus styles, or contrast < 4.5:1 **in either theme** (check tint/surface pairs in both) → global focus ring (#4) + fix contrast.
10. No transitions / janky / `transition: all` / keyframes without a `prefers-reduced-motion` guard → 140–150ms ease, named properties, guard **every** keyframe (#2).
11. Blank divs while loading, or toolbar disabled to signal loading → spinner/skeleton/empty on `--surface-soft`, controls stay interactive (#7).
12. Emojis / mixed icon sets / hardcoded icon colors / odd sizes → Lucide 16/20/24 via `currentColor` (Iconography).
13. `backdrop-filter` without `-webkit-`, or dark theme missing `color-scheme: dark` → add both.
14. Bare overlay (no `role=dialog`/focus-trap/scroll-lock/Escape) → `[yzModal]` + `.overlay`/`.modal` (#8).
15. Collapsed footer tools as wide-short pills → contained icon-buttons below the avatar (#9).
16. Native `title`, icon-only control with no hover hint, a tooltip that's the **only** carrier of an error/description (lost on touch & screen readers), or a tip **echoing already-visible text** → `[data-tip]` + `aria-label`; duplicate critical info into `aria-label` + make focusable; drop redundant tips (#10).
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

- **Copy `palette.css` + `base.css` verbatim** (brand truth, both themes); adapt the rest.
- Tokens live **only** in `palette.css`; components consume `var(--*)`, never raw hex.
- New semantic colors get a `-soft` + `-border` pair, defined in **both** theme blocks.

## Assets

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
| `components.css` | Pills, tags, cards, page/section headers, KPI, alerts, spinner, skeleton, empty states, toasts, **tooltips**, code-chip, diff box, key-value rows |
| `shell.css` | **Reference layout** (swap/reshape per app) — sidebar/drawer, topbar, login, responsive |

### Behavioral primitives (TypeScript — wire to the project's components)

| File | Role |
|------|------|
| `yz-modal.directive.ts` | Accessible dialog: `role`/`aria-modal`, focus-trap, scroll-lock, Escape |
| `yd-anchored.directive.ts` | Popover positioning: flip/clamp vs viewport or modal (`ydConfineToModal`) |
| `yd-select.component.ts` | Themed select with search / tags |
| `yd-date.component.ts` | Themed **calendar** date picker (month/year jump, keyboard nav, optional relative presets for filters) |
| `toast.service.ts` + `toast-stack.component.ts` | Toast system (CSS in `components.css`) |
| `theme.service.ts` | Dark⇄light **circular reveal** (View Transitions): brand-glow edge + icon morph (CSS in `base.css`) |
| `component-template.ts` | Angular standalone starting point (signals, OnPush) |

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
- **CSS**: prefer `transform`/`opacity` (compositor-only); keep the ambient glow a fixed `body::before` (not `background-attachment: fixed`) to avoid full repaints on scroll. (Transitioned-property naming and the `disabled`-flicker rule live in #2 and #7 — not restated here.)
