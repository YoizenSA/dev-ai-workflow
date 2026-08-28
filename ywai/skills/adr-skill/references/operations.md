# Other operations — update, lifecycle, index, bootstrap, categories

Reference for `adr-skill`.

## Other Operations

### Update an Existing ADR

1. Identify the intent:
   - **Accept / reject**: change status, add any final context.
   - **Deprecate**: status → `deprecated`, explain replacement path.
   - **Supersede**: create a new ADR, link both ways (old → new, new → old).
   - **Add learnings**: append to `## More Information` with a date stamp. Do not rewrite history.

2. Use `scripts/set_adr_status.js` for status changes (supports YAML front matter, bullet status, and section status).

### Post-Acceptance Lifecycle

After an ADR is accepted:

1. **Create implementation tasks.** Each item in the Implementation Plan and each follow-up in Consequences should become a trackable task (issue, ticket, or TODO).
2. **Reference the ADR in PRs.** Link to the ADR in PR descriptions: "Implements ADR-0004."
3. **Add code references.** Add `ADR-NNNN` comments at key implementation points.
4. **Check verification criteria.** Once implementation is complete, walk through the Verification checkboxes. Update the ADR with results in `## More Information`.
5. **Revisit when triggers fire.** If the ADR specified revisit conditions ("if X happens, reconsider"), monitor for those conditions.

### Index

If the repo has an ADR index/log file (often `README.md` or `index.md` in the ADR dir), keep it updated.

Preferred: let `scripts/new_adr.js --update-index` do it. Otherwise:
- Add a bullet entry for the new ADR.
- Keep ordering consistent (numeric if numbered; date or alpha if slugs).

### Bootstrap

When introducing ADRs to a repo that has none:

```bash
node /path/to/adr-skill/scripts/bootstrap_adr.js
```

This creates the directory, an index file, and a filled-out first ADR ("Adopt architecture decision records") with real content explaining why the team is using ADRs. Use `--json` for machine-readable output. Use `--dir` to override the directory name.

### Categories (Large Projects)

For repos with many ADRs, organize by subdirectory:

```
docs/decisions/
  backend/
    0001-use-postgres.md
  frontend/
    0001-use-react.md
  infrastructure/
    0001-use-terraform.md
```

Numbers are local to each category. Choose a categorization scheme early (by layer, by domain, by team) and document it in the index.

