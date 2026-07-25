---
name: git-commit
description: Conventional commit standards, branching, SemVer, and changelog conventions. Use when writing a commit or amending one, naming a branch, cutting a release or bumping a version, updating CHANGELOG.md, or when a commit hook rejects a message.
allowed-tools: [Read, Edit, Write, Glob, Grep, Bash]
---

## Critical Patterns

- **ALWAYS** follow the **Conventional Commits** format: `type(scope): subject`
- **NEVER** write subjects in past tense ("fixed", "added"); use imperative mood ("fix", "add")
- **ALWAYS** keep the title at **50 characters** or fewer
- **ALWAYS** add a `BREAKING CHANGE:` footer for backward-incompatible changes
- **NEVER** bundle multiple logical changes into one commit; keep them atomic and focused
- **ALWAYS** reference GitHub issues in the body via `Fixes #123`

## Reference

The detail lives beside this file so the rules above stay readable. Each pointer
says when to open it:

- [Commit message format](references/COMMIT-MESSAGE-FORMAT.md) — the full type
  and scope vocabulary, body and footer rules, and worked examples. Open it when
  you are unsure which type a change is, or need the exact `BREAKING CHANGE`
  footer shape.
- [Branching](references/BRANCHING.md) — branch types, naming, and the
  feature / bugfix / release / hotfix workflows. Open it before creating a
  branch or cutting a release.
- [Git hooks](references/GIT-HOOKS.md) — the lefthook setup, what each hook
  validates, and how to install or bypass them. Open it when a hook rejects a
  commit or the hooks are not running.

## Versioning and changelog

Version bumps follow SemVer, derived from the commits since the last tag: a
`BREAKING CHANGE` footer bumps major, `feat` bumps minor, `fix` bumps patch.
That mapping is the reason the type matters — a commit typed wrong silently
produces the wrong release.

`CHANGELOG.md` follows Keep a Changelog: group entries under Added, Changed,
Fixed, Removed, and Deprecated, newest version first, and write for someone
upgrading rather than for someone reading the diff.

## Discovering the project's setup

The conventions above are the default; a repository may enforce its own. Before
committing into an unfamiliar repo, check what is actually configured —
`lefthook.yml` or `.husky/` for hooks, `commitlint*` for accepted types, and
`git log --oneline -20` for the scopes the project really uses. Match the repo
you are in over this skill.

## Best practices

One logical change per commit, and every commit passes the suite on its own —
a commit that only builds together with the next one cannot be reverted or
bisected, which is what commit history is for.

Explain what changed and why in the body; the diff already shows how. Reference
the issue (`Fixes #123`) so the context survives after everyone involved has
forgotten it.

## Assets
- `assets/scripts/commit.sh` - Commit automation script
- `assets/scripts/release.sh` - Release automation script
