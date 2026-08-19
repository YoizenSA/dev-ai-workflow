package config

import (
	"os"
	"path/filepath"
	"testing"
)

// The repo marker used to be a skills subdirectory name, which any machine with
// a personal ~/skills folder satisfies. One did: $HOME was identified as the
// checkout, so a server started from home read the user's unrelated ~/skills and
// the UI reported "No skills found" while the CLI, run from the repo, listed 18.
func TestIsOurRepoByPath(t *testing.T) {
	t.Run("a checkout is identified by its module path", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "go.mod"), "module github.com/Yoizen/dev-ai-workflow/ywai\n")
		if !IsOurRepoByPath(dir) {
			t.Error("the repo root must be recognized")
		}
	})

	t.Run("also one level down, where the module actually lives", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "ywai"), 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(dir, "ywai", "go.mod"), "module github.com/Yoizen/dev-ai-workflow/ywai\n")
		if !IsOurRepoByPath(dir) {
			t.Error("the parent of the module dir must be recognized")
		}
	})

	t.Run("a home directory with a skills folder is not the repo", func(t *testing.T) {
		home := t.TempDir()
		for _, name := range []string{"_shared", "angular", "react-19"} {
			if err := os.MkdirAll(filepath.Join(home, "skills", name), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if IsOurRepoByPath(home) {
			t.Error("a personal skills folder must not be mistaken for the checkout — it redirects where skills, agents and workflows are read from")
		}
	})

	t.Run("another Go project is not the repo", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/someone/else\n")
		if IsOurRepoByPath(dir) {
			t.Error("an unrelated module must not match")
		}
	})

	t.Run("empty and missing paths", func(t *testing.T) {
		if IsOurRepoByPath("") || IsOurRepoByPath(filepath.Join(t.TempDir(), "absent")) {
			t.Error("nothing to identify means not our repo")
		}
	})
}

func TestOpenCodeConfigDirHonorsEnvironmentOverride(t *testing.T) {
	override := filepath.Join(t.TempDir(), "opencode")
	t.Setenv("OPENCODE_CONFIG_DIR", override)

	if got := OpenCodeConfigDir(); got != override {
		t.Fatalf("OpenCodeConfigDir() = %q, want %q", got, override)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
