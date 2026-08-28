# Theming — tokens, temas y materiales

Referencia de `yz-ui`. Las reglas cortas están en SKILL.md; acá vive el porqué
y el detalle que se consulta al construir o corregir la paleta.

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
