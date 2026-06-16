# Performance & production baseline

Read this when scaffolding a new project or doing a perf pass. Non-negotiable
for new Yoizen products (Angular-specific where noted; the principles port).

## Angular / rendering

- **Zoneless change detection** (`provideZonelessChangeDetection`) + `ChangeDetectionStrategy.OnPush` on every component; signals as the only reactive surface.
- **Lazy `loadComponent` per route** + `withPreloading(PreloadAllModules)` so chunks download in idle and first navigation is instant. Target: initial transfer < 100 KB.
- **Derived state is `computed()`, never a getter/method called from the template** — template-invoked methods re-run on every render; ones that allocate (`.map`/`.filter` building arrays) are the worst. If the input is a plain object (not a signal), memoize with a `WeakMap`.
- **Timers are cancellable**: anything `setTimeout`-driven (toasts, debounce) stores the handle and clears it on manual dismiss/destroy.
- **Every list fetch loops pages until exhausted or paginates server-side** — a silent fixed `limit: 100` shows an incomplete catalog with no error. (See the pagination lesson in `tables.md`.)

## CSS / paint

- **Ambient background goes on a `body::before { position: fixed; inset: 0; z-index: -1 }` layer**, never `background-attachment: fixed` (full-page repaint on every scroll in Chrome). Already in `base.css`.
- `backdrop-filter: blur()` is the costliest effect in the theme: never nest blurred surfaces, keep it off large scrolling tables.
- **Fonts**: variable range (`Inter:wght@400..800`) — the theme uses intermediate weights (550/650/720) that static cuts fake via synthesis; one file, real weights.

## Production-grade polish (what takes it from "works" to "reference")

Bake these in from the start, not as a polish pass.

1. **Tokenize everything — never scatter literals.** Ad-hoc sizes (0.78/0.82/0.86/0.9rem sprinkled everywhere) are the #1 tell of an un-systematic UI. Define one modular scale (`--text-2xs … --text-3xl`) and have every component consume a step. Same discipline for motion (`--t-*`), z-index (`--z-*`) and `--disabled-opacity`. A third-party brand's exact colors (e.g. an OAuth button) are the only sanctioned literal exception.
2. **Skeleton loaders, not empty flashes.** A screen that `await`s data and renders its empty state (or a blank table) until it arrives "jumps" and reads amateur. While loading, render shimmer skeletons that mirror the layout: the `.skeleton` primitive (`.skel-line` rows inside the real `<tr>`/card structure), gated by a `loading` flag: `@if (loading()) { skeleton rows } @else { data / @empty }`. It honours `prefers-reduced-motion`.
3. **Accessibility is part of "professional", not an extra.**
   - **Every icon-only button needs `aria-label`** (mirror the `data-tip` text).
   - **Honour `prefers-color-scheme` on first load.** A saved preference wins; otherwise read `matchMedia('(prefers-color-scheme: light)')`. Set the attribute before first paint (no flash). See `theming.md`.
   - **Keep `--text-faint` near AA.** It's the auxiliary-text level (help, counts, subtitles) used in dozens of places; if it dips below ~4.5:1 the whole secondary layer reads weak. Tune per theme (lighter on dark, darker on light) while staying below `--text-muted`.
   - Global `:focus-visible` ring (`--focus-ring`) is in `base.css`; keep it intact. WCAG 4.5:1 minimum for text.
