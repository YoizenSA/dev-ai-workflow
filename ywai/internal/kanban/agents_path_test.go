package kanban

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolveAgentFile_Flat covers the legacy layout where agents live
// directly under the agents dir (e.g. agents/gentle-orchestrator.md).
func TestResolveAgentFile_Flat(t *testing.T) {
	dir := t.TempDir()
	agentPath := filepath.Join(dir, "gentle-orchestrator.md")
	if err := os.WriteFile(agentPath, []byte("# flat agent"), 0o644); err != nil {
		t.Fatalf("write flat agent: %v", err)
	}

	got := resolveAgentFile(dir, "gentle-orchestrator")
	if got != agentPath {
		t.Errorf("resolveAgentFile(flat) = %q, want %q", got, agentPath)
	}
}

// TestResolveAgentFile_Nested reproduces the bug where agents installed into
// group subdirectories (agents/social-refactor/migration-orchestrator.md,
// agents/core/architect.md, agents/qa-automation/qa-analyst.md) return 404 in
// GetAgent/GetAgentPermissions because the handler only looked for the flat
// {dir}/{name}.md path. ListAgents already scans subdirs, so the list showed
// them but selecting one failed.
func TestResolveAgentFile_Nested(t *testing.T) {
	dir := t.TempDir()

	cases := []struct {
		group string
		name  string
	}{
		{"social-refactor", "migration-orchestrator"},
		{"core", "architect"},
		{"qa-automation", "qa-analyst"},
	}
	for _, c := range cases {
		nested := filepath.Join(dir, c.group, c.name+".md")
		if err := os.MkdirAll(filepath.Dir(nested), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", c.group, err)
		}
		if err := os.WriteFile(nested, []byte("# "+c.name), 0o644); err != nil {
			t.Fatalf("write nested agent %s: %v", c.name, err)
		}

		got := resolveAgentFile(dir, c.name)
		if got != nested {
			t.Errorf("resolveAgentFile(%q) = %q, want %q", c.name, got, nested)
		}
	}
}

// TestResolveAgentFile_NotFound returns "" when no agent matches, so callers
// can map it to a 404 instead of silently serving a missing file.
func TestResolveAgentFile_NotFound(t *testing.T) {
	dir := t.TempDir()
	if got := resolveAgentFile(dir, "does-not-exist"); got != "" {
		t.Errorf("resolveAgentFile(missing) = %q, want empty", got)
	}
}
