## Fast tools (CodeGraph → host tools)

Exploration should reach for structural intelligence before shelling out. Prefer, in order:

### 1. Structure → CodeGraph (first)

For “where is X”, “what calls Y”, “how does Z work”, architecture, blast radius:

- `codegraph_explore` / `codegraph_search` / `codegraph_context` / `codegraph_trace`

Do **not** start with bash + ripgrep for structural questions.

### 2. Text search & path discovery → host tools

When you need regex content search or glob path discovery, use the host-native
tools (`grep`, `glob`, `code_search`, `read`) rather than shelling out to raw
`rg` / `find` in `bash`.

### 3. Host `bash`

Only when you need a **mutating** or build/test/git command. Never use `bash`
solely to run `rg`, `grep`, `find`, or to `cat` large source files for
exploration — use the dedicated `grep` / `glob` / `read` tools instead.

### Read discipline

1. Use `codegraph_explore` before dumping a whole file.
2. `read` only the lines you actually need.
3. Avoid full-file host `read` on large files when codegraph context suffices.
