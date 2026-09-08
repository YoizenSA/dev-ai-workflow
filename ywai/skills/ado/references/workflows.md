# ADO Workflows — task playbooks

Step-by-step recipes for common tasks. Every step is one `ado` command; read
steps come before mutating steps. See `commands.md` for full flag reference.

## Code review a PR

1. `ado pr context <prId>` — one call: metadata, linked work items, commits, changed files, threads.
2. `ado pr diff <prId> --hunks` — the actual unified diff. Review the hunks, not the file list.
3. Resolve the ticket and read it: `ado wi get <id>`. If `pr context` lists no linked work
   item, look for an ID in the branch name, title or description (`AB#1234`, `#1234`,
   `bugfix/1234_...`, a `_workitems/edit/<id>` link); if none is found, ask the user and
   stop. Without the ticket you cannot check the PR against its intended scope.
   `ado wi attachments <id>` when the ticket references screenshots.
4. For files that need a closer look: `ado pr file --path <file> <prId>` (use `--start`/`--end` for large files).
5. Read `ado pr threads <prId>` before writing, so you don't repeat existing feedback.
6. Print the review and ask for confirmation before posting anything. Dry run is the default.
7. Post findings as anchored comments: `ado pr comment <prId> --comment "<finding>" --file <path> --line <n>`.
8. Vote once at the end: `ado pr vote <prId> <approve|suggestions|wait|reject> [--comment "<summary>"]`.

Review output contract:
- One finding = one line: **what — where (`file:line`) — why**. No summary of the PR, no greetings.
- Split blocking defects from non-blocking suggestions; only real defects block.
- Report only what this diff introduces or worsens. Pre-existing problems are at most one suggestion.
- Budget: 12 findings max. Over budget, keep the most severe and close with `(+N minor findings omitted)`.
- Close with the QA angle: what a tester should exercise, one line each. Post it to the
  ticket with `ado wi comment <wiId> --comment "<qa block>"` when the user asks.

Guidelines:
- Comment BEFORE voting — a vote with unexplained rejection is useless to the author.
- `suggestions` = approved with non-blocking notes; `wait` = author must respond; `reject` = blocking defect.
- PR comments are Markdown; only `--description` on work items is HTML.

## Answer / follow up on PR threads

1. `ado pr threads <prId>` — read existing discussion.
2. Reply with `ado pr comment <prId> --comment "<reply>"` (anchor with `--file`/`--line` when it targets code).

## Create a work item

1. Check the project rules first: read `[work_item.create]` in `.adoconfig.toml` (enabled? allowed types? required fields?).
2. Unsure about valid types: `ado wi types` — names differ per process (`User Story` vs `Product Backlog Item` vs `Requirement`).
3. Draft the body with `templates.md` — `--description` is Azure DevOps HTML, comments are Markdown.
4. Resolve the assignee: `ado wi list` → read the `@<name>` column (that is the authenticated user). Never hardcode a name.
5. Ask and lock the kanban: **Social Kanban** or **Infra Kanban**? (Always ask.)
   - Social Kanban → `--profile ysocial` + `--area "ySocial\Kanban"`
   - Infra Kanban → `--area "Infra\Infra Kanban"`
6. Ask **Producto** (`ySocial` | `yFlow` | `yWhatsApp` | `yMobile` | `interno` | `yIA`) and **Sponsors** (`Dev Area` | `Implementacion` | `Producto` | `Soporte`). Both fields are required by the project template — pass `--field "Custom.USProducto=<v>"` and `--field "Custom.Sponsors=<v>"`.
7. Create (defaults from the create contract unless the user says otherwise):
   `ado wi create --title "<t>" --type "User Story" --assigned "<username>" --area "<area>" --field "Custom.USProducto=<producto>" --field "Custom.Sponsors=<sponsor>" [--description "<d>"] [--priority <n>] [--parent <id>]`
8. For a child of an existing item: `ado wi create-child --parent <id> --title "<t>"`.
9. Verify: `ado wi get <id>`; report ID, URL, type, title, state.

Confirm with the user before creating when the request is ambiguous, the item is
high-impact (Feature/Epic), or no type was specified.

If creation is rejected, the error names the violated rule — fix the flag, don't retry blindly.

## Update a work item after finishing work

1. `ado wi get <id>` — confirm current state and that it's the right item.
2. `ado wi update <id> --state "<state>" --comment "<what was done, PR link>"` — one command updates state and comments.

## Create a PR for finished work

1. Confirm the branch exists and is pushed (git, not ado).
2. `ado wi get <id>` if linking a work item — verify the ID.
3. `ado pr create --repo <r> --source <branch> --target <branch> --title "<t>" --wi <id> [--draft]`.
4. Respect `.adoconfig.toml` `[pr]` rules: `require_work_item`, `default_draft`.

## Create a chain of PRs from work items

1. `ado wi get <id>` for each item — confirm order and scope.
2. `ado pr chain --repo <r> --wi 1,2,3 [--strategy feature-chain|stacked]` — order of `--wi` is the chain order.
3. Strategy/base/prefix default from `.adoconfig.toml` `[chain]`; only pass flags to override.

## Find my pending work

- PRs waiting for my review or authored by me: `ado pr list`.
- Work items assigned to me (non-Closed): `ado wi list`.
- Filter: `ado wi list --state "In Dev" --type Bug`.
