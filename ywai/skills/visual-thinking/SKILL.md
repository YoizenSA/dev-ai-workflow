---
name: visual-thinking
description: "Turn an idea, change or decision into a visual HTML canvas with Mermaid diagrams. Trigger: visualizá esto, canvas."
---

# Visual Thinking

One job: when a discussion outgrows chat text, materialize it. A single self-contained HTML canvas — Mermaid diagrams, export buttons, no build step — that the user opens in the browser while the conversation continues.

The canvas is a thinking surface, not a deliverable. It holds the current shape of an idea: what is settled, what is open, what competes with what.

## Rules

1. Render when the discussion has >=3 moving parts (options, unknowns, flows, decisions). A single small question stays in chat.
2. One diagram, one point. If a diagram needs a paragraph, redraw the diagram.
3. 2–5 blocks per canvas, picked to fit the discussion. Never one of each type.
4. Use the user's terms and the domain language of `AGENTS.md`. Never rename entities on the canvas.
5. Nothing lands in the repo. OS temp dir, open it, report the absolute path.
6. The canvas is a snapshot, not the record. Regenerate as decisions land; ADRs and `AGENTS.md` stay the source of truth.

## Process

### 1. Extract

Compress the discussion into its skeleton: topic, what is settled, what is open, options in play, decisions and their dependencies, flows. If the canvas restates everything already said in chat, it failed.

### 2. Compose

Pick blocks from [references/diagrams.md](references/diagrams.md):

- **Idea map** — the overview: parts and how they relate.
- **Before / After** — current state vs proposed state, side by side.
- **Options** — alternatives in competition, trade-offs, one recommendation.
- **Decision tree** — what is settled, what is open, what unlocks what.
- **Sequence** — who calls whom, in what order, how many round-trips.
- **Code sketch** — the shape of the change in a few lines of code.

### 3. Render

Fill [assets/canvas-template.html](assets/canvas-template.html) following the contract in [references/diagrams.md](references/diagrams.md). Write to `<tmpdir>/visual-thinking-<timestamp>.html` (`%TEMP%` on Windows, `$TMPDIR` or `/tmp` elsewhere), open it (`start` / `open` / `xdg-open`), tell the user the absolute path.

### 4. Feed back

Ask which branch to work next. When a decision lands, offer to re-render with a fresh timestamp — never edit diagrams in place. If the plan is worth stress-testing, offer the grilling skill. If a rejected option carries a load-bearing reason, offer an ADR.
