# Tooltips, themed dropdowns & confirmation modals

Read this when adding tooltips, using `yd-select`/`yd-date` inside modals, or
building a delete/confirmation dialog. The CSS ships in `components.css`/
`modal.css`; the Angular references are in `assets/angular/`.

## Tooltips (`data-tip`) — never the native `title`

The native `title` attribute renders the OS white box and breaks the dark glass
look. The theme ships a CSS-only tooltip in `components.css`:

```html
<button class="icon-act" data-tip="Eliminar" aria-label="Eliminar">…</button>
<!-- Angular, conditional: [attr.data-tip]="cond ? text : null" -->
<span class="pill pill-danger" [attr.data-tip]="error ?? null">Falló</span>
```

- **Strict rule: zero native hover text.** Audit with `grep -rn 'title=' src/` (excluding `aria-*` and `<title>`) before shipping — every match becomes `data-tip`. The white OS tooltip box anywhere is a bug.
- Side placement for sidebars/rails: add `data-tip-pos="right"`. In collapsed sidebars bind conditionally: `[attr.data-tip]="collapsed() ? label : null"` (no tooltip when the label is already visible).
- **A tooltip must never alter layout or trigger scrollbars.** Absolutely-positioned tips still contribute to the scrollable overflow of `overflow:auto` ancestors — a centered tip on the LAST table column spawns a horizontal scrollbar on hover. The bundle auto-flips right-aligned action clusters to the left. **This isn't only tables**: the same trap hits any `[data-tip]` inside a *scrollable list/panel* (`overflow-y:auto` forces `overflow-x:auto`), so a trash icon's tip in a scrolling sidebar list "breaks" layout with a phantom scrollbar. Fix both sides: open the tip inward (`data-tip-pos="left"`) **and** put `overflow-x: clip` on the container. **The auto-flip is a single rule that enumerates EVERY right-aligned action cluster** (`.row-actions, .fr-acts, .slot-acts, .ver-acts …`) — add each new cluster to that one rule (in `components.css`), never chase them per-button. Audit: `grep -roE 'class="[a-z-]*-acts?"'`.
- Don't tooltip plain readable text (paths, URLs, descriptions that already fit) — hover noise. Reserve tips for icon-only controls, genuinely-truncated content and status pills with hidden detail. **A `max-width + ellipsis` element is only *potentially* truncated — gate its `data-tip` on length** (`[attr.data-tip]="val.length > N ? val : null"`, with `N` ≈ the `ch` cap), never bind unconditionally, or every row whose text fits flashes a redundant bubble on hover.
- Long texts wrap (`max-width: 360px`); short ones hug content (`width: max-content`). Parents with `overflow` (e.g. `.table-wrap`) clip tooltips — if the table fits without scroll, set `overflow: visible` on that wrapper (but see the containment rule in `tables.md` — containment wins when the table doesn't fit).
- **Every icon-only button also needs `aria-label`.** A visual `data-tip` is invisible to screen readers — they announce a bare "button". Mirror the tip text into `aria-label`.

## Themed dropdowns (`yd-select` / `yd-date`) inside modals

Native `<select>`/`<input type=date>` can't match the theme. Use the themed
custom controls (`assets/angular/yd-select.component.ts`, `yd-date.component.ts`)
— port the pattern to your framework. **Strict: `grep -rn '<select' src/` must
return zero app usages.** `yd-select` supports `[disabled]`, a `[label]` prefix
for inline filters, and shows a search box automatically past ~7 options.

Three traps, all solved by the shipped pieces — use them, don't re-derive:

- **`modal-popovers`**: ANY modal containing a `yd-select`/`yd-date` needs `class="modal modal-popovers"` or the dropdown gets clipped by the modal-body overflow and forces inner scroll. QA: `grep -n 'class="modal"' src/ -r` and check each hit whose body renders a themed dropdown.
- **Never `stopPropagation` on the modal panel** — it silently breaks every popover's close-on-outside. `yd-select`/`yd-date` listen on `document`; a `(click)="$event.stopPropagation()"` on the modal panel never lets the click reach `document`, so opening a second dropdown leaves the first one open (two calendars at once). Don't stop propagation — guard the backdrop instead: close only when `event.target === event.currentTarget` (the click landed on the overlay itself, not bubbled from inside).
- **Docked dropdowns must flip + clamp to the viewport, not just open downward.** A menu hard-pinned `top: calc(100% + 6px)` always opens *down* and never measures free space, so in a short, vertically-centred modal it grows until it kisses the window bottom. Fix once with the shared `ydAnchored` directive (`assets/angular/yd-anchored.directive.ts`) on the `.yd-pop`: on open and on resize/scroll (rAF-throttled) it reads the host's rect, **flips** the menu up (`.yd-menu-up`) when it doesn't fit below but fits above, and **clamps** the height via `--yd-pop-maxh` (the list scrolls inside) to keep a ~14px margin off the edge. **Always measure against the viewport, never the modal** — `modal-popovers` exists precisely so the popover can escape the modal and float over the dimmed overlay. The calendar is the worst case (fixed height, ignores the clamp, only takes the flip), so **top-anchor any modal that holds a tall popover** (`.overlay:has(yd-date) { align-items: start; padding-top: clamp(…) }`) — already in `modal.css`. Mental model: `modal-popovers` stops the *clip*, `ydAnchored` stops the *spill* — you need both. Keep the menu `position: absolute` (it tracks the trigger as the modal scrolls); don't switch to `fixed`. Gotcha: the scroll listener is `capture: true`, so ignore scroll events originating inside the popover (`el.contains(e.target)`) or the inner-list scroll resets `scrollTop` to 0 and you can't reach the last option.
- **Themed date picker — empty padding cells**: leading blank cells carry `iso: ''`; a naive `[class.sel]="c.iso === value()"` marks them selected when `value()` is also `''`, painting random brand-gradient blocks. Guard it (`!c.muted && c.iso === value()`). Keep nav arrows light; footer asymmetric: "Borrar" neutral, "Hoy" the accent action.
- **Custom-element hosts are inline**: Angular hosts like `<yd-select>` default to `display:inline`, which baseline-misaligns inside flex rows. Set `display:block` (or `width:max-content` for compact filters) on the host.

## Destructive actions & confirmation modals

A delete dialog is where a design system proves it's serious. Match the
**friction to the blast radius** — and never centre it.

- **Friction scales with impact, not uniformly.** Tier the confirmation:
  - **High-impact / irreversible** (deleting a resource, connection, account): **type-to-confirm** — the user types the object's exact name into an input and the delete button stays `[disabled]` until it matches. This defeats the reflexive "yes, delete" click.
  - **Frequent, in-flow edits** (removing a field/row while building something): a **lightweight one-button confirm**, no typing. Forcing someone to type a name every time they drop a row is friction they'll come to hate.
  - **Soft-delete / recoverable**: you *may* still type-to-confirm if the object matters, but keep the copy **honest** — don't say "irreversible" for something recoverable; say what actually happens ("stops being managed", "kept as a logical delete").
- **Confirmation modals are LEFT-aligned, never centred.** A centred title over centred multi-line prose reads generic and hurts legibility (the eye loses the line-start on every wrap). Head left (title + muted subtitle, close button top-right *inside* the head), body left, actions bottom-right.
- **Structure**: title → muted subtitle ("Destructive and irreversible action") → a `.alert-warning` stating the consequence in one sentence → *(type-to-confirm only)* a `.confirm-label` + input → footer with a ghost "Cancel" and a `.btn-danger-solid` gated by an exact-match check.
- **The object's name inside the alert blends into the colour — chip it.** A bare mono name in a coloured alert inherits the alert's hue and melts into the prose. Render it as `.del-name`, a **solid code chip** (surface fill + max-contrast text + border + mono). Token-driven → it contrasts in both themes.
- **The "type X to confirm" line is NOT a `field-label`.** The uppercase, letter-spaced, muted field-label flattens the embedded name to the same grey. Use `.confirm-label` (case-normal, legible) and drop the **same** `.del-name` chip into it.
- **Reset the confirm input when the modal opens.** Clear the bound signal in the open handler (`open(x) { confirmText.set(''); target.set(x); }`) or leftover text pre-enables the button. Gate on `confirmText().trim() === target.name`, and let Enter submit when it matches.
- **The scrim stays a fixed dark token in both themes** (`rgba(var(--black-rgb),0.55)`) so the dialog always holds focus.
- **Modal scroll architecture**: `.modal` is `display:flex; flex-direction:column; overflow:hidden`; head/foot are `flex-shrink:0` and the `.modal-body` scrolls (`overflow-y:auto; min-height:0`). This keeps the scroll inside the rounded border and pins the head/foot. (Already in `modal.css`.)
