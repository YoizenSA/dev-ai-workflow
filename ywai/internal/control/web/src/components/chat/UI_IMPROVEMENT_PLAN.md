# Chat UI improvement plan — aligning with the yz-ui design system

Goal: raise the chat UI to the Yoizen design-system (yz-ui) standard — better use
of space, glass surfaces, ambient depth, consistent tokens, accessible tooltips
— without changing behavior.

## Scope & ground rules

- **Stack:** this chat is **React** (`Chat.tsx` + `Chat.css`). yz-ui's *Angular*
  stack norms (signals, standalone, `yd-*` primitives) do **not** apply here.
  What **does** apply and is mandatory: the **design tokens, palette, glass
  hierarchy, spacing scale, iconography, and the visual patterns** in
  `src/styles/theme/*`.
- **The design system is already present** in the project: `palette.css`
  (tokens, dark+light), `base.css` (ambient background `body::before`, global
  `:focus-visible` ring, `.glass`, tooltips), `components.css`, etc. The chat
  currently **under-uses** it. This plan is about *consuming* what already
  exists, not inventing new styles.
- **Token-first:** every value below references an existing token
  (`--space-*`, `--radius-*`, `--shadow-glass`, `--surface*`, `--text*`,
  `--panel-border*`, `--yz-accent`, semantic `-soft`/`-border`). **No raw hex,
  no arbitrary px** for color/shadow/spacing.

---

## 1. Audit against the yz-ui 18-point checklist

| # | yz-ui rule | Current chat state | Fix |
|---|---|---|---|
| 1 | Never flat background | `.chat-container` paints an opaque `var(--surface)`, **hiding the ambient glow** from `body::before` | Let the ambient bg show: use `transparent`/`--surface-soft` glass panels over it, not an opaque fill |
| 4 | Glass surfaces + `--shadow-glass`, no raw/inline shadows | `.templates-menu` uses a raw `rgba(0,0,0,0.28)` shadow; sidebar/composer are flat `--surface-soft` with no glass | Apply `.glass` language: `backdrop-filter` (+`-webkit-`), `--panel-border`, `--shadow-glass`, `--glass-highlight` on cards/popovers |
| 5 | Radii from tokens; pills for badges | Mostly OK; `--radius-lg, 16px` fallback is fine | Keep; drop the px fallback (token exists) |
| 6 | Type hierarchy via `--text-*` scale | Font sizes are hardcoded (`14.5px`, `13px`, `12px`, `20px`) | Map to `--text-sm/base/md/lg/xl` + `--text-soft/muted/faint` |
| 7 | Don't flash controls on load | Selects/buttons OK | Keep filters interactive during send; already fine |
| 9 | Contrast ≥ AA both themes | Not verified for the new blocks (thinking/tool/subagent) | Verify `--text-muted`/`--text-faint` on `--surface-soft` in **light** theme |
| 10 | Named transitions, guard motion | `typing`/`pulse` keyframes lack `prefers-reduced-motion` guard | Wrap animations in `@media (prefers-reduced-motion: no-preference)` |
| 12 | Lucide sizes 16/20/24 only | Mixed 13/14/15/16/18/36/40 | Normalize to **16** (inline), **20** (buttons/nav), **24** (headers/empty). Drop 13/14/15/18 |
| 13 | (tables) n/a | — | — |
| 16 | `[data-tip]` tooltips, not native `title` | Everything uses native `title=` (pins, model, send, status dot, workspace, subagents) | Swap to `[data-tip]` (CSS tooltip in `base.css`/`components.css`) + keep `aria-label` on icon-only buttons |
| 4/glass | Spacing from `--space-*` | Several literals: `9px`, `7px`, `5px 12px`, `1px 5px`, `8px` | Replace with `--space-1..3`; keep sub-token nudges only where a token doesn't exist |

Not currently violated: focus ring (inherited globally), off-brand hex (uses
`--yz-accent`), semantic alerts (tool/error use `--danger`).

---

## 2. Better use of space (the core ask)

The current layout is functional but cramped and uneven. Concrete changes:

### 2.1 Conversation column rhythm
- Messages are capped at `max-width: 760px` centered — **keep**, it's the right
  reading measure. But vertical rhythm is tight: bump inter-message gap to
  `--space-5`, and give each message block more breathing room
  (`padding-block` via `--space-2`).
- **Group turns visually:** add a subtle hairline (`--panel-border` at low alpha)
  or extra `--space-4` between a user turn and the next assistant turn, so the
  eye parses exchanges. Today all messages are equidistant.
- Assistant avatar + body: align body text baseline to the role label; increase
  `message-body` line-height to the token used for reading (`1.65` → keep, but
  set via a `--leading` if we add one).

### 2.2 Sidebar density
- Width `260px` is fine on desktop; make it **collapsible** (rail pattern #9) so
  the conversation gets full width on demand — big spatial win on laptops.
- Workspace switcher + session groups: tighten group headers
  (`--space-2` top, `--space-1` bottom) and let session rows breathe
  (`--space-2` block). Right now the star/time/title compete; give the title
  `flex: 1` (done) and reveal time only on hover to reduce noise.
- Pinned group: add a small "Pinned" section label above the first pinned item
  instead of relying only on order.

### 2.3 Composer
- The composer card is the primary action — give it presence: `.glass` surface,
  `--shadow-glass`, and a max-width matching the conversation (`760px`, done).
- The toolbar row (agent/model/templates/send) is cramped. Use
  `--space-2` gaps, push send to the right (done), and cap the two selects with
  a shared max-width so long model ids don't shove the layout.
- Add generous `padding` (`--space-3`) and let the textarea auto-grow smoothly
  (done) up to ~200px.

### 2.4 Header & subagents strip
- The header is nearly empty now (just title + status dot). Reclaim it: move the
  **context-usage meter** here later (see parity plan), and make the title a
  real focal point with `--text-md`/600.
- The subagents strip is a good pattern; give it glass treatment and align its
  chips to the conversation's left edge (`--space-5`) so it doesn't float.

### 2.5 Empty / welcome states
- `chat-placeholder` is centered — good. Increase the suggestion grid gap to
  `--space-3`, cap width ~520px (done), and give the cards `.glass` + hover lift
  (`translateY(-1px)`, `@media (hover:hover)`) per button pattern #2.
- Use a 24px Lucide icon (header size) for the placeholder, not 36/40.

---

## 3. Component-by-component changes

### Sidebar (`.chat-sessions`)
- `.glass` panel over the ambient bg (don't paint opaque).
- Collapsible rail (icons + `[data-tip]` labels on the right).
- Group headers: uppercase `--text-2xs`, `--text-faint`, tighter.
- Session row: `[data-tip]` on the pin button; time shown on hover.

### Header (`.chat-header`)
- Title `--text-md` / 600; status dot keeps semantic colors (`--success` on,
  `--text-faint` off) but gets a `[data-tip]`.

### Messages (`.chat-message`)
- Avatars: keep 32px, but the inner icon → 18 (already) → **20** to match nav.
- Markdown: code blocks use `--surface-soft` + `--panel-border` (done); add
  syntax highlighting later (parity plan P1.2).

### Part blocks (thinking / tool / subagent)
- Unify as glass sub-cards: `--surface-soft` + `--panel-border` + `--radius-md`
  (done) — add `--shadow-glass` on open state for lift.
- Tool status pill: already semantic; ensure `running`→`--yz-accent`,
  `completed`→`--success`, `error`→`--danger` read AA in light.
- Chevron rotates (done); add `prefers-reduced-motion` guard.

### Composer (`.composer-card`)
- `.glass` + `--shadow-glass`; focus-within ring already uses `--yz-primary-2`.
- Selects → pill style with `--input-bg`/`--panel-border`; `[data-tip]` labels.
- Send/templates icons → 20; `[data-tip]` + `aria-label`.

### Templates popover / autocomplete
- `.glass` popover, `--shadow-glass` (replace the raw shadow), `--radius-md`.

---

## 4. Responsive & mobile (currently desktop-only)

- Below ~760px: collapse the sidebar into an off-canvas **drawer** with a FAB
  (☰) and a blurred scrim (shell pattern #12). The conversation takes full width.
- Composer: full-width, selects wrap to a second line if needed.
- Message padding tightens (`--space-3`), suggestion grid → 1 column.
- Guard all hover lifts with `@media (hover: hover)`.

---

## 5. Accessibility pass

- Every icon-only button: `aria-label` **and** `[data-tip]` (pins, send,
  templates, delete, status).
- Tooltips are reinforcement only — never the sole carrier of info (rule #16).
- Verify AA contrast for thinking/tool/subagent text in **both** themes.
- Motion: guard `typing`, `pulse`, chevron, hover lifts behind
  `prefers-reduced-motion`.
- Keep focus order intact when the sidebar collapses.

---

## 6. Prioritized rollout

1. **Token & tooltip pass (S)** — swap hardcoded sizes/spacing to `--space-*` /
   `--text-*`; native `title` → `[data-tip]`; normalize Lucide to 16/20/24;
   guard animations. *Pure cleanup, no layout risk, biggest consistency win.*
2. **Glass & depth (S–M)** — sidebar/composer/popovers to `.glass` +
   `--shadow-glass`; stop painting the container opaque so the ambient glow
   shows. *This is the single most visible "premium" jump.*
3. **Spacing & rhythm (M)** — turn grouping, sidebar density, composer breathing
   room, empty-state polish.
4. **Collapsible sidebar + mobile drawer (M)** — the real "use space better"
   payoff on small screens.
5. **Contrast/motion audit (S)** — verify both themes, add reduced-motion guards.

Each step is independently shippable and behavior-preserving. Start with 1–2:
they touch only CSS/attributes, carry near-zero risk, and deliver most of the
visual upgrade.

---

## Appendix — tokens available (from `src/styles/theme/palette.css`)

Spacing `--space-1..10` · radius `--radius-sm/md/lg/xl` · type
`--text-2xs..3xl` + `--text/-soft/-muted/-faint` · surfaces
`--surface/-soft/-hover/-strong` · borders `--panel-border/-strong` ·
brand `--yz-primary-1/2`, `--yz-accent`, `--yz-yellow` · semantic
`--success/-danger/-warning/-info` (+ `-soft`/`-border`) · effects
`--shadow-glass`, `--shadow-lift`, `--focus-ring`, `--grad-brand/-accent/-text`.
Ambient background, `.glass`, global focus ring, and `[data-tip]` tooltips are
already defined in `base.css` / `components.css` — **consume them.**
