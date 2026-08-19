# Resources — scripts, templates, and their usage

Reference for `adr-skill`.

## Resources

### scripts/
- `scripts/new_adr.js` — create a new ADR file from a template, using repo conventions.
- `scripts/set_adr_status.js` — update an ADR status in-place (YAML front matter or inline). Use `--json` for machine output.
- `scripts/bootstrap_adr.js` — create ADR dir, `README.md`, and initial "Adopt ADRs" decision.

### references/
- `references/review-checklist.md` — agent-readiness checklist for Phase 3 review.
- `references/adr-conventions.md` — directory, filename, status, and lifecycle conventions.
- `references/template-variants.md` — when to use simple vs MADR-style templates.
- `references/examples.md` — filled-out short and long ADR examples with implementation plans.

### assets/
- `assets/templates/adr-simple.md` — lean template for straightforward decisions.
- `assets/templates/adr-madr.md` — MADR 4.0 template for decisions with multiple options and structured tradeoffs.
- `assets/templates/adr-readme.md` — default ADR index scaffold used by `scripts/bootstrap_adr.js`.

### Script Usage

From the target repo root:

```bash
# Simple ADR
node /path/to/adr-skill/scripts/new_adr.js --title "Choose database" --status proposed

# MADR-style with options
node /path/to/adr-skill/scripts/new_adr.js --title "Choose database" --template madr --status proposed

# With index update
node /path/to/adr-skill/scripts/new_adr.js --title "Choose database" --status proposed --update-index

# Bootstrap a new repo
node /path/to/adr-skill/scripts/bootstrap_adr.js --dir docs/decisions
```

Notes:
- Scripts auto-detect ADR directory and filename strategy.
- Use `--dir` and `--strategy` to override.
- Use `--json` to emit machine-readable output.
