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

Delegate to `@scenario-verification-orchestrator` (the `scenario-verification`
workflow) with the work item id and the environment. It runs every scenario
against the running app, files the evidence under `.evidence/<run-id>/`
(screenshot at each assertion, the Jev Then line for each UI scenario, the
failing request, the log excerpt), and posts the result on the work item: a
summary comment on the parent and one Bug child per failing scenario. If that
agent is not installed, ask the user to run `/scenario-verification <id>`.

**A red scenario blocks**, even with a clean review. Send it back for a fix and
run it again, at most twice; then stop and report what still fails with its
evidence. Never resolve a red by editing the scenario.

Report the verdict with the evidence paths. "Verified" without them is an
opinion.
