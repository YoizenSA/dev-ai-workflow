# The finding bar

Reference for `code-review` step 5. A finding ships only if it names a location, a failure scenario and a fix.

## Weak → strong

| Weak (drop or rewrite) | Strong |
|---|---|
| "Consider adding error handling here." | **Unhandled 404 crashes checkout** — `cart/service.ts:88` — `getPrice()` throws on a deleted product and nothing catches it, so any cart holding a delisted item returns 500 — wrap in try/catch and drop the line item with a warning. |
| "This could be a performance issue." | **N+1 on order history** — `orders/repo.go:141` — one `SELECT` per order inside the loop; a customer with 2,000 orders makes 2,001 queries (~4 s) — load items with one `WHERE order_id IN (...)`. |
| "Security concern with this input." | **Path traversal in export** — `export/handler.py:52` — `filename` from the query string is joined to `EXPORT_DIR` unchecked, so `../../etc/passwd` is served — resolve the path and reject anything outside `EXPORT_DIR`. |
| "Naming could be clearer." | Nit, non-blocking: `d` → `durationMs` at `timer.ts:12` (units are ambiguous). |

## Rules of thumb
- If you wrote "could", "might" or "consider" without a scenario, you have not finished step 3 for that finding.
- One defect, one finding: the same root cause in five places is one finding listing the five locations.
- Confidence reflects evidence: traced and reproduced → high; inferred from reading one side → say so.
- Praise, PR summaries and restating the diff are not findings — leave them out.
