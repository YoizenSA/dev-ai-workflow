# Gate

Reuse orchestrator mode. Do not invent `fast` / `loop` or any parallel pass names.

| Pass | Signal | State |
|---|---|---|
| **solo** | One question, one mechanical fix, one file, checkable in a glance | No ledger |
| **thin** | Clear "do X", few files, one deliverable | No ledger. Ship before anything leaves. |
| **full** | Multi-phase, multi-file, multi-turn, or state that must survive a seam | Open the ledger first |

Escalate solo → thin → full when scope blows up. Say so. Downgrade only on an explicit user override.

User-visible, once per task, short: `mode: <solo|thin|full>`.

Brevity is an outer-length request. Verification stays at the floor of the pass you actually landed on.
