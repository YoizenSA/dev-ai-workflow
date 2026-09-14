package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallSkillCommandsFromCopiesOnlyMarkdown(t *testing.T) {
	cmds := t.TempDir()
	for name, body := range map[string]string{
		"retro.md":  "retro body\n",
		"notes.txt": "skipped\n",
	} {
		if err := os.WriteFile(filepath.Join(cmds, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	a := t.TempDir()
	b := t.TempDir()
	if err := installSkillCommandsFrom(cmds, a, b); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{a, b} {
		got, err := os.ReadFile(filepath.Join(dir, "retro.md"))
		if err != nil {
			t.Fatalf("%s: %v", dir, err)
		}
		if string(got) != "retro body\n" {
			t.Fatalf("%s: wrong body %q", dir, got)
		}
		if _, err := os.Stat(filepath.Join(dir, "notes.txt")); !os.IsNotExist(err) {
			t.Fatalf("%s: non-md file must be skipped", dir)
		}
	}
}

func TestInstallSkillCommandsMissingSkillIsError(t *testing.T) {
	if err := InstallSkillCommands("no-such-skill-xyz", t.TempDir()); err == nil {
		t.Fatal("expected error for unknown skill")
	}
}
