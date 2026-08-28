package ledger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMissingIsEmptyLedger(t *testing.T) {
	dir := t.TempDir()
	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load missing: %v", err)
	}
	if got == nil || got.Goal != "" || len(got.Core) != 0 || got.Next != "" {
		t.Fatalf("missing ledger should be empty, got %+v", got)
	}
}

func TestNoteGoalAndNextRoundTrip(t *testing.T) {
	dir := t.TempDir()
	l, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyNote(l, Note{Goal: "ship the skill", Next: "write tests"}); err != nil {
		t.Fatalf("ApplyNote: %v", err)
	}
	if err := Save(dir, l); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Goal != "ship the skill" {
		t.Fatalf("Goal = %q", got.Goal)
	}
	if got.Next != "write tests" {
		t.Fatalf("Next = %q", got.Next)
	}
}

func TestNoteCheckRequiresBy(t *testing.T) {
	l := &Ledger{}
	err := ApplyNote(l, Note{Check: "tests pass"})
	if err == nil {
		t.Fatal("expected error when --by is empty")
	}
	if !strings.Contains(err.Error(), "--by") {
		t.Fatalf("error should name --by, got %v", err)
	}
	if len(l.Verified) != 0 {
		t.Fatalf("must not write a check without --by: %+v", l.Verified)
	}
}

func TestNoteCheckAppendsWithBy(t *testing.T) {
	l := &Ledger{}
	if err := ApplyNote(l, Note{Check: "layout exists", By: "go test ./internal/skills/"}); err != nil {
		t.Fatal(err)
	}
	if len(l.Verified) != 1 || l.Verified[0].Text != "layout exists" || l.Verified[0].By == "" {
		t.Fatalf("verified = %+v", l.Verified)
	}
}

func TestCoreAdmitsAtMostTwo(t *testing.T) {
	l := &Ledger{}
	if err := ApplyNote(l, Note{Core: "gate"}); err != nil {
		t.Fatal(err)
	}
	if err := ApplyNote(l, Note{Core: "ledger"}); err != nil {
		t.Fatal(err)
	}
	err := ApplyNote(l, Note{Core: "third"})
	if err == nil {
		t.Fatal("third core item must fail without --core-slot")
	}
	if len(l.Core) != 2 {
		t.Fatalf("core len = %d, want 2", len(l.Core))
	}

	if err := ApplyNote(l, Note{Core: "ship", CoreSlot: 2}); err != nil {
		t.Fatalf("swap slot 2: %v", err)
	}
	if l.Core[1] != "ship" {
		t.Fatalf("core[1] = %q, want ship", l.Core[1])
	}
}

func TestOpenAndCloseQuestion(t *testing.T) {
	l := &Ledger{}
	if err := ApplyNote(l, Note{Open: "which host?", SettledBy: "install target"}); err != nil {
		t.Fatal(err)
	}
	if len(l.Open) != 1 || l.Open[0].ID != 1 || l.Open[0].Closed {
		t.Fatalf("open = %+v", l.Open)
	}

	err := ApplyNote(l, Note{Close: 1})
	if err == nil {
		t.Fatal("close without --check/--by must fail")
	}

	if err := ApplyNote(l, Note{Close: 1, Check: "opencode", By: "detected binary"}); err != nil {
		t.Fatal(err)
	}
	if !l.Open[0].Closed {
		t.Fatal("question 1 should be closed")
	}
	if len(l.Verified) != 1 {
		t.Fatalf("close must record a check, got %+v", l.Verified)
	}
}

func TestSeamMissingLedgerIsNotAnError(t *testing.T) {
	out := RenderSeam(&Ledger{})
	if !strings.Contains(strings.ToLower(out), "no ledger") {
		t.Fatalf("empty seam should say no ledger, got %q", out)
	}
}

func TestRenderResumeIncludesGoalAndNext(t *testing.T) {
	out := RenderResume(&Ledger{Goal: "land work-ledger", Next: "write CLI"})
	if !strings.Contains(out, "land work-ledger") {
		t.Fatalf("resume missing goal: %q", out)
	}
	if !strings.Contains(out, "write CLI") {
		t.Fatalf("resume missing next: %q", out)
	}
}

func TestShipRejectsInnerRegisterLeakage(t *testing.T) {
	dir := t.TempDir()
	dirty := filepath.Join(dir, "handoff.md")
	if err := os.WriteFile(dirty, []byte("done\n// ledger: Goal=ship\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ShipFile(dirty); err == nil {
		t.Fatal("ship must reject // ledger: leakage")
	}

	clean := filepath.Join(dir, "clean.md")
	if err := os.WriteFile(clean, []byte("# Feature\n\nImplemented the skill.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ShipFile(clean); err != nil {
		t.Fatalf("clean file: %v", err)
	}
}
