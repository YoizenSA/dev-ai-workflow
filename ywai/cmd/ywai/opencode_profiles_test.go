package main

import (
	"os"
	"path/filepath"
	"testing"

	agentprofiles "github.com/Yoizen/dev-ai-workflow/ywai/internal/agents"
)

func TestPruneProfileAgentsRemovesOthers(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "dev.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "devops.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	keep := map[string]agentprofiles.AgentProfile{
		"dev": {Name: "dev"},
	}
	pruneProfileAgents(dir, keep)
	if _, err := os.Stat(filepath.Join(dir, "dev.md")); err != nil {
		t.Fatal("kept agent removed")
	}
	if _, err := os.Stat(filepath.Join(dir, "devops.md")); err == nil {
		t.Fatal("extra agent not pruned")
	}
}
