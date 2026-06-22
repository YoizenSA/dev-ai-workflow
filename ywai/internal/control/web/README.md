## Development

- `npm test` — run the test suite once
- `npm run test:watch` — run tests in watch mode
- `npm run stylelint:all` — lint all CSS in src/ (whole-project check)
- `npm run lint:all` — lint all TS/TSX in src/ (whole-project check)
- `npm run tsc` — typecheck

The `npm run stylelint` and `npm run lint` scripts are designed to be
called with file arguments (typically by the pre-commit hook below).
Running them with no args will fail; use the `:all` variants for
whole-project checks.

### One-time setup

After cloning or pulling this PR, install the pre-commit hook:

    npx lefthook install

This enables stylelint, eslint, and typecheck on every commit, scoped
to staged files under `internal/control/web/`.
