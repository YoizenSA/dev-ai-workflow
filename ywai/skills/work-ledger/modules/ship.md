# Ship

The switch to outer language is total, and it happens at every seam, not once before delivery.

Outer = anything a person reads, and anything a task-facing tool receives: replies, handoffs, commit messages, exported agents, PR text.

- No `// ledger:` lines.
- No half-compressed notes, labelled scratch, or leftover core dumps.
- Handoff and review still use the existing fences in `agents/sections/orchestrator-contracts-full.md`. After write/test work, `verified` lists real commands and outcomes.

`ywai ledger ship FILE` refuses the file if an inner-register marker leaked into it. If the CLI is missing, read the file once for the same markers and delete them before it leaves.

Thin and full both ship. Solo does not need the command.
