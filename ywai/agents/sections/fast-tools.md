## Fast tools (CodeGraph + ywai-fastfs)

Exploration must avoid shelling out to `rg` / `grep` / `find` / `cat` when better tools exist. Prefer, in order:

### 1. Structure → CodeGraph (first)

For “where is X”, “what calls Y”, “how does Z work”, architecture, blast radius:

- `codegraph_explore` / `codegraph_search` / `codegraph_context` / `codegraph_trace`

Do **not** start with bash + ripgrep for structural questions.

### 2. Text search & path discovery → ywai-fastfs

Long-lived MCP process with **mtime cache** (no per-call fork):

| Tool | Use for |
|---|---|
| `ywai-fastfs_fastfs_find` | Glob paths (`*.go`, `**/*.ts`) |
| `ywai-fastfs_fastfs_search` | Regex content search (structured matches) |
| `ywai-fastfs_fastfs_read_outline` | Summarized file (signatures + elided sample) |
| `ywai-fastfs_fastfs_read_slice` | Bounded line range only (default max 200 lines) |
| `ywai-fastfs_fastfs_stat` | Metadata + cache stats |

### 3. Host `read` / `grep` / `bash`

Only when:

- CodeGraph and ywai-fastfs are unavailable, or
- You need a **mutating** or build/test/git command (`bash`), or
- You need a host-native tool that fastfs does not cover

**Never** use `bash` solely to run `rg`, `grep`, `find`, or to `cat` large source files for exploration.

### Read discipline

1. `ywai-fastfs_fastfs_read_outline` (or codegraph) before dumping a whole file.
2. `ywai-fastfs_fastfs_read_slice` for the lines you actually need.
3. Avoid full-file host `read` on large files when an outline suffices.
