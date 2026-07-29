# Work Item Content Templates

Drafting templates for work item bodies. For the commands that create them, see
`workflows.md` → "Create a work item".

## Field formats (read this first)

- **`description`** and everything inside it (repro steps, acceptance criteria,
  DoD): write **HTML** — `<p>`, `<ul>`, `<ol>`, `<li>`, `<strong>`, `<em>`.
  Azure DevOps stores rich-text fields as HTML; Markdown for work item fields is
  an opt-in migration per organization, so HTML is the safe default.
- **Comments** (`ado wi comment`, `ado wi update --comment`): write **Markdown**.
  The comments API renders it.
- The `ado` CLI passes values through verbatim — it converts nothing.
- There are no flags for repro steps, acceptance criteria, or severity. They go
  inside `--description`. Only pass flags that `commands.md` lists.

## Choosing a type

Map the request to an intent, then resolve the real type name with `ado wi types` —
type names differ per process (Agile has `User Story`, Scrum has `Product Backlog
Item`, CMMI has `Requirement`).

| Intent | Typical type |
| --- | --- |
| Something is broken / fails / errors | `Bug` |
| User-facing capability ("as a user I want…") | `User Story` / `Product Backlog Item` / `Requirement` |
| Technical work: refactor, setup, migration, spike | `Task` |
| Large capability spanning several stories | `Feature` |

Confirm the type against `ado wi types` before creating. Do not assume `state`
defaults either — read an existing item with `ado wi get <id>` to see the states
this project's process actually uses.

## Bug

Repro steps are mandatory. A bug without them is not actionable — ask for them
rather than guessing.

```
Title: [Area] short description of the problem

Description:
<p><strong>Summary:</strong> what happened and what should have happened.</p>

<p><strong>Steps to reproduce:</strong></p>
<ol>
  <li>Go to [page/component]</li>
  <li>Do [action]</li>
  <li>Observe [current result]</li>
</ol>

<p><strong>Actual result:</strong> [what happens]</p>
<p><strong>Expected result:</strong> [what should happen]</p>

<p><strong>Environment:</strong></p>
<ul>
  <li>Browser/app:</li>
  <li>Version:</li>
  <li>Test data:</li>
</ul>

<p><strong>Severity:</strong> 1 - Critical | 2 - Major | 3 - Minor | 4 - Suggestion</p>
```

## User Story / PBI / Requirement

```
Title: As a <role>, I want <goal> so that <benefit>

Description:
<p><strong>Context:</strong> [why this story exists, what problem it solves]</p>

<p><strong>Acceptance criteria:</strong></p>
<ul>
  <li>Given [context] When [action] Then [observable outcome]</li>
  <li>Given [context] When [action] Then [observable outcome]</li>
</ul>
```

Acceptance criteria must be verifiable: a concrete, observable outcome someone
can check. "Works correctly" and "is fast" are not acceptance criteria.

## Task

```
Title: [imperative verb] [noun] — e.g. "Refactor auth module", "Set up CI for API"

Description:
<p><strong>Technical work:</strong> [what to do, why, approach]</p>

<p><strong>Definition of done:</strong></p>
<ul>
  <li>[verifiable item 1]</li>
  <li>[verifiable item 2]</li>
</ul>
```

## Feature

```
Title: [short capability name]

Description:
<p><strong>Business value:</strong> [benefit to the user or the business]</p>

<p><strong>In scope:</strong></p>
<ul>
  <li>[included capability]</li>
</ul>

<p><strong>Out of scope:</strong></p>
<ul>
  <li>[excluded capability]</li>
</ul>

<p><strong>Acceptance criteria (high level):</strong></p>
<ul>
  <li>[criterion]</li>
</ul>
```

Break a Feature down into children with `ado wi create-child --parent <id>`.

## Common field rules

| Field | Rule |
| --- | --- |
| `--title` | Required. Max 256 characters. |
| `--description` | HTML. |
| `--priority` | 1 (highest) to 4 (lowest). |
| `--tags` | Semicolon-separated (`a;b`). Only tags the user asked for. |
| `--assigned` | Only when the user named a person. |
| `--state` | Only when the user asked for a specific state; otherwise let the process default apply. |
