# Plan: ywai-fastfs (omp-inspired fast path)

Implemented 2026-07-13. Host remains **opencode**.

## What shipped

| Layer | Implementation |
|-------|----------------|
| B | `internal/fastfs` — cache, find, search, outline, slice, MCP stdio |
| B | `ywai mcp fastfs` + `InstallFastfsMCP` on install |
| C | `agents/sections/fast-tools.md` + context-gathering + finder/dev/… |
| A | doctor reports codegraph + fastfs workspace |

## Non-goals (still true)

No Rust N-API, no hashline, no replacing opencode shell tools.

## Usage

```bash
ywai install          # wires ywai-fastfs MCP
ywai mcp fastfs       # stdio MCP (agent host spawns this)
ywai doctor           # fast path checks
```
