## Development

- `npm test` — run the test suite once
- `npm run test:watch` — run tests in watch mode
- `npm run stylelint` — lint CSS
- `npm run lint` — lint TS/TSX
- `npm run tsc` — typecheck

### One-time setup

After cloning or pulling this PR, install the pre-commit hook:

    npx lefthook install

This enables stylelint, eslint, and typecheck on every commit.
