package plugins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallCommandMarkdownWritesDest(t *testing.T) {
	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "learn-ywai.md")
	body := "---\ndescription: Teach ywai from official docs\n---\n\nLoad learn-ywai.\n"
	if err := os.WriteFile(src, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	dest := t.TempDir()
	if err := installCommandMarkdown(src, dest, LearnYwaiCommandName); err != nil {
		t.Fatalf("install: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dest, LearnYwaiCommandName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "Load learn-ywai") {
		t.Fatalf("dest missing command body:\n%s", got)
	}
}

func TestInstallLearnYwaiCommandToDirsCopiesEveryDest(t *testing.T) {
	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "learn-ywai.md")
	if err := os.WriteFile(src, []byte("teach\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := t.TempDir()
	b := t.TempDir()
	if err := installLearnYwaiCommandFrom(src, a, b); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{a, b} {
		if _, err := os.Stat(filepath.Join(dir, LearnYwaiCommandName)); err != nil {
			t.Fatalf("%s: %v", dir, err)
		}
	}
}
