# Ledger

The ledger is the only thing that carries task state forward. Keep it short enough to re-read at every seam.

Fields:

| Field | Meaning | Limit |
|---|---|---|
| **Goal** | What done means, in one line | 1 |
| **Core** | Live constraints / names / values the rest of the work must share | at most 2 |
| **Next** | The single next action | 1 |
| **Open** | Questions, each with what would settle it | as needed |
| **Verified** | What now holds, **and** the verifier + coverage | no entry without `--by` |

A third core item does not get a third slot. Swap one (`--core-slot 1|2`) or the stage is overloaded — finish or externalize something first.

Do not call a line verified unless you can name the command or check that covered it. "Should pass" is not a verifier.

Without the CLI, restating these five labelled lines at each seam is the whole protocol.

```
Goal: ...
Core: (1) ... (2) ...
Next: ...
Open: ...
Verified: ... (by ...)
```
