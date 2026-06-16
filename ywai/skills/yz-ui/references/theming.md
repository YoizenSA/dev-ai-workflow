# Theming — light variant + bootstrap

Read this when implementing the light theme or wiring the theme toggle.
The dark theme is the default; everything below is about the light variant and
how the toggle is bootstrapped. The tokens themselves live in `palette.css`
(the `:root[data-theme="light"]` block) and the glow alphas in `base.css`.

## Bootstrap (no flash, covers login too)

Drive the theme with `<html data-theme="light">`, applied **before first paint**
so there's no flash and the login/landing also themes. A saved preference wins;
otherwise honour the OS (`prefers-color-scheme`). Persist on toggle.

`main.ts` (before `bootstrapApplication`):

```ts
const savedTheme = localStorage.getItem('yd-theme');
const prefersLight = window.matchMedia?.('(prefers-color-scheme: light)').matches;
if (savedTheme === 'light' || (!savedTheme && prefersLight)) {
  document.documentElement.setAttribute('data-theme', 'light');
}
```

`index.html` (variable fonts — real intermediate weights, one file):

```html
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400..800&family=JetBrains+Mono:wght@400;500;600&display=swap" rel="stylesheet">
```

Own the theme in a single `ThemeService` (see `assets/angular/theme.service.ts`):
the `theme` signal + `toggle()` that runs the View-Transitions reveal + the
`localStorage`/`<html>` write. Inject it wherever a toggle lives — don't bury
the logic in the shell component. **Put a toggle on the login/landing too**, not
only inside the authenticated shell: a user whose OS theme differs from their
preference (and who has no saved choice) is otherwise stuck on the wrong theme
until they log in. Because the state lives on `<html>` + `localStorage`, a choice
made pre-auth survives the login navigation and reloads for free.

## The inversions that matter (skip these and it looks cheap)

A light theme is not "invert the colors" — it's redefining tokens under a scope
with a few deliberate inversions. Because every color flows from a token, the app
re-skins from one place.

1. **Surfaces: dark→light with near-flat gradients.** Dark cards run a high-luminance-delta gradient (`--surf-1` → `--surf-3`) that reads elegant; the *same delta* in light looks like a dirty band. In light, make the surface gradient almost flat (`#fff` → `#f6f8fc`, ~1-2%) and let a **shadow** carry the depth.
2. **Depth from shadow, not glow.** Replace the dark glass shadow with a *material* layered shadow — a tight contact shadow + a soft ambient one (`0 1px 3px rgba(15,23,42,0.07), 0 10px 28px rgba(15,23,42,0.10)`). Set the page background a touch grey so white cards lift off it.
3. **Text tints: pastel→saturated-dark.** The bright pastels (`--tint-*`) are legible on dark glass but disappear on white. Flip each to the saturated-dark of its hue (`#047857` success, `#b91c1c` danger, `#1d4ed8` info/primary, `#c2410c` accent, `#6d28d9` purple).
4. **Wordmark gradient inverts.** Dark uses the luminous `--grad-text`; on light it washes out. Point `--grad-text` at the **institutional saturated** gradient (`--grad-full`) — it reads on white.
5. **Overlays flip via channel tokens.** `--surface-soft`/`--surface-hover` are `rgba(var(--white-rgb), …)`; set `--white-rgb` to a dark slate in light so they *darken* instead of lighten. Same for `--slate-rgb` (borders need more presence on white).
6. **Ambient glows: present, not invisible.** The fixed radial glow stack at full dark strength looks like saturated blotches on white. Tokenize each layer's alpha (`--glow-1/2/3`) and drop them to ~0.10-0.12 in light — a warm tint, not a stain. Don't kill them to 0.05 or the page goes flat and lifeless.
7. **Semantic fills need more alpha on light.** A translucent tint reads *paler* over white than over black, so a `*-soft` background that pops on dark looks washed out on light. Bump the alpha so alerts keep their punch. **Green is the worst offender** — green/white has less luminance contrast than red or amber, so the success pill washes out first; give it the highest fill+border of the set. And once the green fill is stronger, its **status dot** (the bright `--success`) blends into it — on light, darken the dot to the saturated tint (`:root[data-theme="light"] .pill-success .dot { background: var(--tint-success) }`) so it stays visible.
8. **Native controls**: set `color-scheme: light` in the light scope; warning yellow needs a dark text token (the bright yellow is invisible on white).
9. **Card colour identity from ONE token, and light lifts with a *shadow* not a glow.** Don't scatter separate glow / icon-bg / icon-color vars on a KPI card — give it a single `--kpi-rgb` (the hue's rgb triple) and compose everything from it at different alphas: tinted border (~0.22), a soft corner wash (~0.13), and a gradient + glow icon chip (~0.30→0.12). That's what makes 4 KPI cards read as distinct colours instead of grey twins. A translucent wash reads heavier on white but still won't lift the card off the page — on light add a **material shadow** (`box-shadow: var(--shadow-glass)`); a dark-only radial glow leaves light cards flat.
10. **The modal scrim must NOT use a surface token.** A `.overlay` backed by `rgba(var(--surf-8-rgb), …)` dims fine on dark but flips to a pale wash on light and stops dimming — the modal floats with no focus. Use a fixed dark scrim, `rgba(var(--black-rgb), 0.55)`, in both themes (and don't flip `--black-rgb`).
11. **Animate the switch — don't hard-cut.** Flipping `data-theme` snaps every token at once, which reads cheap. Wrap the token swap in the **View Transitions API** and drive a *circular reveal* from the toggle button: `document.startViewTransition(applyTheme)`, then on `.ready` animate `::view-transition-new(root)`'s `clip-path` from `circle(0 at <x>px <y>px)` to `circle(<R>px at …)` where `x/y` is the click point (fallback: button center, for keyboard) and `R` reaches the farthest corner — `Math.hypot(Math.max(x, vw-x), Math.max(y, vh-y))` — over ~450ms. In CSS, kill the default crossfade so only the wipe shows: `::view-transition-old(root), ::view-transition-new(root) { animation: none; mix-blend-mode: normal; }`. The new theme paints above the old, so the circle reveals the *incoming* theme over the outgoing one. **Always feature-detect** (`if (!document.startViewTransition || matchMedia('(prefers-reduced-motion: reduce)').matches)` → apply instantly). The full implementation is `assets/angular/theme.service.ts`.
12. **Input fills need a "well" on light.** A field at `rgba(white, .6)` is invisible white-on-white inside white cards. Tokenize the fill (`--input-bg`) and give light a subtle grey (`#eef2f8`) so the field reads as a recessed well, not a borderless ghost.
13. **The accent micro-label (page eyebrow) hits the AA edge on light.** Bright orange (`--yz-accent`) over white is borderline; switch the eyebrow to the deeper `--tint-accent` on light. (Same idea as the warning hue: bright-on-dark colours often need the saturated-dark tint on light.)
14. **Spinners/loaders inherit `currentColor`, never hardcode white.** A `border-top-color: #fff` spinner vanishes on any light-surfaced button (e.g. a white OAuth button) — in *both* themes. Build it from `currentColor` (`border: 2.5px solid color-mix(in srgb, currentColor 25%, transparent); border-top-color: currentColor`).
15. **The scrollbar corner + textarea resize grip — the ugly white square.** A resizable `<textarea>` that also scrolls shows a white block in the bottom-right corner in *both* themes: the browser's default `::-webkit-scrollbar-corner` (white) + the grey `::-webkit-resizer` grip. Style both globally once: `*::-webkit-scrollbar-corner { background: transparent; }` and `textarea::-webkit-resizer { background: transparent; }`. (Both already shipped in `base.css`/`forms.css`.)
