package agents

import (
	"os"
	"path/filepath"
	"testing"
)

// TestE2EFullInstallShape renders every real profile + delegations into a
// scratch dir exactly as the installer would, for external opencode validation.
func TestE2EFullInstallShape(t *testing.T) {
	profiles, err := LoadProfiles("../../agents")
	if err != nil {
		t.Fatal(err)
	}
	outDir := os.Getenv("YWAI_E2E_AGENTS_DIR")
	if outDir == "" {
		t.Skip("YWAI_E2E_AGENTS_DIR not set")
	}
	os.MkdirAll(outDir, 0o755)
	if err := InstallOpenCodeMarkdown(outDir, profiles, true); err != nil {
		t.Fatal(err)
	}
	doc, err := LoadDelegations("../../agents")
	if err != nil {
		t.Fatal(err)
	}
	// opencode.json path in the same tree for the delegation task maps.
	cfg := filepath.Join(filepath.Dir(outDir), "opencode.json")
	if err := ApplyDelegations(cfg, outDir, doc); err != nil {
		t.Fatal(err)
	}
}
