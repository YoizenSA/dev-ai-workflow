package agents

import (
	"testing"
)

// ---------- Classification tests ----------

func TestClassifyMCPTool_CodeGraph(t *testing.T) {
	tools := []string{
		"codegraph_codegraph_search",
		"codegraph_codegraph_node",
		"codegraph_codegraph_callers",
		"codegraph_codegraph_callees",
		"codegraph_codegraph_context",
		"codegraph_codegraph_explore",
		"codegraph_codegraph_files",
		"codegraph_codegraph_impact",
		"codegraph_codegraph_status",
		"codegraph_codegraph_trace",
	}
	for _, tool := range tools {
		t.Run(tool, func(t *testing.T) {
			if got := ClassifyMCPTool(tool); got != ToolRead {
				t.Errorf("ClassifyMCPTool(%q) = %v (%s), want ToolRead", tool, got, got)
			}
		})
	}
}

func TestClassifyMCPTool_Context7(t *testing.T) {
	tools := []string{
		"context7_query-docs",
		"context7_resolve-library-id",
		"context7_something_else",
	}
	for _, tool := range tools {
		t.Run(tool, func(t *testing.T) {
			if got := ClassifyMCPTool(tool); got != ToolRead {
				t.Errorf("ClassifyMCPTool(%q) = %v (%s), want ToolRead", tool, got, got)
			}
		})
	}
}

func TestClassifyMCPTool_ADO_Read(t *testing.T) {
	tools := []string{
		"ado_pr",
		"ado_pr_diff",
		"ado_pr_file",
		"ado_pr_threads",
		"ado_prs",
		"ado_profiles",
		"ado_pr_review_context",
		"ado_work_item",
		"ado_work_item_types",
	}
	for _, tool := range tools {
		t.Run(tool, func(t *testing.T) {
			if got := ClassifyMCPTool(tool); got != ToolRead {
				t.Errorf("ClassifyMCPTool(%q) = %v (%s), want ToolRead", tool, got, got)
			}
		})
	}
}

func TestClassifyMCPTool_ADO_Write(t *testing.T) {
	tools := []string{
		"ado_review",
		"ado_pr_comment",
		"ado_work_item_update",
		"ado_work_item_comment",
		"ado_related_work_items",
		"ado_select_pr",
	}
	for _, tool := range tools {
		t.Run(tool, func(t *testing.T) {
			if got := ClassifyMCPTool(tool); got != ToolWrite {
				t.Errorf("ClassifyMCPTool(%q) = %v (%s), want ToolWrite", tool, got, got)
			}
		})
	}
}

func TestClassifyMCPTool_ADO_Admin(t *testing.T) {
	tools := []string{
		"ado_profile_use",
	}
	for _, tool := range tools {
		t.Run(tool, func(t *testing.T) {
			if got := ClassifyMCPTool(tool); got != ToolAdmin {
				t.Errorf("ClassifyMCPTool(%q) = %v (%s), want ToolAdmin", tool, got, got)
			}
		})
	}
}

func TestClassifyMCPTool_Engram_Read(t *testing.T) {
	tools := []string{
		"engram_mem_context",
		"engram_mem_search",
		"engram_mem_get_observation",
		"engram_mem_suggest_topic_key",
		"engram_mem_capture_passive",
	}
	for _, tool := range tools {
		t.Run(tool, func(t *testing.T) {
			if got := ClassifyMCPTool(tool); got != ToolRead {
				t.Errorf("ClassifyMCPTool(%q) = %v (%s), want ToolRead", tool, got, got)
			}
		})
	}
}

func TestClassifyMCPTool_Engram_Write(t *testing.T) {
	tools := []string{
		"engram_mem_save",
		"engram_mem_save_prompt",
		"engram_mem_update",
	}
	for _, tool := range tools {
		t.Run(tool, func(t *testing.T) {
			if got := ClassifyMCPTool(tool); got != ToolWrite {
				t.Errorf("ClassifyMCPTool(%q) = %v (%s), want ToolWrite", tool, got, got)
			}
		})
	}
}

func TestClassifyMCPTool_Engram_Admin(t *testing.T) {
	tools := []string{
		"engram_mem_session_start",
		"engram_mem_session_end",
		"engram_mem_session_summary",
	}
	for _, tool := range tools {
		t.Run(tool, func(t *testing.T) {
			if got := ClassifyMCPTool(tool); got != ToolAdmin {
				t.Errorf("ClassifyMCPTool(%q) = %v (%s), want ToolAdmin", tool, got, got)
			}
		})
	}
}

func TestClassifyMCPTool_Kanban_Read(t *testing.T) {
	tools := []string{
		"ywai-kanban_kanban_get_board",
		"ywai-kanban_kanban_get_activities",
		"ywai-kanban_kanban_get_graph",
		"ywai-kanban_kanban_get_pending_decisions",
		"ywai-kanban_kanban_get_ui_url",
		"ywai-kanban_kanban_list_sessions",
	}
	for _, tool := range tools {
		t.Run(tool, func(t *testing.T) {
			if got := ClassifyMCPTool(tool); got != ToolRead {
				t.Errorf("ClassifyMCPTool(%q) = %v (%s), want ToolRead", tool, got, got)
			}
		})
	}
}

func TestClassifyMCPTool_Kanban_Write(t *testing.T) {
	tools := []string{
		"ywai-kanban_kanban_create_delegation",
		"ywai-kanban_kanban_create_session",
		"ywai-kanban_kanban_delete_session",
		"ywai-kanban_kanban_update_delegation",
		"ywai-kanban_kanban_add_activity",
	}
	for _, tool := range tools {
		t.Run(tool, func(t *testing.T) {
			if got := ClassifyMCPTool(tool); got != ToolWrite {
				t.Errorf("ClassifyMCPTool(%q) = %v (%s), want ToolWrite", tool, got, got)
			}
		})
	}
}

func TestClassifyMCPTool_Delegation_Read(t *testing.T) {
	tools := []string{
		"delegate",
		"delegation_list",
		"delegation_read",
	}
	for _, tool := range tools {
		t.Run(tool, func(t *testing.T) {
			if got := ClassifyMCPTool(tool); got != ToolRead {
				t.Errorf("ClassifyMCPTool(%q) = %v (%s), want ToolRead", tool, got, got)
			}
		})
	}
}

func TestClassifyMCPTool_Unknown_DefaultsToAdmin(t *testing.T) {
	tools := []string{
		"some_random_tool",
		"unknown_mcp_service",
		"custom_tool_v1",
	}
	for _, tool := range tools {
		t.Run(tool, func(t *testing.T) {
			if got := ClassifyMCPTool(tool); got != ToolAdmin {
				t.Errorf("ClassifyMCPTool(%q) = %v (%s), want ToolAdmin", tool, got, got)
			}
		})
	}
}

// ---------- Precedence tests ----------

func TestMCPEnforce_PerToolOverrideBeatsCategory(t *testing.T) {
	// mcp:write is deny, but tool is explicitly allowed
	perms := map[string]string{
		"mcp:write": "deny",
		"ado_review": "allow", // explicit override
	}
	if !MCPEnforce(perms, "ado_review") {
		t.Error("MCPEnforce: per-tool override 'allow' should beat category 'deny'")
	}
}

func TestMCPEnforce_CategoryBeatsBlanket(t *testing.T) {
	// mcp is allow, but mcp:write is deny
	perms := map[string]string{
		"mcp":       "allow",
		"mcp:write": "deny",
	}
	if MCPEnforce(perms, "ado_review") {
		t.Error("MCPEnforce: category 'deny' should beat blanket 'allow'")
	}
}

func TestMCPEnforce_NoCategoryFallsBackToBlanket(t *testing.T) {
	// Only blanket mcp: allow, no category keys
	perms := map[string]string{
		"mcp": "allow",
	}
	if !MCPEnforce(perms, "ado_review") {
		t.Error("MCPEnforce: should fall back to blanket 'allow' when no category key")
	}
}

func TestMCPEnforce_NoCategoryFallsBackToBlanket_Deny(t *testing.T) {
	// Only blanket mcp: deny
	perms := map[string]string{
		"mcp": "deny",
	}
	if MCPEnforce(perms, "ado_review") {
		t.Error("MCPEnforce: should fall back to blanket 'deny' when no category key")
	}
}

func TestMCPEnforce_ExplicitDenyBeatsExplicitAllow_PerToolOverride(t *testing.T) {
	// Per-tool override explicitly denies, even though category allows
	perms := map[string]string{
		"mcp:write":  "allow",
		"ado_review": "deny",
	}
	if MCPEnforce(perms, "ado_review") {
		t.Error("MCPEnforce: explicit per-tool 'deny' should override category 'allow'")
	}
}

func TestMCPEnforce_ToolClassifiedWrite_MCPWriteDeny(t *testing.T) {
	// mcp:write is deny, tool is classified as Write → deny
	perms := map[string]string{
		"mcp:read":  "allow",
		"mcp:write": "deny",
	}
	if MCPEnforce(perms, "ado_review") {
		t.Error("MCPEnforce: tool classified as Write with mcp:write=deny should be denied")
	}
}

func TestMCPEnforce_ToolClassifiedRead_MCPReadAllow(t *testing.T) {
	// mcp:read is allow, read tool should be allowed
	perms := map[string]string{
		"mcp:read": "allow",
	}
	if !MCPEnforce(perms, "codegraph_search") {
		t.Error("MCPEnforce: tool classified as Read with mcp:read=allow should be allowed")
	}
}

func TestMCPEnforce_UnknownToolDefaultsToDeny(t *testing.T) {
	// No permissions set at all
	perms := map[string]string{}
	if MCPEnforce(perms, "some_unknown_tool") {
		t.Error("MCPEnforce: unknown tool with no permissions should default to deny")
	}
}

func TestMCPEnforce_UnknownToolWithMCPAllow(t *testing.T) {
	// Unknown tool, but mcp is allow → should be allowed
	perms := map[string]string{
		"mcp": "allow",
	}
	if !MCPEnforce(perms, "some_unknown_tool") {
		t.Error("MCPEnforce: unknown tool with blanket 'allow' should be allowed")
	}
}

func TestMCPEnforce_UnknownToolWithMCPReadAllow(t *testing.T) {
	// Unknown tool is classified as Admin, mcp:admin not set but mcp:read is allow
	perms := map[string]string{
		"mcp:read": "allow",
	}
	if MCPEnforce(perms, "some_unknown_tool") {
		t.Error("MCPEnforce: unknown tool (Admin) should not use mcp:read category")
	}
}

// ---------- Backward compatibility tests ----------

func TestMCPEnforce_BackwardCompat_MCPAllow(t *testing.T) {
	// Old-style mcp: allow should allow all tools
	perms := map[string]string{
		"mcp": "allow",
	}
	tools := []string{
		"codegraph_search",
		"ado_review",
		"ado_profile_use",
		"engram_mem_save",
	}
	for _, tool := range tools {
		t.Run(tool, func(t *testing.T) {
			if !MCPEnforce(perms, tool) {
				t.Errorf("MCPEnforce(%q): mcp=allow should allow all tools", tool)
			}
		})
	}
}

func TestMCPEnforce_BackwardCompat_MCPDeny(t *testing.T) {
	// Old-style mcp: deny should deny all tools
	perms := map[string]string{
		"mcp": "deny",
	}
	tools := []string{
		"codegraph_search",
		"ado_review",
		"ado_profile_use",
	}
	for _, tool := range tools {
		t.Run(tool, func(t *testing.T) {
			if MCPEnforce(perms, tool) {
				t.Errorf("MCPEnforce(%q): mcp=deny should deny all tools", tool)
			}
		})
	}
}

func TestMCPEnforce_BackwardCompat_NoMCPKey(t *testing.T) {
	// No mcp key at all should default to deny
	perms := map[string]string{
		"read": "allow",
		"edit": "allow",
	}
	if MCPEnforce(perms, "codegraph_search") {
		t.Error("MCPEnforce: no mcp key should default to deny")
	}
}

func TestMCPEnforce_BackwardCompat_MCPAllowWithOtherPerms(t *testing.T) {
	// mcp: allow alongside other permissions should still work
	perms := map[string]string{
		"read": "allow",
		"edit": "deny",
		"bash": "deny",
		"mcp":  "allow",
	}
	if !MCPEnforce(perms, "ado_pr") {
		t.Error("MCPEnforce: mcp=allow with other perms should allow MCP tools")
	}
}

func TestMCPEnforce_AskAgentConfig(t *testing.T) {
	// Simulate the ask agent's permissions
	perms := map[string]string{
		"mcp:read":  "allow",
		"mcp:write": "deny",
		"mcp:admin": "deny",
		"mcp":       "allow",
	}

	// Read tools should be allowed
	readTools := []string{
		"codegraph_search",
		"context7_query-docs",
		"ado_pr",
		"engram_mem_context",
		"ywai-kanban_kanban_get_board",
		"delegate",
	}
	for _, tool := range readTools {
		t.Run("read_"+tool, func(t *testing.T) {
			if !MCPEnforce(perms, tool) {
				t.Errorf("MCPEnforce(%q): should be allowed as Read tool", tool)
			}
		})
	}

	// Write tools should be denied
	writeTools := []string{
		"ado_review",
		"ado_pr_comment",
		"engram_mem_save",
		"ywai-kanban_kanban_create_delegation",
	}
	for _, tool := range writeTools {
		t.Run("write_"+tool, func(t *testing.T) {
			if MCPEnforce(perms, tool) {
				t.Errorf("MCPEnforce(%q): should be denied as Write tool", tool)
			}
		})
	}

	// Admin tools should be denied
	adminTools := []string{
		"ado_profile_use",
		"engram_mem_session_start",
	}
	for _, tool := range adminTools {
		t.Run("admin_"+tool, func(t *testing.T) {
			if MCPEnforce(perms, tool) {
				t.Errorf("MCPEnforce(%q): should be denied as Admin tool", tool)
			}
		})
	}
}

// ---------- ParseMCPConfig tests ----------

func TestParseMCPConfig_StandardKeys(t *testing.T) {
	perms := map[string]string{
		"read":       "allow",
		"edit":       "deny",
		"mcp":        "allow",
		"mcp:read":   "allow",
		"mcp:write":  "deny",
		"mcp:admin":  "deny",
	}
	cfg := ParseMCPConfig(perms)

	if cfg.Read != "allow" {
		t.Errorf("cfg.Read = %q, want %q", cfg.Read, "allow")
	}
	if cfg.Write != "deny" {
		t.Errorf("cfg.Write = %q, want %q", cfg.Write, "deny")
	}
	if cfg.Admin != "deny" {
		t.Errorf("cfg.Admin = %q, want %q", cfg.Admin, "deny")
	}
	if cfg.Blanket != "allow" {
		t.Errorf("cfg.Blanket = %q, want %q", cfg.Blanket, "allow")
	}

	// Standard keys like "read", "edit" should NOT appear as tool overrides
	if _, ok := cfg.ToolOverrides["read"]; ok {
		t.Error("cfg.ToolOverrides should not contain 'read'")
	}
	if _, ok := cfg.ToolOverrides["edit"]; ok {
		t.Error("cfg.ToolOverrides should not contain 'edit'")
	}
}

func TestParseMCPConfig_ToolOverrides(t *testing.T) {
	perms := map[string]string{
		"mcp:write":          "deny",
		"ado_review":         "allow",
		"ado_profile_use":    "deny",
	}
	cfg := ParseMCPConfig(perms)

	if cfg.ToolOverrides["ado_review"] != "allow" {
		t.Errorf("cfg.ToolOverrides['ado_review'] = %q, want %q", cfg.ToolOverrides["ado_review"], "allow")
	}
	if cfg.ToolOverrides["ado_profile_use"] != "deny" {
		t.Errorf("cfg.ToolOverrides['ado_profile_use'] = %q, want %q", cfg.ToolOverrides["ado_profile_use"], "deny")
	}
}

func TestParseMCPConfig_Empty(t *testing.T) {
	perms := map[string]string{}
	cfg := ParseMCPConfig(perms)

	if cfg.Read != "" {
		t.Errorf("cfg.Read = %q, want empty", cfg.Read)
	}
	if cfg.Write != "" {
		t.Errorf("cfg.Write = %q, want empty", cfg.Write)
	}
	if cfg.Admin != "" {
		t.Errorf("cfg.Admin = %q, want empty", cfg.Admin)
	}
	if cfg.Blanket != "" {
		t.Errorf("cfg.Blanket = %q, want empty", cfg.Blanket)
	}
	if len(cfg.ToolOverrides) != 0 {
		t.Errorf("cfg.ToolOverrides = %v, want empty", cfg.ToolOverrides)
	}
}

// ---------- MCPConfig String tests ----------

func TestToolCategory_String(t *testing.T) {
	tests := []struct {
		cat  ToolCategory
		want string
	}{
		{ToolRead, "read"},
		{ToolWrite, "write"},
		{ToolAdmin, "admin"},
		{ToolCategory(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.cat.String(); got != tt.want {
				t.Errorf("ToolCategory(%d).String() = %q, want %q", tt.cat, got, tt.want)
			}
		})
	}
}

// ---------- Edge cases ----------

func TestClassifyMCPTool_ExactMatchBeatsPrefix(t *testing.T) {
	// "ado_profile_use" is listed as Admin in the registry and "ado_" prefix
	// could match as Read. The longer prefix "ado_profile_use" should win.
	if got := ClassifyMCPTool("ado_profile_use"); got != ToolAdmin {
		t.Errorf("ClassifyMCPTool('ado_profile_use') = %s, want ToolAdmin", got)
	}
}

func TestMCPEnforce_ToolOverrideWithCategoryKey(t *testing.T) {
	// Tool has both an explicit override AND a category key.
	// The override should take precedence.
	perms := map[string]string{
		"mcp:write":  "deny",
		"ado_review": "allow", // explicit override beats mcp:write=deny
	}
	if !MCPEnforce(perms, "ado_review") {
		t.Error("MCPEnforce: explicit per-tool 'allow' should beat category 'deny'")
	}
}

func TestMCPEnforce_ToolOverrideDenyWithCategoryAllow(t *testing.T) {
	// Per-tool override says deny, but category says allow → override wins
	perms := map[string]string{
		"mcp:read":           "allow",
		"codegraph_search":   "deny",
	}
	if MCPEnforce(perms, "codegraph_search") {
		t.Error("MCPEnforce: explicit per-tool 'deny' should beat category 'allow'")
	}
}
