---
description: Weekly dev-AI workflow retro over sessions, evals and memory
argument-hint: "[scope, e.g. ultimos 7 dias, --days 30, proyecto X]"
---

The user ran `/workflow-retro`. Load the `workflow-retro` skill and run the retro.

Parse scope from the arguments (examples: `ultimos 7 dias` -> days=7, `--days 30` -> days=30, a path or name -> worktree/project). Defaults: days=7. Then follow the skill process: collect -> validate -> cluster -> interrogate -> propose -> HTML report.

$ARGUMENTS
