# Signature Visual Patterns

Referencia de `yz-ui`. Aplicalos al construir o corregir UI. El checklist de
corrección en SKILL.md apunta acá por número. El CSS de cada uno vive en el
archivo de `assets/` que se nombra en el patrón.

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

**11. Themed selects & date pickers (never native).** Native `<select>`/`<input type=date>` show OS chrome that breaks the glass look. Use `assets/yd-select.component.ts` (signal select: glass popover, search above a threshold, `tags` mode) and `assets/yd-date.component.ts` (themed calendar). Both position via `assets/yd-anchored.directive.ts` (see #8). Adapt the components' I/O but keep the markup contract (`.yd-select` / `.yd-cal`) so `assets/forms.css` + `components.css` apply. **Selected option (`.yd-select-opt.sel`) is minimal:** a faint brand-blue fill + a `✓` — no side bar, no glow (an over-decorated selected row reads loud, not premium).

- **Calendar header = dropdown-caption (shadcn / Google Calendar), NOT a "title → month-grid → year-block-grid" jump** — that progressive grid reads *rebuscado* and got rejected by a real user twice. `‹ ›` move the month one step; **Month and Year are themed dropdowns of their own** (same glass language as `yd-select`), the year a scrollable list centred on the view year (`scrollIntoView`). **Never a native `<select>` for month/year** — its OS list shatters the glass (rejected on sight). Keep it sober: the month/year triggers need **no chevron** (hover signals they're clickable) and "Hoy" needs **no icon** (a `circle-dot` reads like an archery target).
- **Nested popovers:** the month/year dropdowns live inside the calendar's DOM with a local `picker` signal and do **NOT** go through `PopoverService` (that would close the calendar containing them). Every inner click `stopPropagation`; Esc closes the open dropdown first, then the calendar.
- **Popover direction is a product call.** Default flips up when it doesn't fit below; some products want **always-down** (a flip-up that covers the trigger reads worse than spilling under the edge). `yd-anchored` can force down — confirm the preference per app.

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

**14. Side-by-side diff (`yd-diff`, `assets/diff.ts` + `assets/yd-diff.component.ts`, CSS `.diff-split` in `components.css`).** For showing differences (config YAML, audit changes) GitHub-style: two columns (antes | después) with line numbers and a red/green per-line background. The math lives in `diff.ts` — `unifiedToRows()` for a backend-parsed diff (added/removed/context), `diffTexts()` = an LCS between two raw strings (audit's old/new value); the component only renders `rows`. Hard-won rules:

- **Grid `auto minmax(0,1fr) auto minmax(0,1fr)`** — the `minmax(0,…)` is the trick: it lets a cell shrink **below** its content width. Without it a long line forces its column wide and **collapses the opposite column** (you see one column + a red sliver) plus horizontal scroll.
- **Long lines → `white-space: pre-wrap; overflow-wrap: anywhere`** so a long URL wraps inside its column instead of overflowing. Side-by-side + wrap reads better than per-pane horizontal scroll.
- **Memoize the rows** per file/change (`WeakMap`) — the LCS is O(m·n); don't recompute it on every change-detection pass.
- Widen the modal (~1100px clients, ~820px audit). Mark unchanged files with a **"Sin cambios"** badge so the user sees the full picture before pushing.
