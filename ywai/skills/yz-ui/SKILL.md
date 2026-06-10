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

These are the exact hex values from the official Yoizen color palette (`assets/paleta-institucional.png`). Every color decision must trace back to this table — no substitutions.

### Colores institucionales

| Swatch | Name | Hex | Usage |
|--------|------|-----|-------|
| 🔵 | Primary Blue | `#1a66ff` | Buttons, links, primary actions, brand glow |
| 🟣 | Secondary Purple | `#4a3abf` | Supportive emphasis, gradients, sidebar active |
| 🟠 | Accent Orange | `#fd6421` | Critical CTAs, urgent attention, accent glow |
| 🟡 | Yellow | `#fdbd27` | Auxiliary highlights, nav badges |
| ⚫ | Dark | `#272a35` | Text, dark surfaces, body background base |
| 🌑 | Dark Navy | `#00183f` | Deep dark background alternative |
| 🌑 | Darker Navy | `#00122d` | Deepest dark background |
| ⚫ | Black | `#000000` | Pure black (use sparingly — not for backgrounds) |

### Brand Gradients

These appear as "Gradient 1" and "Gradient 2" in the palette (`assets/paleta-degrade.png`). The three gradient source colors are `#1a66ff`, `#4a3abf`, and `#fd6421`.

```css
/* Gradient 1 — blue → purple → orange (horizontal, bright) */
background: linear-gradient(90deg, #1a66ff 0%, #4a3abf 55%, #fd6421 100%);

/* Gradient 2 — dark version (blue/purple → orange/black) */
background: linear-gradient(135deg, #1a66ff 0%, #4a3abf 50%, #fd6421 80%, #000000 100%);

/* Brand accent shortcut (UI buttons, highlights, active states) */
background: linear-gradient(135deg, #1a66ff, #4a3abf);
```

All theme tokens below derive from this palette. When a project's colors drift from it (off-brand blues, muddy grays), realigning them to these hexes is part of any UI work.

## Two Canonical Themes

Pick per product; both share the brand palette and token structure. These tokens ARE the standard — copy them as-is into new projects.

### Dark glass theme

Dark base with ambient brand glows, glassmorphism panels, and a full token hierarchy.

```css
:root {
  /* Brand palette */
  --yz-primary-1: #1a66ff;
  --yz-primary-2: #4a3abf;
  --yz-accent:    #fd6421;
  --yz-yellow:    #fdbd27;
  --yz-dark:      #272a35;
  --yz-dark-soft: #1d2029;
  --yz-navy:      #00183f;
  --yz-light:     #ffffff;

  /* Surfaces */
  --surface:             rgba(30, 35, 48, 0.72);
  --surface-strong:      rgba(22, 26, 38, 0.92);
  --surface-soft:        rgba(255, 255, 255, 0.04);
  --surface-hover:       rgba(255, 255, 255, 0.07);
  --panel-border:        rgba(148, 163, 184, 0.18);
  --panel-border-strong: rgba(148, 163, 184, 0.32);

  /* Text hierarchy */
  --text:       #f3f5fb;
  --text-soft:  #c4cbdd;
  --text-muted: #8b93ab;
  --text-faint: #5f6781;

  /* Semantic */
  --success:        #34d399;
  --success-soft:   rgba(16, 185, 129, 0.16);
  --success-border: rgba(52, 211, 153, 0.4);
  --danger:         #f87171;
  --danger-soft:    rgba(220, 38, 38, 0.16);
  --danger-border:  rgba(248, 113, 113, 0.42);
  --warning:        #fbbf24;
  --warning-soft:   rgba(251, 189, 39, 0.15);
  --warning-border: rgba(251, 191, 36, 0.42);
  --info:           #60a5fa;
  --info-soft:      rgba(26, 102, 255, 0.16);
  --info-border:    rgba(96, 165, 250, 0.42);

  /* Radii */
  --radius-sm: 0.55rem;
  --radius-md: 0.85rem;
  --radius-lg: 1.15rem;
  --radius-xl: 1.6rem;

  /* Spacing */
  --space-1: 0.25rem; --space-2: 0.5rem;  --space-3: 0.75rem;
  --space-4: 1rem;    --space-5: 1.25rem; --space-6: 1.5rem; --space-8: 2rem;

  /* Shadows */
  --shadow-glass: 0 24px 60px rgba(5, 9, 20, 0.55);
  --shadow-lift:  0 16px 40px rgba(26, 102, 255, 0.4);
  --focus-ring:   0 0 0 3px rgba(26, 102, 255, 0.42);

  /* Gradient shortcuts */
  --grad-brand:  linear-gradient(135deg, var(--yz-primary-1), var(--yz-primary-2));
  --grad-accent: linear-gradient(135deg, var(--yz-accent), #df4f12);
  --grad-full:   linear-gradient(90deg, #1a66ff 0%, #4a3abf 55%, #fd6421 100%);

  --sidebar-w: 264px;
}

* { box-sizing: border-box; }
html, body { height: 100%; }

body {
  margin: 0;
  font-family: "Inter", "Segoe UI", system-ui, sans-serif;
  color: var(--text);
  line-height: 1.45;
  -webkit-font-smoothing: antialiased;
  text-rendering: optimizeLegibility;
  background:
    radial-gradient(circle at 12%  8%,  rgba(26, 102, 255, 0.30), transparent 52%),
    radial-gradient(circle at 88%  92%, rgba(253, 100,  33, 0.26), transparent 55%),
    radial-gradient(circle at 78%  12%, rgba(74,   58, 191, 0.24), transparent 48%),
    var(--yz-dark);
  background-attachment: fixed;
}

h1, h2, h3, h4 { margin: 0; font-weight: 650; letter-spacing: -0.01em; }
p  { margin: 0; }
button { font-family: inherit; }
::selection { background: rgba(26, 102, 255, 0.4); }
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

Never use flat backgrounds. Layer 2–3 radial gradients (brand blue + purple + orange at opposite corners, ~0.12 alpha light / ~0.24–0.35 alpha dark) over a base gradient or dark color. Add `background-attachment: fixed` in dark theme so the ambient glows stay fixed as content scrolls — this is the effect that makes the dark glass theme feel premium.

### 2. Glass panels

The glass effect is used on sidebars, cards, modals, and panels:

```css
.glass {
  border-radius: var(--radius-lg);
  border: 1px solid var(--panel-border);
  background: linear-gradient(155deg, rgba(38, 44, 60, 0.82), rgba(20, 24, 36, 0.86));
  backdrop-filter: blur(22px);
  -webkit-backdrop-filter: blur(22px);
  box-shadow: var(--shadow-glass);
}
```

### 3. Buttons with lift

All buttons: `font-weight: 600`, `border-radius: 999px` (pill — dark theme), transition on transform + shadow + filter, `translateY(-1px)` on hover, reset on `:active`, `cursor: not-allowed; opacity: 0.55` when disabled.

```css
.btn {
  display: inline-flex; align-items: center; justify-content: center; gap: 0.45rem;
  font-size: 0.9rem; font-weight: 600;
  padding: 0.6rem 1.25rem;
  border-radius: 999px;
  border: 1px solid transparent;
  cursor: pointer; white-space: nowrap;
  transition: transform 150ms ease, box-shadow 150ms ease, filter 150ms ease,
              background 150ms ease, border-color 150ms ease, color 150ms ease;
}
.btn svg { width: 17px; height: 17px; }
.btn:disabled { cursor: not-allowed; opacity: 0.55; filter: saturate(0.6); }

/* Primary — brand gradient pill + strong glow */
.btn-primary {
  background: var(--grad-brand);
  color: #fff;
  border-color: rgba(255, 255, 255, 0.22);
  box-shadow: var(--shadow-lift);
}
.btn-primary:hover:not(:disabled)  { filter: brightness(1.07); transform: translateY(-1px); }
.btn-primary:active:not(:disabled) { transform: translateY(0); }

/* Accent — orange glow */
.btn-accent {
  background: var(--grad-accent);
  color: #fff;
  border-color: rgba(255, 255, 255, 0.2);
  box-shadow: 0 14px 34px rgba(253, 100, 33, 0.4);
}
.btn-accent:hover:not(:disabled) { filter: brightness(1.07); transform: translateY(-1px); }

/* Ghost — subtle surface */
.btn-ghost {
  background: var(--surface-soft);
  color: var(--text-soft);
  border-color: var(--panel-border);
}
.btn-ghost:hover:not(:disabled) { background: var(--surface-hover); color: var(--text); transform: translateY(-1px); }

/* Outline */
.btn-outline {
  background: transparent;
  color: var(--text-soft);
  border-color: var(--panel-border-strong);
}
.btn-outline:hover:not(:disabled) { background: var(--surface-hover); color: var(--text); }

/* Danger */
.btn-danger {
  background: var(--danger-soft);
  color: #fecaca;
  border-color: var(--danger-border);
}
.btn-danger:hover:not(:disabled) { background: rgba(220, 38, 38, 0.3); color: #fff; transform: translateY(-1px); }

/* Danger solid */
.btn-danger-solid {
  background: linear-gradient(135deg, #ef4444, #dc2626);
  color: #fff;
  border-color: rgba(255, 255, 255, 0.18);
  box-shadow: 0 12px 30px rgba(220, 38, 38, 0.38);
}
.btn-danger-solid:hover:not(:disabled) { filter: brightness(1.06); transform: translateY(-1px); }

/* Sizes */
.btn-sm { padding: 0.4rem 0.85rem; font-size: 0.82rem; }

/* Icon-only button — circular */
.btn-icon {
  width: 38px; height: 38px; padding: 0; border-radius: 50%;
  background: var(--surface-soft); border-color: var(--panel-border); color: var(--text-soft);
}
.btn-icon:hover:not(:disabled) { background: var(--surface-hover); color: var(--text); transform: translateY(-1px); }
```

**Light theme** — use `border-radius: var(--radius-sm)` (rounded rect, not pill), replace `--grad-brand` with `var(--primary)`, and reduce the box-shadow alpha.

### 4. Outline variants via custom property overrides

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

### 5. Global focus ring

```css
a:focus-visible, button:focus-visible,
input:focus-visible, select:focus-visible, [tabindex]:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
}
```

### 6. Semantic pill badges with status dot

Pills always include a `.dot` indicator; the running variant has an animated pulse:

```css
.pill {
  display: inline-flex; align-items: center; gap: 0.35rem;
  font-size: 0.72rem; font-weight: 650; letter-spacing: 0.03em;
  padding: 0.22rem 0.6rem 0.22rem 0.5rem;
  border-radius: 999px;
  text-transform: uppercase;
  border: 1px solid transparent;
  white-space: nowrap;
}
.pill .dot { width: 7px; height: 7px; border-radius: 50%; flex-shrink: 0; }

.pill-success { background: var(--success-soft); color: #6ee7b7; border-color: var(--success-border); }
.pill-success .dot { background: var(--success); }
.pill-info    { background: var(--info-soft);    color: #93c5fd; border-color: var(--info-border); }
.pill-info .dot    { background: var(--info); }
.pill-danger  { background: var(--danger-soft);  color: #fca5a5; border-color: var(--danger-border); }
.pill-danger .dot  { background: var(--danger); }
.pill-warning { background: var(--warning-soft); color: var(--warning); border-color: var(--warning-border); }
.pill-warning .dot { background: var(--warning); }
.pill-muted   { background: var(--surface-soft); color: var(--text-muted); border-color: var(--panel-border); }
.pill-muted .dot   { background: var(--text-muted); }

/* Running state — animated pulse dot */
.pill-running .dot { animation: pulse 1.4s ease-in-out infinite; }
@keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.3; } }
```

### 7. Toasts with motion

Spring-like entrance from the right, linear progress bar, semantic icon tints. Always honor reduced motion:

```css
.toast-stack {
  position: fixed; bottom: var(--space-6); right: var(--space-6);
  z-index: 90; display: flex; flex-direction: column; gap: var(--space-3);
}
.toast {
  display: flex; align-items: center; gap: 0.7rem;
  min-width: 300px; max-width: 400px;
  padding: 0.85rem 1rem;
  border-radius: var(--radius-md);
  border: 1px solid var(--panel-border-strong);
  background: linear-gradient(160deg, rgba(38,44,60,0.97), rgba(22,26,38,0.98));
  box-shadow: var(--shadow-glass);
  animation: toastIn 0.44s cubic-bezier(0.16, 1, 0.3, 1);
  position: relative; overflow: hidden;
}
@keyframes toastIn { from { opacity: 0; transform: translateX(30px); } to { opacity: 1; transform: none; } }

.toast-ico { width: 30px; height: 30px; border-radius: 9px; display: grid; place-items: center; flex-shrink: 0; }
.toast-ico svg { width: 17px; height: 17px; }
.toast.success .toast-ico { background: var(--success-soft); color: var(--success); }
.toast.error   .toast-ico { background: var(--danger-soft);  color: var(--danger); }
.toast.info    .toast-ico { background: var(--info-soft);    color: var(--info); }
.toast-msg { font-size: 0.88rem; color: var(--text); }

/* Progress bar that auto-depletes */
.toast-bar {
  position: absolute; left: 0; bottom: 0; height: 2.5px;
  background: var(--grad-brand);
  animation: toastBar 3.2s linear forwards;
}
@keyframes toastBar { from { width: 100%; } to { width: 0%; } }

@media (prefers-reduced-motion: reduce) {
  .toast, .toast-bar { animation: none; }
}
```

### 8. KPI cards

Four-column grid of metric cards with radial glow via CSS custom property, trend delta badge, and hover lift:

```css
.kpi-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: var(--space-4); }

.kpi {
  position: relative;
  padding: var(--space-5);
  border-radius: var(--radius-lg);
  border: 1px solid var(--panel-border);
  background: linear-gradient(155deg, rgba(38,44,60,0.78), rgba(20,24,36,0.84));
  backdrop-filter: blur(18px);
  overflow: hidden;
  transition: transform 160ms ease, border-color 160ms ease;
}
.kpi:hover { transform: translateY(-2px); border-color: var(--panel-border-strong); }

/* Per-card ambient glow in top-right corner, driven by --kpi-glow */
.kpi::after {
  content: ""; position: absolute; inset: 0 0 auto auto;
  width: 130px; height: 130px;
  background: radial-gradient(circle at top right, var(--kpi-glow, rgba(26,102,255,0.35)), transparent 70%);
  pointer-events: none;
}
.kpi-top { display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-4); }

.kpi-icon {
  width: 40px; height: 40px; border-radius: 12px;
  display: grid; place-items: center;
  background: var(--kpi-icon-bg, rgba(26,102,255,0.16));
  color: var(--kpi-icon-color, #79a9ff);
  border: 1px solid var(--panel-border);
}
.kpi-icon svg { width: 20px; height: 20px; }

.kpi-delta {
  display: inline-flex; align-items: center; gap: 0.25rem;
  font-size: 0.78rem; font-weight: 650;
  padding: 0.18rem 0.5rem; border-radius: 99px;
}
.kpi-delta svg { width: 13px; height: 13px; }
.kpi-delta.up   { color: var(--success); background: var(--success-soft); border: 1px solid var(--success-border); }
.kpi-delta.down { color: var(--danger);  background: var(--danger-soft);  border: 1px solid var(--danger-border); }
.kpi-delta.flat { color: var(--text-muted); background: var(--surface-soft); border: 1px solid var(--panel-border); }

.kpi-value { font-size: 2rem; font-weight: 720; letter-spacing: -0.02em; line-height: 1; }
.kpi-label { color: var(--text-muted); font-size: 0.86rem; margin-top: 0.4rem; }
```

Usage — each card receives its own glow color and icon color via inline CSS custom properties:
```html
<div class="kpi" style="--kpi-glow: rgba(26,102,255,0.35); --kpi-icon-bg: rgba(26,102,255,0.16); --kpi-icon-color: #79a9ff;">
```

Collapse to 2 columns at ≤1100px, 1 column at ≤560px.

### 9. App shell — sidebar + content layout

```css
.app-shell {
  display: grid;
  grid-template-columns: var(--sidebar-w) 1fr;
  min-height: 100vh;
}

/* Sidebar */
.sidebar {
  position: sticky; top: 0; height: 100vh;
  display: flex; flex-direction: column; gap: var(--space-2);
  padding: var(--space-6) var(--space-4);
  border-right: 1px solid var(--panel-border);
  background: linear-gradient(185deg, rgba(24,28,42,0.86), rgba(16,19,30,0.92));
  backdrop-filter: blur(18px);
  z-index: 20;
}

/* Brand mark (isotipo + wordmark) */
.brand { display: flex; align-items: center; gap: 0.7rem; padding: 0 var(--space-2) var(--space-2); }
.brand-mark {
  width: 42px; height: 42px; border-radius: 13px;
  display: grid; place-items: center;
  background: linear-gradient(150deg, rgba(26,102,255,0.18), rgba(74,58,191,0.12));
  border: 1px solid rgba(148,163,184,0.22); flex-shrink: 0;
}
.brand-mark img { width: 26px; height: 26px; }
.brand-text { display: flex; flex-direction: column; line-height: 1.1; }
.brand-name { font-size: 1.18rem; font-weight: 720; letter-spacing: -0.02em; }
/* Gradient text on the product name */
.brand-name b { background: var(--grad-full); -webkit-background-clip: text; background-clip: text; color: transparent; }
.brand-sub { font-size: 0.66rem; letter-spacing: 0.18em; text-transform: uppercase; color: var(--text-faint); font-weight: 600; }

/* Section divider */
.nav-section-label {
  font-size: 0.64rem; letter-spacing: 0.16em; text-transform: uppercase;
  color: var(--text-faint); font-weight: 700;
  padding: var(--space-4) var(--space-3) var(--space-2);
}

/* Nav items */
.nav { display: flex; flex-direction: column; gap: 3px; }
.nav-link {
  display: flex; align-items: center; gap: 0.7rem;
  padding: 0.62rem 0.8rem; border-radius: var(--radius-sm);
  color: var(--text-soft); font-size: 0.92rem; font-weight: 550;
  cursor: pointer; border: 1px solid transparent;
  background: transparent; width: 100%; text-align: left;
  transition: background 140ms ease, color 140ms ease, border-color 140ms ease;
}
.nav-link:hover { background: var(--surface-hover); color: var(--text); }
.nav-link.is-active {
  color: #fff;
  background: linear-gradient(135deg, rgba(26,102,255,0.26), rgba(74,58,191,0.18));
  border-color: rgba(96,165,250,0.38);
  box-shadow: inset 0 0 0 1px rgba(255,255,255,0.04), 0 8px 22px rgba(26,102,255,0.22);
}
.nav-link.is-active svg { color: #79a9ff; }

/* Notification badge on nav item */
.nav-badge {
  margin-left: auto; font-size: 0.7rem; font-weight: 700;
  padding: 0.05rem 0.45rem; border-radius: 99px;
  background: var(--warning-soft); color: var(--warning); border: 1px solid var(--warning-border);
}

/* User chip at sidebar bottom */
.sidebar-foot { margin-top: auto; display: flex; flex-direction: column; gap: var(--space-2); }
.user-chip {
  display: flex; align-items: center; gap: 0.65rem;
  padding: 0.6rem 0.7rem; border-radius: var(--radius-md);
  background: var(--surface-soft); border: 1px solid var(--panel-border);
}
.user-avatar {
  width: 34px; height: 34px; border-radius: 50%;
  background: var(--grad-brand); display: grid; place-items: center;
  font-size: 0.85rem; font-weight: 700; color: #fff; flex-shrink: 0;
}
.user-meta { display: flex; flex-direction: column; line-height: 1.2; min-width: 0; }
.user-name  { font-size: 0.85rem; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.user-mail  { font-size: 0.72rem; color: var(--text-muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

/* Main content area */
.content {
  min-width: 0; display: flex; flex-direction: column;
  padding: var(--space-8) clamp(1.2rem, 3vw, 2.6rem) var(--space-8);
  gap: var(--space-6);
}
```

### 10. Page header with eyebrow

```css
.page-header { display: flex; align-items: flex-end; justify-content: space-between; gap: var(--space-5); flex-wrap: wrap; }
.page-heading { display: flex; flex-direction: column; gap: 0.3rem; }

/* Eyebrow: accent-colored, uppercase, tiny — sets context above the title */
.page-eyebrow {
  font-size: 0.7rem; letter-spacing: 0.16em; text-transform: uppercase;
  color: var(--yz-accent); font-weight: 700;
  display: flex; align-items: center; gap: 0.4rem;
}
.page-title    { font-size: clamp(1.5rem, 2.4vw, 2rem); font-weight: 700; }
.page-subtitle { color: var(--text-muted); font-size: 0.95rem; }
.page-actions  { display: flex; align-items: center; gap: var(--space-3); flex-wrap: wrap; }
```

### 11. Dark glass tables

```css
.table-wrap  { overflow-x: auto; border-radius: var(--radius-md); }
.data-table  { width: 100%; border-collapse: collapse; font-size: 0.88rem; }

.data-table thead th {
  text-align: left; font-size: 0.72rem; font-weight: 700;
  letter-spacing: 0.06em; text-transform: uppercase;
  color: var(--text-muted); padding: 0.7rem 1rem;
  border-bottom: 1px solid var(--panel-border);
  white-space: nowrap;
  position: sticky; top: 0;
  background: rgba(24, 28, 40, 0.6); backdrop-filter: blur(8px);
}
.data-table th.sortable { cursor: pointer; user-select: none; }
.data-table th.sortable:hover { color: var(--text); }

.data-table tbody td {
  padding: 0.72rem 1rem;
  border-bottom: 1px solid rgba(148, 163, 184, 0.08);
  color: var(--text-soft); white-space: nowrap;
}
.data-table tbody tr { transition: background 120ms ease; }
.data-table tbody tr:hover { background: var(--surface-soft); }
.data-table tbody tr:last-child td { border-bottom: none; }

/* Cell modifier classes */
.cell-strong { color: var(--text); font-weight: 600; }
.cell-mono   { font-variant-numeric: tabular-nums; font-feature-settings: "tnum"; }
.cell-muted  { color: var(--text-muted); }
.cell-num    { text-align: right; font-variant-numeric: tabular-nums; }
```

### 12. Filter bar with dark inputs

```css
.filters { display: flex; align-items: flex-end; gap: var(--space-3); flex-wrap: wrap; }
.field   { display: flex; flex-direction: column; gap: 0.35rem; min-width: 0; }
.field-label { font-size: 0.74rem; font-weight: 600; color: var(--text-muted); letter-spacing: 0.02em; text-transform: uppercase; }

.input, .select {
  appearance: none;
  background: rgba(15, 18, 28, 0.6); border: 1px solid var(--panel-border);
  color: var(--text); border-radius: var(--radius-sm);
  padding: 0.55rem 0.8rem; font-size: 0.9rem; font-family: inherit; min-width: 150px;
  transition: border-color 140ms ease, background 140ms ease;
}
.input::placeholder { color: var(--text-faint); }
.input:hover,  .select:hover  { border-color: var(--panel-border-strong); }
.input:focus,  .select:focus  { border-color: rgba(96,165,250,0.6); background: rgba(15,18,28,0.85); }

/* Search input with icon */
.search-field { position: relative; }
.search-field .icon-inner { position: absolute; left: 0.7rem; top: 50%; transform: translateY(-50%); width: 16px; height: 16px; color: var(--text-faint); pointer-events: none; }
.search-field .input { padding-left: 2.1rem; }
```

### 13. Glass modal with spring animation

```css
.overlay {
  position: fixed; inset: 0; z-index: 60;
  background: rgba(8, 11, 22, 0.66); backdrop-filter: blur(6px);
  display: grid; place-items: center; padding: var(--space-5);
  animation: fadeIn 0.2s ease;
}
@keyframes fadeIn { from { opacity: 0; } to { opacity: 1; } }

.modal {
  width: 100%; max-width: var(--modal-w, 540px); max-height: 90vh; overflow-y: auto;
  border-radius: var(--radius-xl);
  border: 1px solid var(--panel-border-strong);
  background: linear-gradient(165deg, rgba(34,39,54,0.96), rgba(19,23,34,0.98));
  box-shadow: var(--shadow-glass);
  animation: modalIn 0.34s cubic-bezier(0.16, 1, 0.3, 1);
}
@keyframes modalIn { from { opacity: 0; transform: translateY(14px) scale(0.98); } to { opacity: 1; transform: none; } }

.modal-head {
  display: flex; align-items: flex-start; justify-content: space-between; gap: var(--space-4);
  padding: var(--space-6) var(--space-6) var(--space-3);
}
.modal-title    { font-size: 1.2rem; font-weight: 680; }
.modal-subtitle { font-size: 0.88rem; color: var(--text-muted); margin-top: 0.25rem; }
.modal-close {
  width: 34px; height: 34px; border-radius: 10px; flex-shrink: 0;
  display: grid; place-items: center; cursor: pointer;
  background: var(--surface-soft); border: 1px solid var(--panel-border); color: var(--text-muted);
  transition: all 140ms ease;
}
.modal-close:hover { background: var(--surface-hover); color: var(--text); }
.modal-body { padding: var(--space-3) var(--space-6) var(--space-4); display: flex; flex-direction: column; gap: var(--space-4); }
.modal-foot { display: flex; justify-content: flex-end; gap: var(--space-3); padding: var(--space-4) var(--space-6) var(--space-6); }

/* Two-column form grid inside modals */
.form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-4); }
.form-grid .span-2 { grid-column: 1 / -1; }
.modal-label { font-size: 0.78rem; font-weight: 600; color: var(--text-soft); margin-bottom: 0.35rem; }
```

### 14. Action popup (row context menu)

```css
.action-popup {
  position: fixed; z-index: 70;
  display: flex; flex-direction: column; gap: 4px;
  padding: 6px; border-radius: var(--radius-md);
  border: 1px solid var(--panel-border-strong);
  background: linear-gradient(165deg, rgba(38,44,60,0.98), rgba(22,26,38,0.98));
  box-shadow: var(--shadow-glass); min-width: 170px;
  animation: popIn 0.16s ease;
}
@keyframes popIn { from { opacity: 0; transform: scale(0.96); } to { opacity: 1; transform: none; } }

.popup-item {
  display: flex; align-items: center; gap: 0.55rem;
  padding: 0.5rem 0.7rem; border-radius: var(--radius-sm);
  font-size: 0.86rem; font-weight: 550; cursor: pointer;
  background: transparent; border: none; color: var(--text-soft); text-align: left; width: 100%;
  transition: background 120ms ease, color 120ms ease;
}
.popup-item svg { width: 16px; height: 16px; }
.popup-item:hover        { background: var(--surface-hover); color: var(--text); }
.popup-item.danger:hover { background: var(--danger-soft);   color: #fca5a5; }
```

### 15. Utility components

**Progress bar** (active operations):
```css
.progress-track { height: 6px; border-radius: 99px; background: var(--surface-soft); overflow: hidden; }
.progress-fill  { height: 100%; border-radius: 99px; background: var(--grad-brand); transition: width 0.5s ease; }
```

**Segmented toggle**:
```css
.toggle-seg { display: inline-flex; padding: 3px; border-radius: 99px; background: var(--surface-soft); border: 1px solid var(--panel-border); gap: 2px; }
.seg-btn {
  padding: 0.4rem 0.9rem; border-radius: 99px; border: none; cursor: pointer;
  background: transparent; color: var(--text-muted); font-size: 0.84rem; font-weight: 600; font-family: inherit;
  transition: all 140ms ease;
}
.seg-btn.active { background: var(--grad-brand); color: #fff; box-shadow: 0 6px 16px rgba(26,102,255,0.3); }
```

**Product/service tags** (yFlow, ySocial, ySmart):
```css
.tag {
  display: inline-flex; align-items: center; gap: 0.3rem;
  font-size: 0.74rem; font-weight: 600; padding: 0.16rem 0.5rem; border-radius: 6px;
  background: var(--surface-soft); border: 1px solid var(--panel-border); color: var(--text-soft);
}
.tag.yflow   { background: rgba(26,102,255,0.14); color: #93c5fd; border-color: var(--info-border); }
.tag.ysocial { background: rgba(74,58,191,0.16);  color: #c4b5fd; border-color: rgba(139,123,255,0.4); }
.tag.ysmart  { background: rgba(253,100,33,0.14); color: #fdba74; border-color: rgba(253,100,33,0.4); }
```

**Spinner / loading inline**:
```css
.spinner { width: 18px; height: 18px; border-radius: 50%; border: 2.5px solid rgba(255,255,255,0.2); border-top-color: #fff; animation: spin 0.7s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
.loading-inline { display: flex; align-items: center; gap: 0.6rem; color: var(--text-muted); font-size: 0.9rem; padding: var(--space-4); }
```

**Empty state**:
```css
.empty-state { display: flex; flex-direction: column; align-items: center; gap: var(--space-3); padding: var(--space-8); text-align: center; color: var(--text-muted); }
.empty-state .empty-icon {
  width: 56px; height: 56px; border-radius: 16px; display: grid; place-items: center;
  background: var(--surface-soft); border: 1px solid var(--panel-border); color: var(--text-faint);
}
.empty-state .empty-icon svg { width: 26px; height: 26px; }
```

**Custom scrollbar**:
```css
*::-webkit-scrollbar { width: 10px; height: 10px; }
*::-webkit-scrollbar-track { background: transparent; }
*::-webkit-scrollbar-thumb {
  background: rgba(148,163,184,0.25); border-radius: 99px;
  border: 2px solid transparent; background-clip: content-box;
}
*::-webkit-scrollbar-thumb:hover { background: rgba(148,163,184,0.4); background-clip: content-box; }
```

### 16. Login — split-screen layout

Left panel: rich brand content with multi-glow background. Right panel: centered login card.

```css
.login-stage { min-height: 100vh; display: grid; grid-template-columns: 1.15fr 1fr; }

/* Left panel — brand side */
.login-aside {
  position: relative; overflow: hidden;
  padding: clamp(2rem, 5vw, 4.5rem);
  display: flex; flex-direction: column; justify-content: space-between;
  border-right: 1px solid var(--panel-border);
}
.login-aside::before {
  content: ""; position: absolute; inset: 0;
  background:
    radial-gradient(circle at 20%  25%, rgba(26,102,255,0.4),  transparent 45%),
    radial-gradient(circle at 75%  80%, rgba(253,100,33,0.32), transparent 48%),
    radial-gradient(circle at 90%  15%, rgba(74,58,191,0.3),   transparent 40%);
  pointer-events: none;
}
.login-headline { font-size: clamp(2rem, 4vw, 3.1rem); font-weight: 720; letter-spacing: -0.03em; line-height: 1.05; max-width: 16ch; }
.login-headline b { background: var(--grad-full); -webkit-background-clip: text; background-clip: text; color: transparent; }

/* Right panel — login card */
.login-right { display: grid; place-items: center; padding: clamp(2rem, 5vw, 4rem); }
.login-card  { width: 100%; max-width: 400px; }

/* Google auth button */
.google-btn {
  display: flex; align-items: center; justify-content: center; gap: 0.7rem;
  width: 100%; padding: 0.8rem; border-radius: var(--radius-sm);
  background: #fff; color: #1f2433; font-weight: 600; font-size: 0.95rem;
  border: 1px solid rgba(255,255,255,0.3); cursor: pointer;
  transition: transform 150ms ease, box-shadow 150ms ease;
  box-shadow: 0 10px 30px rgba(0,0,0,0.3);
}
.google-btn:hover { transform: translateY(-1px); box-shadow: 0 14px 38px rgba(0,0,0,0.4); }
```

### 17. Responsive breakpoints

```css
@media (max-width: 1100px) {
  .kpi-grid { grid-template-columns: repeat(2, 1fr); }
  .grid-2   { grid-template-columns: 1fr; }
}
@media (max-width: 920px) {
  /* Sidebar becomes a mobile drawer */
  .app-shell { grid-template-columns: 1fr; }
  .sidebar {
    position: fixed; left: 0; top: 0; width: var(--sidebar-w);
    transform: translateX(-105%); transition: transform 0.28s cubic-bezier(0.16, 1, 0.3, 1);
    box-shadow: var(--shadow-glass);
  }
  .sidebar.open { transform: translateX(0); }
  .scrim { position: fixed; inset: 0; background: rgba(8,11,22,0.6); backdrop-filter: blur(3px); z-index: 19; }
  .login-aside { display: none; }
  .login-stage { grid-template-columns: 1fr; }
}
@media (max-width: 560px) {
  .kpi-grid  { grid-template-columns: 1fr; }
  .form-grid { grid-template-columns: 1fr; }
  .content   { padding: var(--space-5) var(--space-4) var(--space-6); }
}

/* Always respect reduced motion */
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: 0.001ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.001ms !important;
  }
}
```

## Visual Correction Checklist

When asked to fix or polish an existing screen, audit against this list and correct every miss:

1. **Background** — flat solid color? Replace with the theme's ambient 3-layer radial gradient stack + `background-attachment: fixed`.
2. **Palette drift** — off-brand hexes, muddy grays, default browser blues? Realign to brand tokens.
3. **Buttons** — no hover lift, no glow, abrupt color-only hover? Apply the full button system (pill shape, gradient, shadow, lift, filter, active/disabled states).
4. **Borders & shadows** — harsh `1px solid #ccc` + `box-shadow: 0 1px 2px`? Use `--panel-border`/`--panel-border-strong` and `--shadow-glass` (dark) or `--shadow-soft` (light).
5. **Spacing** — arbitrary paddings/margins? Normalize to `--space-*` tokens; consistent rhythm beats more whitespace.
6. **Typography hierarchy** — everything same size/weight? Establish title (weight 700), page-eyebrow (accent, uppercase, 0.7rem), body, and muted levels using `--text`/`--text-soft`/`--text-muted`/`--text-faint`.
7. **Radius consistency** — mixed 2px/4px/10px corners? Normalize to `--radius-sm/md/lg/xl`; pills (`999px`) for badges and dark-theme buttons.
8. **Semantic states** — raw red/green text for errors/success? Use pill badges with `.dot`, semantic soft+border patterns.
9. **Focus & a11y** — missing focus styles, contrast under 4.5:1? Add the global focus ring; fix contrast.
10. **Motion** — no transitions, or janky long ones? 140–150ms ease for hovers, spring `cubic-bezier(0.16,1,0.3,1)` for entrances (modals, toasts, popups, drawers), `prefers-reduced-motion` guard.
11. **Empty/loading states** — blank divs while loading? Add `.spinner`/`.loading-inline` or `.empty-state` with glass icon container.
12. **Icons** — emojis, mixed sets, inconsistent sizes, hardcoded colors? Migrate to Lucide per the Iconography norms.
13. **Sidebar active state** — plain highlight? Apply gradient background + blue glow box-shadow + lighter icon tint.
14. **KPI section missing** — dashboard with no KPI cards? Add the 4-column `.kpi-grid` with per-card radial glow.
15. **Tables** — basic HTML table? Apply `data-table` with sticky uppercase headers, subtle row hover, and `cell-*` modifier classes.
16. **Glass panels** — cards with white/flat background? Apply `.glass` or the `linear-gradient(155deg, …)` + `backdrop-filter: blur(22px)` pattern.

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
- **Spacing**: icon-to-label gap `0.4–0.5rem` (see `.btn` gap).
- **Icon-only buttons**: circular `.btn-icon` with border and hover state, and an `aria-label`.
- **NEVER** use emojis, mixed icon sets, or ad-hoc inline SVGs when a Lucide icon exists.

## Best Practices

### DO:
- Verify the project is on the latest stable Angular major (`ng version`) — upgrade with `ng update` if it's behind
- Use signals, zoneless change detection, and native control flow (`@if`/`@for`/`@defer`) in all new code
- Detect the project's styling approach (pure CSS tokens vs Tailwind 4) before writing styles
- Use existing tokens (`var(--yz-primary-1)`, `var(--space-4)`) — extend `palette.css` if one is missing
- Add the 3-layer radial-gradient ambient background + `background-attachment: fixed` (dark theme)
- Give buttons pill shape + gradient + glow + lift on hover
- Use `backdrop-filter: blur(…)` on sidebar, cards, modals for the glass effect
- Use spring `cubic-bezier(0.16, 1, 0.3, 1)` for entrance animations (modals, toasts, drawers)
- Use `:focus-visible` + `--focus-ring` token for accessibility
- Add `@media (prefers-reduced-motion: reduce)` for any animation
- Maintain WCAG contrast (4.5:1 for body text)
- Use Lucide for all UI icons, sized 16/20/24, colored via `currentColor`
- Use gradient text (`-webkit-background-clip: text`) on brand product names (wordmark, headlines)
- Add per-card radial glow via `--kpi-glow` CSS custom property on KPI cards
- Animate status dots with `pulse` keyframe for running/active states

### DON'T:
- Leave a project on an outdated Angular major — latest stable is mandatory
- Write `*ngIf`/`*ngFor`, `@Input()`/`@Output()` decorators, or NgModules in new code
- Imitate a legacy project's current visuals — correct them toward this standard
- Hardcode colors outside the palette
- Create `tailwind.config.js` in Tailwind 4 projects
- Write React/JSX examples — these projects are Angular
- Use pure black `#000000` backgrounds — use `#272a35`/`#1d2029` (dark) or the light gradient
- Put feature styles in shared files or tokens outside `palette.css`
- Use the full brand gradient on large surfaces — reserve it for highlights/CTAs, nav active states, and segmented toggle active item
- Use emojis as icons, mix icon libraries, or hardcode icon colors
- Skip `backdrop-filter` on glass panels — it's the core of the glass effect
- Use flat table headers — they must be sticky, uppercase, with `backdrop-filter: blur(8px)`
