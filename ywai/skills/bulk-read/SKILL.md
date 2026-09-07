---
name: bulk-read
description: "Delegate large-file reading to a cheap opencode2 worker and read only its summary. Keeps big file contents out of this session's context. Trigger: bulk read, leer archivo grande, archivo muy largo, resumir archivos."
---

# Bulk read with opencode2

A cold `opencode2` worker reads the files and answers one question. The file contents land in the worker's context, not here. This session receives only the summary.

Load when a read would burn more context than the answer is worth.

## When to delegate a read

| Situation | Action |
|---|---|
| One file over ~350 lines, one specific question | delegate |
| Several large files, one question about all of them | delegate |
| Docs or references you only need summarized | delegate |
| You need exact text to edit | do not delegate — targeted `Read` with offset/limit |
| You need to debug or hunt a subtle bug | do not delegate — read inline |
| The file is small, or you already know the section | do not delegate — targeted read is faster |

## Invocation

Write the worker prompt to a file first. Name paths; never paste file contents.

```sh
cd <repo>
~/.opencode/bin/opencode2 run --auto --standalone --format json \
  --model opencode-admin/glm-5.3-flash --title "bulk-read" \
  "$(cat /tmp/opencode/bulk-read-prompt.md)"
```

Prompt file template:

```markdown
Read these files only:
- <path>

Question: <one question>

Output rules:
- Structured bullets only. No prose, no greetings.
- Lead each bullet with the exact symbol, name, or line number.
- Skip anything the question does not ask for.
- Keep the answer under 400 words.

This is a read-only task. Do not modify any file.
```

## Traps

- **Name paths, never paste contents.** The worker reads them with its own cheap tokens. Pasting re-costs the corpus in this session and defeats the purpose.
- **`--auto` always.** Without it the run blocks on a permission prompt nobody can answer and dies.
- **The binary is `~/.opencode/bin/opencode2`.** `opencode` is a different tool.
- **Run from the repo root.** Relative paths in the prompt resolve from there.
- **Tell the worker read-only.** A cold agent that spots a bug will fix it unless told not to.
- **Cap the answer (~400 words).** The summary returns through this session's bash output.
- **Follow-ups:** re-running with the same paths is the simple path. To reuse the worker's warm context for heavy follow-ups, pass `--session <id>` from the first run's JSON output instead.

## Verify before acting

- Worker summaries lack reliable line numbers. Spot-check any symbol you will act on with a targeted `Read` (offset/limit).
- Never edit based on a summary alone.
- If the summary reports something odd, read that section inline before you trust it.

## Related

- `delegate-opencode2` — general delegation mechanics, verification rules, and model routing.
- The 4-file rule in `agents/delegations.json` — understanding that needs 4+ files is a delegation, not a read.
