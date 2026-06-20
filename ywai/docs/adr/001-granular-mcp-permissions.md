# ADR-001: Granular MCP Tool Permissions

**Status:** Proposed

---

## Context

ywai defines agent roles (`ask`, `dev`, `qa`, `architect`, `reviewer`, `devops`, `orchestrator`) in `ywai/agents/core/*/`. Each agent carries a `permissions.json` with flat permission keys like `"read": "allow"`, `"edit": "deny"`.

The `mcp` permission is currently **binary** — `allow` or `deny`. When set to `allow`, it opens ALL MCP tools to the agent, regardless of whether they are read-only or write-capable.

**The problem:** The `ask` agent has `edit: deny`, `write: deny`, `bash: deny` but `mcp: allow`. This is contradictory — `ask` is meant to be a research & Q&A agent that cannot modify anything, yet `mcp: allow` grants access to destructive tools like:

| Tool | Operation |
|---|---|
| `ado_review` | Submit a PR review vote |
| `ado_pr_comment` | Write comments on PRs |
| `ado_work_item_update` | Change work item state |
| `engram_mem_save` | Write to persistent memory |
| `ywai-kanban_delete_session` | Delete entire sessions |

### Active MCP servers

| Server | Tools | Nature |
|---|---|---|
| **Azure DevOps** | `ado_*` | Mixed — query PRs (read), submit reviews (write), update work items (write) |
| **CodeGraph** | `codegraph_*` | All read-only — search, explore, context, trace |
| **Context7** | `context7_*` | All read-only — query docs, resolve library IDs |
| **Engram** | `engram_mem_*` | Mixed — search/context/get (read), save/update/delete (write), session control (admin) |
| **ywai-kanban** | `ywai-kanban_*` | Mixed — list/get (read), create/delete/update (write) |

The current binary model cannot express "read-only MCP access." Every agent that needs any MCP tool gets all of them.

---

## Decision

Introduce a **three-level permission model** for MCP tools:

### Level 1: Automatic tool classification

Every MCP tool is classified into one of three categories based on its operation semantics:

| Category | Definition | Examples |
|---|---|---|
| **read** | Queries or retrieves data. No side effects. | `codegraph_*`, `context7_*`, `ado_prs`, `engram_mem_context`, `ywai-kanban_get_board` |
| **write** | Creates, modifies, or deletes state. | `ado_review`, `engram_mem_save`, `ywai-kanban_delete_session` |
| **admin** | Changes configuration or session lifecycle. | `ado_profile_use`, `engram_mem_session_start`, `engram_mem_session_end` |

**Classification rules:**
- Tool names are matched against a registry of known patterns (exact name or prefix-based, e.g. `codegraph_* → read`).
- Unknown tools default to `write` (fail-safe: default-deny on unknown).
- The registry is maintained in the ywai codebase and updated when new MCP servers are added.

### Level 2: Category-based permission keys

New keys in `permissions.json` that filter MCP tools by category:

```json
{
  "mcp:read": "allow",
  "mcp:write": "deny",
  "mcp:admin": "deny"
}
```

### Level 3: Per-tool overrides (highest precedence)

Explicit allow/deny for specific tool names. Useful when a single tool within a category needs different treatment:

```json
{
  "mcp:write": "deny",
  "engram_mem_save": "allow"
}
```

### Precedence rules (deny-wins)

1. **Per-tool override** — most specific, checked first
2. **Category rule** (`mcp:read`, `mcp:write`, `mcp:admin`) — applies if no per-tool rule matches
3. **Blanket `mcp` rule** — fallback if no category rule is set
4. If no rule matches at any level, the tool is **denied** (default-deny)

### Backward compatibility

- `mcp: allow` is equivalent to:
  ```
  mcp:read: allow, mcp:write: allow, mcp:admin: allow
  ```
- `mcp: deny` blocks all MCP tools (unchanged behavior).
- Existing `permissions.json` files without category keys continue to work — the blanket `mcp` key acts as the fallback.
- No changes needed to MCP server implementations. The enforcer lives in ywai's permission parsing layer.

---

## Consequences

### Positive

- **Principle of least privilege:** The `ask` agent can be granted read-only MCP access by setting `"mcp:read": "allow"` while keeping `"mcp:write": "deny"` and `"mcp:admin": "deny"`.
- **Future-proof:** New MCP servers are automatically constrained by their tool classification — no need to audit every agent.
- **No MCP server changes:** The enforcer is purely on ywai's side, intercepting tool execution requests.
- **Clear precedence:** deny-wins + specificity ordering makes permission resolution predictable.

### Negative

- **Registry maintenance:** Every new MCP server requires a manual review to classify its tools in the registry.
- **Parsing complexity:** The permission model moves from a flat map with one `mcp` key to three category keys plus optional per-tool overrides.
- **Agent migration:** Existing agents continue to work (backward compatible), but their `mcp: allow` may be broader than intended — a review is warranted.

### Neutral

- **Documentation needed:** The three-level model and classification rules must be documented in the agent authoring guide.
- **Community MCP servers:** Tools from community MCP servers default to `write` unless explicitly classified — safe but may cause false denials.

---

## Implementation sketch

### 1. Tool classification registry (`ywai/internal/agents/permissions.go` — new file)

```go
// ToolCategory represents the MCP tool operation category.
type ToolCategory int

const (
    ToolRead  ToolCategory = iota
    ToolWrite
    ToolAdmin
)

// toolRegistry maps MCP server prefixes and exact tool names to categories.
var toolRegistry = map[string]ToolCategory{
    // CodeGraph — all read-only
    "codegraph_": ToolRead,
    // Context7 — all read-only
    "context7_":  ToolRead,
    // Azure DevOps — mixed
    "ado_prs":             ToolRead,
    "ado_pr_diff":         ToolRead,
    "ado_pr_file":         ToolRead,
    "ado_pr_threads":      ToolRead,
    "ado_work_item":       ToolRead,
    "ado_work_items":      ToolRead,
    "ado_related_work_items": ToolRead,
    "ado_review":          ToolWrite,
    "ado_pr_comment":      ToolWrite,
    "ado_work_item_comment":  ToolWrite,
    "ado_work_item_update":   ToolWrite,
    "ado_profile_use":     ToolAdmin,
    "ado_profiles":        ToolRead,
    "ado_select_pr":       ToolWrite,
    // Engram — mixed
    "engram_mem_context":   ToolRead,
    "engram_mem_search":    ToolRead,
    "engram_mem_get_observation": ToolRead,
    "engram_mem_save":      ToolWrite,
    "engram_mem_update":    ToolWrite,
    "engram_mem_save_prompt": ToolWrite,
    "engram_mem_capture_passive": ToolWrite,
    "engram_mem_session_start": ToolAdmin,
    "engram_mem_session_end":   ToolAdmin,
    "engram_mem_session_summary": ToolWrite,
    "engram_mem_suggest_topic_key": ToolRead,
    // ywai-kanban — mixed
    "ywai-kanban_get_":     ToolRead,
    "ywai-kanban_list_":    ToolRead,
    "ywai-kanban_get_board": ToolRead,
    "ywai-kanban_get_graph": ToolRead,
    "ywai-kanban_get_ui_url": ToolRead,
    "ywai-kanban_create_":  ToolWrite,
    "ywai-kanban_update_":  ToolWrite,
    "ywai-kanban_delete_":  ToolWrite,
    "ywai-kanban_add_activity": ToolWrite,
    // Delegation — mixed
    "delegation_list": ToolRead,
    "delegation_read": ToolRead,
    "delegate":        ToolWrite,
}

// ClassifyTool returns the category for a given MCP tool name.
// Falls back to ToolWrite for unknown tools (safe default).
func ClassifyTool(name string) ToolCategory {
    // Check exact match first
    if cat, ok := toolRegistry[name]; ok {
        return cat
    }
    // Check prefix matches (longest prefix wins)
    var match ToolCategory
    var matchLen int
    for pattern, cat := range toolRegistry {
        if strings.HasPrefix(name, pattern) && len(pattern) > matchLen {
            match = cat
            matchLen = len(pattern)
        }
    }
    if matchLen > 0 {
        return match
    }
    return ToolWrite // safe default for unknown tools
}
```

### 2. Extended permission struct (in `agents.go`)

```go
type AgentPermissions struct {
    // Existing flat keys (kept for backward compatibility)
    Raw map[string]string

    // Parsed MCP categories (populated from mcp:read, mcp:write, mcp:admin, mcp)
    MCPRead  *bool // nil = not set, falls back to blanket mcp
    MCPWrite *bool
    MCPAdmin *bool

    // Per-tool overrides (e.g. "ado_review": "deny")
    ToolOverrides map[string]bool
}
```

### 3. MCP Enforcer component

```go
// MCPEnforcer evaluates whether an agent may invoke a specific MCP tool.
type MCPEnforcer struct {
    perms AgentPermissions
}

func (e *MCPEnforcer) Allow(toolName string) (bool, string) {
    // 1. Per-tool override (highest precedence)
    if allow, ok := e.perms.ToolOverrides[toolName]; ok {
        return allow, fmt.Sprintf("per-tool override: %v", allow)
    }

    // 2. Classify the tool
    cat := ClassifyTool(toolName)

    // 3. Check category rule
    var catRule *bool
    switch cat {
    case ToolRead:
        catRule = e.perms.MCPRead
    case ToolWrite:
        catRule = e.perms.MCPWrite
    case ToolAdmin:
        catRule = e.perms.MCPAdmin
    }
    if catRule != nil {
        return *catRule, fmt.Sprintf("mcp:%s rule: %v", categoryName(cat), *catRule)
    }

    // 4. Fall back to blanket mcp rule
    if blanket, ok := e.perms.Raw["mcp"]; ok {
        return blanket == "allow", fmt.Sprintf("blanket mcp rule: %s", blanket)
    }

    // 5. Default-deny
    return false, "no matching permission — default deny"
}
```

### 4. Updated `ask` agent permissions

```json
{
  "read": "allow",
  "edit": "deny",
  "write": "deny",
  "bash": "deny",
  "mcp:read": "allow",
  "mcp:write": "deny",
  "mcp:admin": "deny"
}
```

This grants access to `codegraph_*`, `context7_*`, `engram_mem_search`/`context`, `ado_prs` etc. while blocking `ado_review`, `engram_mem_save`, `ywai-kanban_delete_session`, and other write-capable tools.

### 5. Tests to write

| Scenario | What it verifies |
|---|---|
| Classification | `codegraph_search` → read, `ado_review` → write, `ado_profile_use` → admin |
| Unknown tool | Defaults to write (denied if mcp:write is deny) |
| Precedence | Per-tool override beats category rule beats blanket mcp |
| Deny-wins | Conflicting rules resolve to deny |
| Backward compat | Old `mcp: allow` still works, grants all three categories |
| Ask agent effective | Only read MCP tools are allowed |

---

## Relevant files

| File | Role |
|---|---|
| `ywai/internal/agents/agents.go` | Main permission parsing, `permOrder`, `parsePermissions` |
| `ywai/internal/agents/permissions.go` | New file — tool classification registry + `MCPEnforcer` |
| `ywai/internal/agents/agents_test.go` | Tests for permission parsing |
| `ywai/agents/core/ask/permissions.json` | First agent to adopt granular MCP permissions |
| `ywai/agents/core/*/permissions.json` | Other agents — review for appropriate MCP scope |
| `~/.config/opencode/opencode.json` | OpenCode MCP server configuration |
