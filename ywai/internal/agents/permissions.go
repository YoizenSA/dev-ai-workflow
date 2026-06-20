package agents

import "strings"

// ToolCategory represents the category of an MCP tool.
type ToolCategory int

const (
	// ToolRead represents read-only MCP tools (safe, no side effects).
	ToolRead ToolCategory = iota
	// ToolWrite represents MCP tools that modify state or create resources.
	ToolWrite
	// ToolAdmin represents MCP tools that change configuration or have broad impact.
	ToolAdmin
)

// String returns the human-readable category name.
func (c ToolCategory) String() string {
	switch c {
	case ToolRead:
		return "read"
	case ToolWrite:
		return "write"
	case ToolAdmin:
		return "admin"
	default:
		return "unknown"
	}
}

// toolPattern holds a prefix pattern and its category for classification.
type toolPattern struct {
	prefix   string
	category ToolCategory
}

// toolRegistry defines the classification rules as prefix patterns.
// More specific (longer) prefixes must come before shorter ones for correct matching.
// Unknown tools default to Admin (safest).
var toolRegistry = []toolPattern{
	// codegraph — all read-only queries
	{prefix: "codegraph_", category: ToolRead},

	// context7 — read-only documentation queries
	{prefix: "context7_", category: ToolRead},

	// Azure DevOps — read tools
	{prefix: "ado_pr_diff", category: ToolRead},
	{prefix: "ado_pr_file", category: ToolRead},
	{prefix: "ado_pr_threads", category: ToolRead},
	{prefix: "ado_pr_review_context", category: ToolRead},
	{prefix: "ado_prs", category: ToolRead},
	{prefix: "ado_profiles", category: ToolRead},
	{prefix: "ado_pr", category: ToolRead},
	{prefix: "ado_work_item_types", category: ToolRead},
	{prefix: "ado_work_item", category: ToolRead},

	// Azure DevOps — write tools
	{prefix: "ado_review", category: ToolWrite},
	{prefix: "ado_pr_comment", category: ToolWrite},
	{prefix: "ado_work_item_update", category: ToolWrite},
	{prefix: "ado_work_item_comment", category: ToolWrite},
	{prefix: "ado_related_work_items", category: ToolWrite},
	{prefix: "ado_select_pr", category: ToolWrite},

	// Azure DevOps — admin tools
	{prefix: "ado_profile_use", category: ToolAdmin},

	// Engram — read tools
	{prefix: "engram_mem_context", category: ToolRead},
	{prefix: "engram_mem_search", category: ToolRead},
	{prefix: "engram_mem_get_observation", category: ToolRead},
	{prefix: "engram_mem_suggest_topic_key", category: ToolRead},
	{prefix: "engram_mem_capture_passive", category: ToolRead},

	// Engram — write tools
	{prefix: "engram_mem_save", category: ToolWrite},
	{prefix: "engram_mem_save_prompt", category: ToolWrite},
	{prefix: "engram_mem_update", category: ToolWrite},

	// Engram — admin tools
	{prefix: "engram_mem_session_start", category: ToolAdmin},
	{prefix: "engram_mem_session_end", category: ToolAdmin},
	{prefix: "engram_mem_session_summary", category: ToolAdmin},

	// ywai-kanban — read tools
	{prefix: "ywai-kanban_get_", category: ToolRead},
	{prefix: "ywai-kanban_list_sessions", category: ToolRead},

	// ywai-kanban — write tools
	{prefix: "ywai-kanban_create_", category: ToolWrite},
	{prefix: "ywai-kanban_delete_session", category: ToolWrite},
	{prefix: "ywai-kanban_update_delegation", category: ToolWrite},
	{prefix: "ywai-kanban_add_activity", category: ToolWrite},

	// Delegation — read tools
	{prefix: "delegate", category: ToolRead},
	{prefix: "delegation_list", category: ToolRead},
	{prefix: "delegation_read", category: ToolRead},
}

// ClassifyMCPTool returns the ToolCategory for a given MCP tool name.
// It matches exact tool names first (longest prefix wins), then falls back to prefix patterns.
// Unknown tools default to ToolAdmin (safest).
func ClassifyMCPTool(toolName string) ToolCategory {
	// Find the longest matching prefix
	var match ToolCategory
	var matchLen int

	for _, p := range toolRegistry {
		if strings.HasPrefix(toolName, p.prefix) && len(p.prefix) > matchLen {
			match = p.category
			matchLen = len(p.prefix)
		}
	}

	if matchLen > 0 {
		return match
	}

	// Unknown tools default to Admin (safest).
	return ToolAdmin
}

// MCPConfig holds the parsed MCP permission configuration for an agent.
type MCPConfig struct {
	// Read is the value of the mcp:read key (empty if not set).
	Read string
	// Write is the value of the mcp:write key (empty if not set).
	Write string
	// Admin is the value of the mcp:admin key (empty if not set).
	Admin string
	// Blanket is the value of the mcp key for backward compatibility (empty if not set).
	Blanket string
	// ToolOverrides holds per-tool explicit allow/deny rules.
	ToolOverrides map[string]string
}

// ParseMCPConfig extracts MCP permissions from a flat permission map.
// It picks out mcp:read, mcp:write, mcp:admin, and the blanket mcp key,
// plus any other keys that look like per-tool overrides.
func ParseMCPConfig(perms map[string]string) MCPConfig {
	cfg := MCPConfig{
		ToolOverrides: make(map[string]string),
	}

	for k, v := range perms {
		switch k {
		case "mcp:read":
			cfg.Read = v
		case "mcp:write":
			cfg.Write = v
		case "mcp:admin":
			cfg.Admin = v
		case "mcp":
			cfg.Blanket = v
		default:
			// Store non-standard keys as potential tool overrides.
			// Standard keys are those in permOrder plus mcp:read/write/admin.
			if !isStandardPermissionKey(k) {
				cfg.ToolOverrides[k] = v
			}
		}
	}

	return cfg
}

// isStandardPermissionKey returns true if the key is a known ywai permission key
// (including the new mcp: category keys), and should not be treated as a tool override.
func isStandardPermissionKey(key string) bool {
	switch key {
	case "read", "edit", "write", "bash", "glob", "grep", "lsp",
		"ast_grep", "websearch", "code_search", "webfetch",
		"task", "delegate", "question", "skill", "memory",
		"intercom", "ado", "mcp",
		"mcp:read", "mcp:write", "mcp:admin":
		return true
	default:
		return false
	}
}

// Enforce checks whether a specific MCP tool is allowed based on the agent's
// MCP permission configuration. It applies the following precedence:
//
//  1. Per-tool override > category (mcp:read/write/admin) > blanket mcp
//  2. deny always wins over allow in case of ambiguity
//  3. Unknown tools default to deny
//
// Returns true if the tool is allowed, false if denied.
func (c MCPConfig) Enforce(toolName string) bool {
	// 1. Check explicit per-tool override first
	if val, ok := c.ToolOverrides[toolName]; ok {
		return val == "allow"
	}

	// 2. Classify the tool to determine its category
	cat := ClassifyMCPTool(toolName)

	// 3. Check the category-specific key
	var catVal string
	switch cat {
	case ToolRead:
		catVal = c.Read
	case ToolWrite:
		catVal = c.Write
	case ToolAdmin:
		catVal = c.Admin
	}
	if catVal != "" {
		return catVal == "allow"
	}

	// 4. Fall back to blanket mcp key (backward compatibility)
	if c.Blanket != "" {
		return c.Blanket == "allow"
	}

	// 5. Default deny for safety
	return false
}

// MCPEnforce is a convenience function that combines ParseMCPConfig and Enforce.
// Given a flat permission map and a tool name, it returns true if the tool is allowed.
func MCPEnforce(perms map[string]string, toolName string) bool {
	cfg := ParseMCPConfig(perms)
	return cfg.Enforce(toolName)
}
