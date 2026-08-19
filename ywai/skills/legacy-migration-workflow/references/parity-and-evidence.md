# Parity and evidence requirements

Reference for `legacy-migration-workflow`. Read this when writing a plan or
validating one — it defines what a plan must contain to be executable.

## Mandatory parity coverage

- Legacy business logic parity (`WebMethod`, `RegisterJsonVariable`, Page_Load branches, hidden branches, default values, validation, side effects).
- UI/visual parity: filters, table columns/order, buttons/actions, row actions, dialogs/popups, tabs/sections, scroll containers, empty/loading/error states, and approved deviations.
- Shared/global UI and API capabilities discovered from legacy helpers, popups, drill-ins, catalogs, and reusable surfaces.
- Permission parity.
- License parity.
- `isSuper` parity.
- i18n parity across every supported language and rendered UI key validation.
- Enum/icon parity with exact legacy enum source and numeric values.
- DTO reuse-first policy.

## Shared / foundation scope expansion

- Missing shared/global capabilities are not blockers by default.
- The planner must add them to `Shared / foundation dependencies` as required migration work with stable `F###` IDs.
- The build phase implements page-local scope plus all in-scope shared/foundation dependencies.
- The validator validates page parity plus shared/foundation dependency completeness.
- Validation must fail if a required shared/foundation dependency is missing, partial, unwired, or untested.
- Defer shared/foundation work only when the user explicitly approves the parity impact.

## Evidence requirements

- Every plan must contain a Legacy Parity Contract table. Each row maps `ID`, `Legacy element`, `Legacy source`, `Modern API/DTO`, `Modern UI`, `Test/evidence`, and `Status`.
- Every UI-bearing migration must contain a Visual Parity Inventory. The inventory must cite the legacy markup/script source and the modern template/style source.
- Every enum/status/icon/label mapping must contain an Enum/Icon Matrix with exact enum file, member name, numeric value, modern mapping, i18n key, icon/badge class, and evidence.
- Every visible string added or reused by the migration must be present in all supported language bundles. Validation fails on raw rendered translation keys.
- Build/remediation must append an Evidence Log entry with commands run, results, files changed, and affected parity rows before requesting validation.
- Validation reads the evidence but must independently inspect source and tests; evidence alone is not proof of parity.

## Mandatory governance coverage

- Unit tests required.
- Integration tests where feasible.
- `Yoizen.Legacy/migration-progress-tracker.md` update required in same change set.

## Tracker validation requirement

Validation must fail if any of these is missing or inconsistent:
- Migrated page line status was not updated.
- `### Overall Progress` counters/percentages mismatch page status totals.
- `### By Module Progress` counters/percentages mismatch module totals.
- `*Last Updated:*` not refreshed after tracker modification.

