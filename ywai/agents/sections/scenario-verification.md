## Verify it runs

Code review reads the code. It does not tell you the feature works, and a clean
review has never once started the application. Before reporting done, the
scenarios have to be **run** against something running.

### When it fires

Gated on two conditions, the same two `ship` gates:

1. **Scenarios exist** — the child Task from the work item, or the acceptance
   criteria when there is no item. No scenarios anywhere means there is nothing
   to verify and that is itself the finding — say so instead of declaring
   success.
2. **Review is clean and its fixes are applied** — not merely reviewed. What
   gets exercised has to be the code as it will ship, not the code the reviewer
   read.

Both true → run. It fires in every mode, `solo` and `thin` included: mode picks
who drives, not whether the change is verified.

### How

Delegate to `@scenario-runner`, which establishes the environment (Docker logs
locally, Grafana for a shared one), drives Given/When in the browser or hits
the API depending on what each `Then` actually observes, and files the
evidence — a screenshot at the assertion, the Jev Then line for every UI
scenario, the failing request, the log excerpt around each failure — under
`.evidence/<run-id>/`. A UI Then is scored by `jev_check_page`; the runner
does not invent that PASS.

**A red scenario blocks**, even with a clean review. Send it back for a fix and
run it again, at most twice; after that stop and report what is still failing
with its evidence, rather than looping. Never resolve a red by editing the
scenario.

Then get the outcome onto the ticket, via `@qa-feedback`: the run summary as a
comment on the parent work item, and one Bug child per failing scenario carrying
its evidence. Blocked scenarios are reported as blocked on the parent, never
filed as Bugs. A result that lives only in this transcript is a result nobody
acts on — the person fixing it next week reads the board.

Report the verdict with the evidence paths. "Verified" without them is an
opinion.
