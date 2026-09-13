# Report — HTML contract

Render `../assets/report-template.html`. It is self-contained (inline CSS, no build step). One CDN only: `html2canvas` for PNG export. PDF uses the browser print pipeline, so it works offline.

## Fill these slots (exact ids)

- `#meta-scope`, `#meta-window`, `#meta-date` — header.
- `#kpi-sessions`, `#kpi-skills`, `#kpi-cost`, `#kpi-hitrate` — KPI cards.
- `#tbl-agents`, `#tbl-skills`, `#tbl-tools` — `Name / Count / Share / Sessions|Cost` rows.
- `#frictions` — one `<article class="card">` per finding:
  title / badge (`Strong|Worth exploring|Speculative`) / lever pill (`skill|agent|tool|code`) / files (`mono`) / evidence list (3 sessions + metrics) / hypothesis (1 sentence) / proposal (1 sentence) / experiment + success metric.
- `#mem-misses` — top 5 `misses[]` as `prompt snippet → top_result`.
- `#next` — the single experiment for next week + metric.

## Output rules

- One primary window. If you widened (thin-window rule), the title, KPIs and tables all use the widened window.
- Freshness first: if the newest tracked session is >3 days old while commits exist, the tracker is blind — that is finding #1, before any usage conclusion.

- Write to OS temp: `%TEMP%/retro-<timestamp>.html` (Windows) or `$TMPDIR/retro-<timestamp>.html`. Never in the repo.
- Open it (`start` / `open` / `xdg-open`), tell the user the absolute path.
- Buttons `#btn-pdf` (print) and `#btn-png` (html2canvas on `#report`) ship in the template. Both hide in print. PNG file: `retro-<timestamp>.png` at the same scale.
- Prose: short sentences, one topic each. No hedging paragraphs. If a card needs a paragraph to be understood, cut it.
