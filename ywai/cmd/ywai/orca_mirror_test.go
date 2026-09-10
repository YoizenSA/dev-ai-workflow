package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Orca runs OpenCode against its own shared config dir, so a `ywai update`
// from a plain terminal left Orca on stale providers. The user config is
// copied over Orca's, keeping the replaced file as .bak.
func TestMirrorOpenCodeToOrca_CopiesUserConfig(t *testing.T) {
	home := t.TempDir()
	pinOCEnv(t, home)
	userCfg := filepath.Join(home, ".config", "opencode", "opencode.json")
	orcaDir := filepath.Join(home, ".config", "orca", "opencode-hooks", "shared")
	for _, d := range []string{filepath.Dir(userCfg), orcaDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	user := `{"providers":{"opencode-admin":{"package":"@opencode/ai/providers/openai-compatible"}}}`
	old := `{"provider":{"opencode-admin":{"npm":"@ai-sdk/openai-compatible"}}}`
	if err := os.WriteFile(userCfg, []byte(user), 0o644); err != nil {
		t.Fatal(err)
	}
	orcaCfg := filepath.Join(orcaDir, "opencode.json")
	if err := os.WriteFile(orcaCfg, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	mirrorOpenCodeToOrca(false)

	if got, _ := os.ReadFile(orcaCfg); string(got) != user {
		t.Errorf("Orca config = %s, want the user config %s", got, user)
	}
	if bak, _ := os.ReadFile(orcaCfg + ".bak"); string(bak) != old {
		t.Errorf("the replaced Orca config must be kept as .bak, got %s", bak)
	}
}

func TestMirrorOpenCodeToOrca_NoOrcaIsNoop(t *testing.T) {
	home := t.TempDir()
	pinOCEnv(t, home)
	userCfg := filepath.Join(home, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(userCfg), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userCfg, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	mirrorOpenCodeToOrca(false)

	if _, err := os.Stat(filepath.Join(home, ".config", "orca")); !os.IsNotExist(err) {
		t.Errorf("without Orca installed nothing must be created, stat err = %v", err)
	}
}
