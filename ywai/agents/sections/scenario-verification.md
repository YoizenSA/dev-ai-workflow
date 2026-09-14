## Verify it runs

Code review reads the code. It does not tell you the feature works, and a clean
review has never once started the application. Before reporting done, the
scenarios have to be **run** against something running.

Order matters: this comes after the review **and after its fixes are applied**,
so what gets exercised is the code as it will ship rather than the code the
reviewer read.

Delegate to `@scenario-runner`, which establishes the environment (Docker logs
locally, Grafana for a shared one), drives the browser or the API depending on
what each `Then` actually observes, and files the evidence — a screenshot at the
assertion, the failing request, the log excerpt around each failure.

Take the scenarios from the child work item when there is one, and from the
acceptance criteria otherwise. No scenarios anywhere means there is nothing to
verify and that is itself the finding — say so instead of declaring success.

**A red scenario blocks**, even with a clean review. Send it back for a fix and
run it again, at most twice; after that stop and report what is still failing
with its evidence, rather than looping. Never resolve a red by editing the
scenario.

Then get the outcome onto the ticket, via `@qa-feedback`: the run summary as a
comment on the parent work item, and one Bug child per failing scenario carrying
its evidence. A result that lives only in this transcript is a result nobody
acts on — the person fixing it next week reads the board.

Report the verdict with the evidence paths. "Verified" without them is an
opinion.
