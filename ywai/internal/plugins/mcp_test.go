package plugins

import (
	"path/filepath"
	"testing"
)

// TestRemoveRetiredMCPs covers RemoveRetiredMCPs across agent config formats.
// Retired servers must be deleted from existing configs without disturbing
// sibling entries, or the agent keeps advertising tools that fail on call.
func TestRemoveRetiredMCPs(t *testing.T) {
	t.Run("opencode_removes_entry", func(t *testing.T) {
		path := writeAgentConfig(t, "opencode.json", map[string]any{
			"mcp": map[string]any{
				"ywai-kanban": map[string]any{
					"type":    "local",
					"command": []any{"ywai", "serve", "--mcp-only"},
					"enabled": true,
				},
				"context7": map[string]any{
					"type":    "remote",
					"url":     "https://mcp.context7.com/mcp",
					"enabled": true,
				},
			},
		})

		if _, err := RemoveRetiredMCPs(path, "opencode"); err != nil {
			t.Fatalf("RemoveRetiredMCPs() error = %v", err)
		}
		root := readConfigRoot(t, path)
		mcp, _ := root["mcp"].(map[string]any)
		if _, ok := mcp["ywai-kanban"]; ok {
			t.Fatalf("ywai-kanban still present: %v", mcp["ywai-kanban"])
		}
		if _, ok := mcp["context7"]; !ok {
			t.Fatal("context7 was removed; only ywai-kanban should be deleted")
		}
	})

	t.Run("claude_code_removes_entry", func(t *testing.T) {
		path := writeAgentConfig(t, "claude_desktop_config.json", map[string]any{
			"mcpServers": map[string]any{
				"ywai-kanban": map[string]any{
					"command": "ywai",
					"args":    []any{"serve", "--mcp-only"},
				},
			},
		})
		if _, err := RemoveRetiredMCPs(path, "claude-code"); err != nil {
			t.Fatalf("RemoveRetiredMCPs() error = %v", err)
		}
		root := readConfigRoot(t, path)
		mcp, _ := root["mcpServers"].(map[string]any)
		if _, ok := mcp["ywai-kanban"]; ok {
			t.Fatalf("ywai-kanban still present: %v", mcp["ywai-kanban"])
		}
	})

	t.Run("noop_when_missing", func(t *testing.T) {
		path := writeAgentConfig(t, "opencode.json", map[string]any{})
		if _, err := RemoveRetiredMCPs(path, "opencode"); err != nil {
			t.Fatalf("RemoveRetiredMCPs() error = %v", err)
		}
	})
}

func TestRemoveVisionMCP(t *testing.T) {
	t.Run("opencode_removes_entry", func(t *testing.T) {
		path := writeAgentConfig(t, "opencode.json", map[string]any{
			"mcp": map[string]any{
				"mcp-vision": map[string]any{
					"type":    "local",
					"command": []any{"mcp-vision"},
					"enabled": true,
				},
				"ywai-kanban": map[string]any{
					"type":    "local",
					"command": []any{"ywai", "serve", "--mcp-only"},
					"enabled": true,
				},
			},
		})

		if err := RemoveVisionMCP(path, "opencode"); err != nil {
			t.Fatalf("RemoveVisionMCP() error = %v", err)
		}
		root := readConfigRoot(t, path)
		mcp, _ := root["mcp"].(map[string]any)
		if _, ok := mcp["mcp-vision"]; ok {
			t.Fatalf("mcp-vision still present: %v", mcp["mcp-vision"])
		}
		if _, ok := mcp["ywai-kanban"]; !ok {
			t.Fatal("ywai-kanban was removed; only mcp-vision should be deleted")
		}
	})

	t.Run("noop_when_missing", func(t *testing.T) {
		path := writeAgentConfig(t, "opencode.json", map[string]any{})
		if err := RemoveVisionMCP(path, "opencode"); err != nil {
			t.Fatalf("RemoveVisionMCP() error = %v", err)
		}
	})

	t.Run("claude_code_removes_entry", func(t *testing.T) {
		path := writeAgentConfig(t, "claude_desktop_config.json", map[string]any{
			"mcpServers": map[string]any{
				"mcp-vision": map[string]any{"command": "mcp-vision"},
			},
		})
		if err := RemoveVisionMCP(path, "claude-code"); err != nil {
			t.Fatalf("RemoveVisionMCP() error = %v", err)
		}
		root := readConfigRoot(t, path)
		mcp, _ := root["mcpServers"].(map[string]any)
		if _, ok := mcp["mcp-vision"]; ok {
			t.Fatalf("mcp-vision still present: %v", mcp["mcp-vision"])
		}
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

// Every retired server must be swept in one pass, and the caller needs to know
// what changed — an upgrade that silently rewrites the user's MCP config with
// no output is indistinguishable from one that did nothing.
func TestRemoveRetiredMCPs_RemovesAllAndReports(t *testing.T) {
	path := writeAgentConfig(t, "opencode.json", map[string]any{
		"mcp": map[string]any{
			"ywai-kanban": map[string]any{"type": "local", "enabled": true},
			"ywai-fastfs": map[string]any{"type": "local", "enabled": true},
			"codegraph":   map[string]any{"type": "local", "enabled": true},
		},
		"model": "provider/model",
	})

	removed, err := RemoveRetiredMCPs(path, "opencode")
	if err != nil {
		t.Fatalf("RemoveRetiredMCPs() error = %v", err)
	}
	if len(removed) != 2 {
		t.Fatalf("removed = %v, want both retired servers", removed)
	}

	root := readConfigRoot(t, path)
	mcp, _ := root["mcp"].(map[string]any)
	if _, still := mcp["ywai-fastfs"]; still {
		t.Error("ywai-fastfs survived — the removed feature stays advertised")
	}
	if _, still := mcp["ywai-kanban"]; still {
		t.Error("ywai-kanban survived")
	}
	if _, ok := mcp["codegraph"]; !ok {
		t.Error("a live MCP server must not be collateral damage")
	}
	if root["model"] != "provider/model" {
		t.Error("unrelated top-level keys must be preserved")
	}
}

// A config with nothing retired must not be rewritten at all: touching the file
// on every install churns the user's config and its formatting for no reason.
func TestRemoveRetiredMCPs_NoOpWhenClean(t *testing.T) {
	path := writeAgentConfig(t, "opencode.json", map[string]any{
		"mcp": map[string]any{"codegraph": map[string]any{"enabled": true}},
	})

	removed, err := RemoveRetiredMCPs(path, "opencode")
	if err != nil {
		t.Fatalf("RemoveRetiredMCPs() error = %v", err)
	}
	if len(removed) != 0 {
		t.Errorf("removed = %v, want nothing", removed)
	}
}
