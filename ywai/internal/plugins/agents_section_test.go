package plugins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpsertAgentsSection_AppendsAndReplaces(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(path, []byte("# Curated\n\nExisting content.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := UpsertAgentsSection(path, "jev-gate", "First body."); err != nil {
		t.Fatalf("UpsertAgentsSection() error = %v", err)
	}
	got := readFile(t, path)
	if !strings.Contains(got, "Existing content.") {
		t.Error("append must not destroy the curated content ywai wrote")
	}
	if !strings.Contains(got, "<!-- BEGIN ywai:jev-gate -->") || !strings.Contains(got, "First body.") {
		t.Errorf("section not written: %s", got)
	}

	// Re-running install must update in place, not stack a second copy.
	if err := UpsertAgentsSection(path, "jev-gate", "Second body."); err != nil {
		t.Fatalf("UpsertAgentsSection() second error = %v", err)
	}
	got = readFile(t, path)
	if n := strings.Count(got, "<!-- BEGIN ywai:jev-gate -->"); n != 1 {
		t.Errorf("marker count = %d, want 1 (idempotent upsert)", n)
	}
	if strings.Contains(got, "First body.") || !strings.Contains(got, "Second body.") {
		t.Errorf("body was not replaced: %s", got)
	}
}

func TestUpsertAgentsSection_CreatesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	if err := UpsertAgentsSection(path, "jev-gate", "Body."); err != nil {
		t.Fatalf("UpsertAgentsSection() error = %v", err)
	}
	if !strings.Contains(readFile(t, path), "Body.") {
		t.Error("a missing AGENTS.md should be created, not skipped")
	}
}

func TestRemoveAgentsSection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	if err := os.WriteFile(path, []byte("# Curated\n\nKeep me.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := UpsertAgentsSection(path, "jev-gate", "Body."); err != nil {
		t.Fatal(err)
	}
	if err := RemoveAgentsSection(path, "jev-gate"); err != nil {
		t.Fatalf("RemoveAgentsSection() error = %v", err)
	}
	got := readFile(t, path)
	if strings.Contains(got, "jev-gate") {
		t.Errorf("section survived removal: %s", got)
	}
	if !strings.Contains(got, "Keep me.") {
		t.Error("removal must leave the rest of the file alone")
	}
	// Removing from a file that never had the section is not an error.
	if err := RemoveAgentsSection(filepath.Join(t.TempDir(), "none.md"), "jev-gate"); err != nil {
		t.Errorf("RemoveAgentsSection() on a missing file error = %v", err)
	}
}

// The shipped section is the thing that stops an agent from presenting a
// review it never ran, so its load-bearing rules are pinned here.
func TestJevGateSection_CarriesTheAttributionRules(t *testing.T) {
	body, ok := agentsSections["jev-gate"]
	if !ok {
		t.Fatal("manifest names agentsSection jev-gate but no body is registered")
	}
	for _, want := range []string{
		"clean` is not `approved",
		"did not run",
		"6 of 8",
		"jev_review_diff",
		"jev_check_page",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("jev-gate section missing %q", want)
		}
	}
}

// Every section a manifest entry names must exist, or install fails late with
// a confusing error on a machine that is not this one.
func TestManifestAgentsSectionsResolve(t *testing.T) {
	mf, warnings := LoadManifest()
	if len(warnings) > 0 {
		t.Fatalf("LoadManifest() warnings = %v", warnings)
	}
	for _, e := range mf.Install {
		if e.AgentsSection == "" {
			continue
		}
		if _, ok := agentsSections[e.AgentsSection]; !ok {
			t.Errorf("entry %q names unknown agents section %q", e.ID, e.AgentsSection)
		}
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
