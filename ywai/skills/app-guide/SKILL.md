---
name: app-guide
description: "Per-repo app guide for QA agents. Trigger: launch the app, sign in to test, QA run setup, never-do list."
---

# App Guide

Every project a QA agent runs against carries `.ywai/app-guide.md`: a committed, plain-English guide to launching and using the app. One file, in the repo, so the agent starts the app the way the team does instead of guessing.

Before any run, read it. If it is missing, ask for it — do not reconstruct launch steps from memory.

## The four sections

| Section | What it holds |
|---|---|
| Launch | The command that starts the app |
| Sign-in | Test account or seed steps |
| Never-do | What the agent must not touch |
| Secrets | List of `{{NAME}}` placeholder names |

## Secrets are placeholders, never values

Secret values never appear in the file. Only `{{NAME}}` placeholders. The operator's environment resolves them at run time. A placeholder's value is never sent to a model and never written to a log — the name is the contract, the value stays local.

When writing or editing a guide: fill in the four sections, list every placeholder name under Secrets, and stop there. A guide longer than a screen is two guides.

## Minimal example

```markdown
# App Guide

## Launch
docker compose up -d
Ready at http://localhost:3000.

## Sign-in
Use the seeded account: qa@example.com / {{QA_PASSWORD}}.

## Never-do
Do not touch production profiles or the billing settings screen.

## Secrets
- {{QA_PASSWORD}}
```
