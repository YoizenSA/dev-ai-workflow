package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/ledger"
)

func chdirTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(old)
	})
	return dir
}

func execRoot(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	return out.String(), err
}

func TestLedgerNoteRequiresSomethingToRecord(t *testing.T) {
	chdirTemp(t)
	_, err := execRoot(t, "ledger", "note")
	if err == nil {
		t.Fatal("empty note must fail")
	}
	if !strings.Contains(err.Error(), "nothing to record") {
		t.Fatalf("got %v", err)
	}
}

func TestLedgerNoteAndSeamRoundTrip(t *testing.T) {
	dir := chdirTemp(t)

	if _, err := execRoot(t, "ledger", "note", "--goal", "land work-ledger", "--next", "write tests"); err != nil {
		t.Fatalf("note: %v", err)
	}

	l, err := ledger.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if l.Goal != "land work-ledger" || l.Next != "write tests" {
		t.Fatalf("ledger = %+v", l)
	}

	out, err := execRoot(t, "ledger", "seam")
	if err != nil {
		t.Fatalf("seam: %v", err)
	}
	if !strings.Contains(out, "land work-ledger") {
		t.Fatalf("seam output: %q", out)
	}
}

func TestLedgerShipCLIRejectsLeak(t *testing.T) {
	dir := chdirTemp(t)
	path := filepath.Join(dir, "out.md")
	if err := os.WriteFile(path, []byte("// ledger: Goal=nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := execRoot(t, "ledger", "ship", path); err == nil {
		t.Fatal("ship must reject leaked inner register")
	}
}
