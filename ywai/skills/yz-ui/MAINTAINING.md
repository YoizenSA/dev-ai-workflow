# Maintaining `yz-ui`

Read this **only when evolving the skill** (adding/changing a token, component, primitive, or norm). It is *not* loaded on normal skill triggers — keep it here, not in `SKILL.md`.

> **Theory:** this discipline is `writing-great-skills` applied to `yz-ui` — see `~/.claude/skills/writing-great-skills/` (+ its `GLOSSARY.md`) for the full vocabulary. In its terms: `SKILL.md` is the legible top of an **information hierarchy** whose deep CSS/TS is **progressively disclosed** to `assets/`; every meaning keeps a **single source of truth**; and pruning hunts **duplication**, **no-ops**, **sediment**, and **sprawl**. The terms in **bold** below are levers from that skill.

## Golden rule

`SKILL.md` is loaded into context every time the skill fires; `assets/*` are read on-demand (effectively free). So:

> **Assets can grow freely. `SKILL.md` must stay terse.**

Deep code (CSS/TS) lives **only** in `assets/` — one source of truth. `SKILL.md` is the scannable index of *rule → when → pointer*. **Never paste CSS into `SKILL.md`** (that is what bloated it before).

## Entry filter — before changing anything

Ask: **is this a reusable design norm, or a one-off specific to a project?** Only generalizable decisions go in the skill. A project-specific tweak stays in that project. This is the gate that stops "info así nomás" from leaking in.

## Route by layer (single source of truth)

| What changed | Where it goes | Touch `SKILL.md`? |
|---|---|---|
| A **token** (new/changed value) | `assets/palette.css` — edit **both** `:root` and `:root[data-theme="light"]` | Only if it's a new token *family* |
| **Component CSS** | the matching `assets/*.css` (or `cp` the whole file from the source project to re-sync) | Only if it introduces a new rule |
| A **behavior / primitive** | `assets/*.ts` (directive/component/service) | Only if it introduces a new rule |
| A **rule / norm** | `SKILL.md` — **one line of prose + pointer to the asset** | yes (tersely) |

## Anti-bloat discipline

1. **Edit, don't append.** If a rule already exists, refine it in place. If a pattern grows, compress it. (Fights **sediment** — stale layers that settle because adding feels safe.)
2. **Duplication test:** "is this already in Patterns / Checklist / Tech-Stack / Performance?" If yes, fold it in — don't add a new bullet. A pointer (`see #7`) is fine; a restated *meaning* is **duplication** and inflates its rank on the ladder.
3. **No-op test:** does the line change behaviour vs. the model's default? `be consistent`, `use good contrast`, `write clean CSS` are **no-ops** — the agent already does them. Keep only lines that *redirect*: a specific token, a counter-intuitive rule, a hard-won bug. A weak **leading word** (`be thorough`) is a no-op — swap it for a stronger one, don't delete the intent.
4. **Does it even need a `SKILL.md` line?** If the asset CSS already enforces it implicitly, the asset alone may be enough — no prose needed.
5. Reference the asset file by name; don't inline its contents.

## The loop while updating a consuming project

1. You change the project's design system (palette, a shared CSS, a primitive).
2. Generalizable? → `cp` the file into `assets/` here (re-sync verbatim for `palette.css`/`base.css`/component CSS).
3. New norm introduced? → add/edit **one** line in `SKILL.md`. Otherwise leave `SKILL.md` alone.
4. Verify (below).

## Verification (no runtime of its own)

**1. Token consistency — no orphan tokens** (every `var(--x)` consumed in the bundle is defined in `palette.css`):

```bash
cd ~/.claude/skills/yz-ui/assets
grep -oE '\-\-[a-z0-9-]+:' palette.css | tr -d ':' | sort -u > /tmp/defined.txt
grep -rhoE 'var\(--[a-z0-9-]+' base.css buttons.css forms.css table.css modal.css components.css shell.css \
  | sed 's/var(//' | sort -u > /tmp/used.txt
comm -23 /tmp/used.txt /tmp/defined.txt   # should list ONLY contextual/runtime vars
```

Expected "orphans" are **contextual vars set at runtime**, not palette tokens — these are fine:
`--glow-1/2/3` (set in `base.css` + light block), `--kpi-rgb`/`--kpi-icon-color`/`--modal-w`/`--yd-pop-maxh`/`--nav-active-a1/a2`/`--toast-dur` (set inline or by a directive). Anything else in the list is a real orphan — fix it.

**2. Dark + light** — confirm nothing is hardcoded that breaks in light; every colour flows from a token re-themed by `data-theme`.

**3. Smoke (optional)** — render `palette.css + base.css + components.css` (+ a few static `.btn`/`.pill`/`[data-tip]`/`.card`) in a scratch HTML, open it in dark and toggle `<html data-theme="light">`, screenshot both. Confirms the bundle paints standalone in both themes.

## Layer map (what's the design system vs what's swappable)

- **Mandatory, layout-agnostic:** `palette.css`, `base.css`, `buttons/forms/table/modal/components.css`, the primitives.
- **Reference, swappable:** `shell.css` (one nav/layout). Other apps reuse everything else and build their own chassis.
