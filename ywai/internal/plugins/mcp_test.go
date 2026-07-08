package plugins

import (
	"path/filepath"
	"reflect"
	"testing"
)

// TestInstallKanbanMCP covers InstallKanbanMCP behavior across agent formats.
//
// The function currently has a bug: if the "ywai-kanban" entry already exists
// in the agent config (e.g. from a previous install that wrote the old
// "daemon --mcp" command), it leaves the entry untouched. Users upgrading
// from older ywai versions end up with a broken MCP server entry.
//
// TDD Red expectations:
//   - Cases A, B, F (current behavior) MUST pass with the code as-is.
//   - Cases C, D, E (migration) MUST fail until migration logic is added.
func TestInstallKanbanMCP(t *testing.T) {
	t.Run("opencode", func(t *testing.T) {
		// Case A — entry missing → must be created with the new command.
		t.Run("creates_entry_when_missing", func(t *testing.T) {
			path := writeAgentConfig(t, "opencode.json", map[string]any{})

			if err := InstallKanbanMCP(path, "opencode"); err != nil {
				t.Fatalf("InstallKanbanMCP() error = %v", err)
			}

			entry := readMCPServer(t, path, "opencode", "ywai-kanban")
			assertOpencodeCommand(t, entry, []any{"ywai", "serve", "--mcp-only"})
		})

		// Case B — entry already correct → must not be modified.
		t.Run("preserves_correct_entry", func(t *testing.T) {
			path := writeAgentConfig(t, "opencode.json", map[string]any{
				"mcp": map[string]any{
					"ywai-kanban": map[string]any{
						"type":    "local",
						"command": []any{"ywai", "serve", "--mcp-only"},
						"enabled": true,
					},
				},
			})

			if err := InstallKanbanMCP(path, "opencode"); err != nil {
				t.Fatalf("InstallKanbanMCP() error = %v", err)
			}

			entry := readMCPServer(t, path, "opencode", "ywai-kanban")
			assertOpencodeCommand(t, entry, []any{"ywai", "serve", "--mcp-only"})
		})

		// Case C — entry has the OLD command (from pre-fix versions) → must migrate.
		// This is the TDD Red case: current code does nothing when the entry exists.
		t.Run("migrates_old_daemon_command", func(t *testing.T) {
			path := writeAgentConfig(t, "opencode.json", map[string]any{
				"mcp": map[string]any{
					"ywai-kanban": map[string]any{
						"type":    "local",
						"command": []any{"ywai", "daemon", "--mcp"},
						"enabled": true,
					},
				},
			})

			if err := InstallKanbanMCP(path, "opencode"); err != nil {
				t.Fatalf("InstallKanbanMCP() error = %v", err)
			}

			entry := readMCPServer(t, path, "opencode", "ywai-kanban")
			assertOpencodeCommand(t, entry, []any{"ywai", "serve", "--mcp-only"})
		})

		// Case D — old format variant: command missing the --mcp flag.
		// Must still be migrated to the new format.
		t.Run("migrates_old_daemon_without_flag", func(t *testing.T) {
			path := writeAgentConfig(t, "opencode.json", map[string]any{
				"mcp": map[string]any{
					"ywai-kanban": map[string]any{
						"type":    "local",
						"command": []any{"ywai", "daemon"},
						"enabled": true,
					},
				},
			})

			if err := InstallKanbanMCP(path, "opencode"); err != nil {
				t.Fatalf("InstallKanbanMCP() error = %v", err)
			}

			entry := readMCPServer(t, path, "opencode", "ywai-kanban")
			assertOpencodeCommand(t, entry, []any{"ywai", "serve", "--mcp-only"})
		})

		// Case E — multiple MCP servers; ywai-kanban has the OLD command,
		// a sibling server has its own (correct) command. The migration must
		// touch only ywai-kanban and leave siblings alone.
		t.Run("migrates_ywai_kanban_only_preserves_siblings", func(t *testing.T) {
			path := writeAgentConfig(t, "opencode.json", map[string]any{
				"mcp": map[string]any{
					"some-other-server": map[string]any{
						"type":    "remote",
						"url":     "https://example.com/mcp",
						"enabled": true,
					},
					"ywai-kanban": map[string]any{
						"type":    "local",
						"command": []any{"ywai", "daemon", "--mcp"},
						"enabled": true,
					},
				},
			})

			if err := InstallKanbanMCP(path, "opencode"); err != nil {
				t.Fatalf("InstallKanbanMCP() error = %v", err)
			}

			root := readConfigRoot(t, path)
			mcp := root["mcp"].(map[string]any)

			// ywai-kanban must be migrated.
			kanban := mcp["ywai-kanban"].(map[string]any)
			assertOpencodeCommand(t, kanban, []any{"ywai", "serve", "--mcp-only"})

			// The sibling server must be untouched.
			sibling := mcp["some-other-server"].(map[string]any)
			if sibling["type"] != "remote" {
				t.Errorf("sibling.type = %v, want remote (must be preserved)", sibling["type"])
			}
			if sibling["url"] != "https://example.com/mcp" {
				t.Errorf("sibling.url = %v, want https://example.com/mcp (must be preserved)", sibling["url"])
			}
		})

		// Case F — config has no "mcp" block → function must create it.
		t.Run("creates_block_when_mcp_missing", func(t *testing.T) {
			path := writeAgentConfig(t, "opencode.json", map[string]any{
				"theme": "dark",
			})

			if err := InstallKanbanMCP(path, "opencode"); err != nil {
				t.Fatalf("InstallKanbanMCP() error = %v", err)
			}

			// Pre-existing key must survive.
			root := readConfigRoot(t, path)
			if root["theme"] != "dark" {
				t.Errorf("theme = %v, want dark (must be preserved)", root["theme"])
			}

			entry := readMCPServer(t, path, "opencode", "ywai-kanban")
			assertOpencodeCommand(t, entry, []any{"ywai", "serve", "--mcp-only"})
		})

		// Edge case — entry is a map but its "command" field is the wrong
		// type (string instead of []any). The type assertion in
		// migrateKanbanEntry fails, so no migration happens and the
		// user's data must NOT be replaced.
		t.Run("leaves_entry_with_wrong_command_type_untouched", func(t *testing.T) {
			path := writeAgentConfig(t, "opencode.json", map[string]any{
				"mcp": map[string]any{
					"ywai-kanban": map[string]any{
						"type":    "local",
						"command": "ywai", // wrong type: string, not array
						"enabled": true,
					},
				},
			})

			if err := InstallKanbanMCP(path, "opencode"); err != nil {
				t.Fatalf("InstallKanbanMCP() error = %v", err)
			}

			entry := readMCPServer(t, path, "opencode", "ywai-kanban")
			cmd, ok := entry["command"].(string)
			if !ok {
				t.Fatalf("entry.command type changed: got %T (%v), want string %q", entry["command"], entry["command"], "ywai")
			}
			if cmd != "ywai" {
				t.Errorf("entry.command = %q, want %q (must be preserved)", cmd, "ywai")
			}
		})

		// Edge case — ywai-kanban entry exists but is a bare string
		// (malformed config). Defensive no-op: the installer must not
		// destroy user-owned data it cannot parse.
		t.Run("preserves_string_entry_defensively", func(t *testing.T) {
			path := writeAgentConfig(t, "opencode.json", map[string]any{
				"mcp": map[string]any{
					"ywai-kanban": "some-string",
				},
			})

			if err := InstallKanbanMCP(path, "opencode"); err != nil {
				t.Fatalf("InstallKanbanMCP() error = %v", err)
			}

			raw := readMCPRaw(t, path, "opencode", "ywai-kanban")
			assertRawValue(t, raw, "some-string")
		})

		// Edge case — ywai-kanban entry is JSON null. Defensive no-op.
		t.Run("preserves_null_entry_defensively", func(t *testing.T) {
			path := writeAgentConfig(t, "opencode.json", map[string]any{
				"mcp": map[string]any{
					"ywai-kanban": nil,
				},
			})

			if err := InstallKanbanMCP(path, "opencode"); err != nil {
				t.Fatalf("InstallKanbanMCP() error = %v", err)
			}

			raw := readMCPRaw(t, path, "opencode", "ywai-kanban")
			if raw != nil {
				t.Errorf("mcp.ywai-kanban = %v (type %T), want nil (defensive no-op)", raw, raw)
			}
		})
	})

	t.Run("claude_code", func(t *testing.T) {
		// Case A — entry missing → must be created.
		t.Run("creates_entry_when_missing", func(t *testing.T) {
			path := writeAgentConfig(t, "settings.json", map[string]any{})

			if err := InstallKanbanMCP(path, "claude-code"); err != nil {
				t.Fatalf("InstallKanbanMCP() error = %v", err)
			}

			entry := readMCPServer(t, path, "claude-code", "ywai-kanban")
			assertClaudeCodeArgs(t, entry, []any{"serve", "--mcp-only"})
		})

		// Case B — entry already correct → must not be modified.
		t.Run("preserves_correct_entry", func(t *testing.T) {
			path := writeAgentConfig(t, "settings.json", map[string]any{
				"mcpServers": map[string]any{
					"ywai-kanban": map[string]any{
						"command": "ywai",
						"args":    []any{"serve", "--mcp-only"},
					},
				},
			})

			if err := InstallKanbanMCP(path, "claude-code"); err != nil {
				t.Fatalf("InstallKanbanMCP() error = %v", err)
			}

			entry := readMCPServer(t, path, "claude-code", "ywai-kanban")
			assertClaudeCodeArgs(t, entry, []any{"serve", "--mcp-only"})
		})

		// Case C — entry has the OLD "args" → must migrate.
		// TDD Red: current code leaves the old "daemon --mcp" args in place.
		t.Run("migrates_old_daemon_args", func(t *testing.T) {
			path := writeAgentConfig(t, "settings.json", map[string]any{
				"mcpServers": map[string]any{
					"ywai-kanban": map[string]any{
						"command": "ywai",
						"args":    []any{"daemon", "--mcp"},
					},
				},
			})

			if err := InstallKanbanMCP(path, "claude-code"); err != nil {
				t.Fatalf("InstallKanbanMCP() error = %v", err)
			}

			entry := readMCPServer(t, path, "claude-code", "ywai-kanban")
			assertClaudeCodeArgs(t, entry, []any{"serve", "--mcp-only"})
		})

		// Case F — config has no "mcpServers" block → function must create it.
		t.Run("creates_block_when_mcp_servers_missing", func(t *testing.T) {
			path := writeAgentConfig(t, "settings.json", map[string]any{
				"theme": "dark",
			})

			if err := InstallKanbanMCP(path, "claude-code"); err != nil {
				t.Fatalf("InstallKanbanMCP() error = %v", err)
			}

			root := readConfigRoot(t, path)
			if root["theme"] != "dark" {
				t.Errorf("theme = %v, want dark (must be preserved)", root["theme"])
			}

			entry := readMCPServer(t, path, "claude-code", "ywai-kanban")
			assertClaudeCodeArgs(t, entry, []any{"serve", "--mcp-only"})
		})

		// Edge case — old format without the --mcp trailing flag.
		// claude-code/pi analog of opencode/migrates_old_daemon_without_flag.
		t.Run("migrates_old_daemon_args_without_flag", func(t *testing.T) {
			path := writeAgentConfig(t, "settings.json", map[string]any{
				"mcpServers": map[string]any{
					"ywai-kanban": map[string]any{
						"command": "ywai",
						"args":    []any{"daemon"},
					},
				},
			})

			if err := InstallKanbanMCP(path, "claude-code"); err != nil {
				t.Fatalf("InstallKanbanMCP() error = %v", err)
			}

			entry := readMCPServer(t, path, "claude-code", "ywai-kanban")
			assertClaudeCodeArgs(t, entry, []any{"serve", "--mcp-only"})
		})

		// Edge case — sibling preservation in mcpServers format.
		// claude-code/pi analog of opencode/migrates_ywai_kanban_only_preserves_siblings.
		t.Run("migrates_ywai_kanban_only_preserves_siblings", func(t *testing.T) {
			path := writeAgentConfig(t, "settings.json", map[string]any{
				"mcpServers": map[string]any{
					"some-other-server": map[string]any{
						"type":    "remote",
						"url":     "https://example.com/mcp",
						"enabled": true,
					},
					"ywai-kanban": map[string]any{
						"command": "ywai",
						"args":    []any{"daemon", "--mcp"},
					},
				},
			})

			if err := InstallKanbanMCP(path, "claude-code"); err != nil {
				t.Fatalf("InstallKanbanMCP() error = %v", err)
			}

			root := readConfigRoot(t, path)
			mcp := root["mcpServers"].(map[string]any)

			// ywai-kanban must be migrated.
			kanban := mcp["ywai-kanban"].(map[string]any)
			assertClaudeCodeArgs(t, kanban, []any{"serve", "--mcp-only"})

			// The sibling server must be untouched.
			sibling := mcp["some-other-server"].(map[string]any)
			if sibling["type"] != "remote" {
				t.Errorf("sibling.type = %v, want remote (must be preserved)", sibling["type"])
			}
			if sibling["url"] != "https://example.com/mcp" {
				t.Errorf("sibling.url = %v, want https://example.com/mcp (must be preserved)", sibling["url"])
			}
			if sibling["enabled"] != true {
				t.Errorf("sibling.enabled = %v, want true (must be preserved)", sibling["enabled"])
			}
		})

		// Edge case — entry is a map but its "args" field is the wrong
		// type (string instead of []any). Defensive no-op: the type
		// assertion in migrateKanbanEntry fails, no migration happens,
		// and the user's data must NOT be replaced.
		t.Run("leaves_entry_with_wrong_args_type_untouched", func(t *testing.T) {
			path := writeAgentConfig(t, "settings.json", map[string]any{
				"mcpServers": map[string]any{
					"ywai-kanban": map[string]any{
						"command": "ywai",
						"args":    "daemon", // wrong type: string, not array
					},
				},
			})

			if err := InstallKanbanMCP(path, "claude-code"); err != nil {
				t.Fatalf("InstallKanbanMCP() error = %v", err)
			}

			entry := readMCPServer(t, path, "claude-code", "ywai-kanban")
			args, ok := entry["args"].(string)
			if !ok {
				t.Fatalf("entry.args type changed: got %T (%v), want string %q", entry["args"], entry["args"], "daemon")
			}
			if args != "daemon" {
				t.Errorf("entry.args = %q, want %q (must be preserved)", args, "daemon")
			}
		})

		// Edge case — ywai-kanban entry exists but is a bare string
		// (malformed config). Defensive no-op: the installer must not
		// destroy user-owned data it cannot parse.
		t.Run("preserves_string_entry_defensively", func(t *testing.T) {
			path := writeAgentConfig(t, "settings.json", map[string]any{
				"mcpServers": map[string]any{
					"ywai-kanban": "some-string",
				},
			})

			if err := InstallKanbanMCP(path, "claude-code"); err != nil {
				t.Fatalf("InstallKanbanMCP() error = %v", err)
			}

			raw := readMCPRaw(t, path, "claude-code", "ywai-kanban")
			assertRawValue(t, raw, "some-string")
		})

		// Edge case — ywai-kanban entry is JSON null. Defensive no-op.
		t.Run("preserves_null_entry_defensively", func(t *testing.T) {
			path := writeAgentConfig(t, "settings.json", map[string]any{
				"mcpServers": map[string]any{
					"ywai-kanban": nil,
				},
			})

			if err := InstallKanbanMCP(path, "claude-code"); err != nil {
				t.Fatalf("InstallKanbanMCP() error = %v", err)
			}

			raw := readMCPRaw(t, path, "claude-code", "ywai-kanban")
			if raw != nil {
				t.Errorf("mcpServers.ywai-kanban = %v (type %T), want nil (defensive no-op)", raw, raw)
			}
		})
	})

	t.Run("pi", func(t *testing.T) {
		// pi uses the same mcpServers format as claude-code. Cover the
		// migration path so a pi user upgrading from an older ywai
		// doesn't get stuck with the broken "daemon --mcp" command.
		t.Run("migrates_old_daemon_args", func(t *testing.T) {
			path := writeAgentConfig(t, "mcp.json", map[string]any{
				"mcpServers": map[string]any{
					"ywai-kanban": map[string]any{
						"command": "ywai",
						"args":    []any{"daemon", "--mcp"},
					},
				},
			})

			if err := InstallKanbanMCP(path, "pi"); err != nil {
				t.Fatalf("InstallKanbanMCP() error = %v", err)
			}

			entry := readMCPServer(t, path, "pi", "ywai-kanban")
			assertClaudeCodeArgs(t, entry, []any{"serve", "--mcp-only"})
		})

		// Edge case — old format without the --mcp trailing flag.
		// pi analog of opencode/migrates_old_daemon_without_flag.
		t.Run("migrates_old_daemon_args_without_flag", func(t *testing.T) {
			path := writeAgentConfig(t, "mcp.json", map[string]any{
				"mcpServers": map[string]any{
					"ywai-kanban": map[string]any{
						"command": "ywai",
						"args":    []any{"daemon"},
					},
				},
			})

			if err := InstallKanbanMCP(path, "pi"); err != nil {
				t.Fatalf("InstallKanbanMCP() error = %v", err)
			}

			entry := readMCPServer(t, path, "pi", "ywai-kanban")
			assertClaudeCodeArgs(t, entry, []any{"serve", "--mcp-only"})
		})

		// Edge case — sibling preservation in mcpServers format.
		// pi analog of opencode/migrates_ywai_kanban_only_preserves_siblings.
		t.Run("migrates_ywai_kanban_only_preserves_siblings", func(t *testing.T) {
			path := writeAgentConfig(t, "mcp.json", map[string]any{
				"mcpServers": map[string]any{
					"some-other-server": map[string]any{
						"type":    "remote",
						"url":     "https://example.com/mcp",
						"enabled": true,
					},
					"ywai-kanban": map[string]any{
						"command": "ywai",
						"args":    []any{"daemon", "--mcp"},
					},
				},
			})

			if err := InstallKanbanMCP(path, "pi"); err != nil {
				t.Fatalf("InstallKanbanMCP() error = %v", err)
			}

			root := readConfigRoot(t, path)
			mcp := root["mcpServers"].(map[string]any)

			kanban := mcp["ywai-kanban"].(map[string]any)
			assertClaudeCodeArgs(t, kanban, []any{"serve", "--mcp-only"})

			sibling := mcp["some-other-server"].(map[string]any)
			if sibling["type"] != "remote" {
				t.Errorf("sibling.type = %v, want remote (must be preserved)", sibling["type"])
			}
			if sibling["url"] != "https://example.com/mcp" {
				t.Errorf("sibling.url = %v, want https://example.com/mcp (must be preserved)", sibling["url"])
			}
			if sibling["enabled"] != true {
				t.Errorf("sibling.enabled = %v, want true (must be preserved)", sibling["enabled"])
			}
		})

		// Edge case — entry is a map but its "args" field is the wrong
		// type. Defensive no-op.
		t.Run("leaves_entry_with_wrong_args_type_untouched", func(t *testing.T) {
			path := writeAgentConfig(t, "mcp.json", map[string]any{
				"mcpServers": map[string]any{
					"ywai-kanban": map[string]any{
						"command": "ywai",
						"args":    "daemon", // wrong type: string, not array
					},
				},
			})

			if err := InstallKanbanMCP(path, "pi"); err != nil {
				t.Fatalf("InstallKanbanMCP() error = %v", err)
			}

			entry := readMCPServer(t, path, "pi", "ywai-kanban")
			args, ok := entry["args"].(string)
			if !ok {
				t.Fatalf("entry.args type changed: got %T (%v), want string %q", entry["args"], entry["args"], "daemon")
			}
			if args != "daemon" {
				t.Errorf("entry.args = %q, want %q (must be preserved)", args, "daemon")
			}
		})

		// Edge case — ywai-kanban entry is a bare string. Defensive no-op.
		t.Run("preserves_string_entry_defensively", func(t *testing.T) {
			path := writeAgentConfig(t, "mcp.json", map[string]any{
				"mcpServers": map[string]any{
					"ywai-kanban": "some-string",
				},
			})

			if err := InstallKanbanMCP(path, "pi"); err != nil {
				t.Fatalf("InstallKanbanMCP() error = %v", err)
			}

			raw := readMCPRaw(t, path, "pi", "ywai-kanban")
			assertRawValue(t, raw, "some-string")
		})

		// Edge case — ywai-kanban entry is JSON null. Defensive no-op.
		t.Run("preserves_null_entry_defensively", func(t *testing.T) {
			path := writeAgentConfig(t, "mcp.json", map[string]any{
				"mcpServers": map[string]any{
					"ywai-kanban": nil,
				},
			})

			if err := InstallKanbanMCP(path, "pi"); err != nil {
				t.Fatalf("InstallKanbanMCP() error = %v", err)
			}

			raw := readMCPRaw(t, path, "pi", "ywai-kanban")
			if raw != nil {
				t.Errorf("mcpServers.ywai-kanban = %v (type %T), want nil (defensive no-op)", raw, raw)
			}
		})
	})
}

func TestInstallVisionMCP(t *testing.T) {
	t.Run("opencode", func(t *testing.T) {
		t.Run("creates_entry_when_missing", func(t *testing.T) {
			path := writeAgentConfig(t, "opencode.json", map[string]any{})

			if err := InstallVisionMCP(path, "opencode"); err != nil {
				t.Fatalf("InstallVisionMCP() error = %v", err)
			}

			entry := readMCPServer(t, path, "opencode", "mcp-vision")
			if entry == nil {
				t.Fatal("mcp-vision entry is nil")
			}
			got := entry["command"].([]any)
			want := []any{"mcp-vision"}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("entry.command = %v, want %v", got, want)
			}
			if typ, _ := entry["type"].(string); typ != "local" {
				t.Errorf("entry.type = %q, want \"local\"", typ)
			}
			if enabled, _ := entry["enabled"].(bool); !enabled {
				t.Errorf("entry.enabled = %v, want true", enabled)
			}
		})

		t.Run("preserves_existing_entry", func(t *testing.T) {
			path := writeAgentConfig(t, "opencode.json", map[string]any{
				"mcp": map[string]any{
					"mcp-vision": map[string]any{
						"type":    "local",
						"command": []any{"mcp-vision"},
						"enabled": true,
					},
				},
			})

			if err := InstallVisionMCP(path, "opencode"); err != nil {
				t.Fatalf("InstallVisionMCP() error = %v", err)
			}

			entry := readMCPServer(t, path, "opencode", "mcp-vision")
			if entry == nil {
				t.Fatal("mcp-vision entry is nil after preserve")
			}
			got := entry["command"].([]any)
			want := []any{"mcp-vision"}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("entry.command = %v, want %v", got, want)
			}
		})
	})

	t.Run("claude_code", func(t *testing.T) {
		t.Run("creates_entry_when_missing", func(t *testing.T) {
			path := writeAgentConfig(t, "claude_desktop_config.json", map[string]any{})

			if err := InstallVisionMCP(path, "claude-code"); err != nil {
				t.Fatalf("InstallVisionMCP() error = %v", err)
			}

			entry := readMCPServer(t, path, "claude-code", "mcp-vision")
			if entry == nil {
				t.Fatal("mcp-vision entry is nil")
			}
			if cmd, _ := entry["command"].(string); cmd != "mcp-vision" {
				t.Errorf("entry.command = %q, want \"mcp-vision\"", cmd)
			}
		})

		t.Run("preserves_existing_entry", func(t *testing.T) {
			path := writeAgentConfig(t, "claude_desktop_config.json", map[string]any{
				"mcpServers": map[string]any{
					"mcp-vision": map[string]any{
						"command": "mcp-vision",
					},
				},
			})

			if err := InstallVisionMCP(path, "claude-code"); err != nil {
				t.Fatalf("InstallVisionMCP() error = %v", err)
			}

			entry := readMCPServer(t, path, "claude-code", "mcp-vision")
			if entry == nil {
				t.Fatal("mcp-vision entry is nil after preserve")
			}
			if cmd, _ := entry["command"].(string); cmd != "mcp-vision" {
				t.Errorf("entry.command = %q, want \"mcp-vision\"", cmd)
			}
		})
	})

	t.Run("pi", func(t *testing.T) {
		t.Run("creates_entry_when_missing", func(t *testing.T) {
			path := writeAgentConfig(t, "pi.json", map[string]any{})

			if err := InstallVisionMCP(path, "pi"); err != nil {
				t.Fatalf("InstallVisionMCP() error = %v", err)
			}

			entry := readMCPServer(t, path, "pi", "mcp-vision")
			if entry == nil {
				t.Fatal("mcp-vision entry is nil")
			}
			if cmd, _ := entry["command"].(string); cmd != "mcp-vision" {
				t.Errorf("entry.command = %q, want \"mcp-vision\"", cmd)
			}
		})
	})
}

// ─── helpers ───────────────────────────────────────────────────────────────

// writeAgentConfig writes a JSON config file inside a fresh temp dir and
// returns its absolute path. The file is created with the content from data
// (a map[string]any shaped as the agent would write it).
func writeAgentConfig(t *testing.T, filename string, data map[string]any) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, filename)
	writeJSON(t, path, data)
	return path
}

// readConfigRoot reads the entire config back as map[string]any.
func readConfigRoot(t *testing.T, path string) map[string]any {
	t.Helper()
	var root map[string]any
	readJSON(t, path, &root)
	return root
}

// readMCPServer returns the mcpServers[name] (claude-code / pi) or mcp[name]
// (opencode / default) entry as a map.
func readMCPServer(t *testing.T, path, agentName, name string) map[string]any {
	t.Helper()
	root := readConfigRoot(t, path)

	key := mcpConfigKey(agentName)
	mcp, ok := root[key].(map[string]any)
	if !ok {
		t.Fatalf("config has no %q block; got keys %v", key, rootKeys(root))
	}
	entry, ok := mcp[name].(map[string]any)
	if !ok {
		t.Fatalf("%q has no %q entry; got entries %v", key, name, mapKeys(mcp))
	}
	return entry
}

// assertOpencodeCommand asserts the entry's "command" field equals want.
// (Used for the opencode "mcp" format where command is a full argv array.)
func assertOpencodeCommand(t *testing.T, entry map[string]any, want []any) {
	t.Helper()
	got, ok := entry["command"].([]any)
	if !ok {
		t.Fatalf("entry.command is not []any; got %T (%v)", entry["command"], entry["command"])
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("entry.command = %v, want %v (migration did not happen)", got, want)
	}
}

// assertClaudeCodeArgs asserts the entry's "args" field equals want and the
// "command" field is "ywai". (Used for the claude-code / pi "mcpServers"
// format where command is a string and args is the argv array.)
func assertClaudeCodeArgs(t *testing.T, entry map[string]any, want []any) {
	t.Helper()
	if cmd, _ := entry["command"].(string); cmd != "ywai" {
		t.Errorf("entry.command = %q, want \"ywai\"", cmd)
	}
	got, ok := entry["args"].([]any)
	if !ok {
		t.Fatalf("entry.args is not []any; got %T (%v)", entry["args"], entry["args"])
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("entry.args = %v, want %v (migration did not happen)", got, want)
	}
}

func rootKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func mapKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// readMCPRaw returns the raw value of mcp[name] without asserting a specific
// type. Use this to verify defensive behavior where a malformed ywai-kanban
// entry (string, null, array, ...) is preserved as-is instead of being
// replaced.
func readMCPRaw(t *testing.T, path, agentName, name string) any {
	t.Helper()
	root := readConfigRoot(t, path)
	key := mcpConfigKey(agentName)
	mcp, ok := root[key].(map[string]any)
	if !ok {
		t.Fatalf("config has no %q block; got keys %v", key, rootKeys(root))
	}
	return mcp[name]
}

// assertRawValue asserts the raw mcp value equals want using reflect.DeepEqual,
// so it works uniformly for strings, numbers, nil, arrays, and maps.
func assertRawValue(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("value = %v (type %T), want %v (type %T)", got, got, want, want)
	}
}
