# Review lenses

Reference for `code-review` step 3. Apply every lens to every changed file. Each entry lists the questions that most often turn into real failure scenarios.

## Correctness
- Off-by-one, inclusive/exclusive bounds, empty and single-element inputs.
- Null / undefined / zero / empty string on every new path — who can pass it?
- Changed condition: is every branch of the old truth table still handled?
- Copy-paste drift: a duplicated block where one variable was not renamed.
- Units and formats: ms vs s, UTC vs local, cents vs decimals, 0- vs 1-based.

## Contracts and callers
- A changed signature, return shape, default or thrown error: does every caller (traced in step 2) still hold?
- Removed or renamed field in an API, event, DTO or config key: who still reads it — other services, old clients, stored data?
- Behaviour change hidden behind an unchanged name.

## Error paths
- What happens when the new call fails, times out or returns partial data?
- Swallowed errors (`catch {}`, ignored return values, `_ = err`) that turn failures into silent wrong results.
- Retries without idempotency; cleanup (files, locks, transactions) skipped on the error path.

## Concurrency and state
- Shared mutable state touched from requests, goroutines, workers or async callbacks.
- Check-then-act races (exists → create, read → update without a lock or version).
- Ordering assumptions between async operations; missing `await`.

## Security
- Untrusted input reaching SQL, shell, file paths, HTML, templates, regex or deserialization.
- Authorization: is the new endpoint/action checked for *this* user and *this* resource, not just "logged in"?
- Secrets in code, logs, URLs or error messages; overly broad CORS, permissions or tokens.

## Data and migrations
- Migration on a large table: lock, backfill and rollback plan; nullable vs default on existing rows.
- Code deployed before/after the migration: does each version work with both schemas?
- Deletes and updates without a tight `WHERE`; cascades.

## Performance
- Queries or remote calls inside loops (N+1); unbounded result sets without pagination.
- Work moved onto a hot path (per request, per render, per message).
- Caches: invalidation on write, unbounded growth, key collisions.

## Tests
- Does a test fail if the new behaviour breaks? (See the `testing-expert` skill for assertion quality.)
- Error paths and boundaries from the lenses above: covered or not?
- Tests changed to match new output: was the old expectation actually wrong?
