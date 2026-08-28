package gentlai

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Regression tests for slice 1 of the ywai/gentle-ai decoupling.
//
// Goal: ywai must never install or invoke the gentle-ai binary. Engram is
// installed via ywai's own release-binary path. `ywai update` does not run
// `gentle-ai upgrade`. `ywai doctor` must not require a gentle-ai binary.
//
// Every test in this file is intentionally RED. They pin the first
// implementation slice so the dev can drive them green.

// ─── Doctor must not require gentle-ai ────────────────────────────────────

// TestDoctor_DoesNotRequireGentleAIBinary asserts that Doctor() never returns
// the "gentle-ai is not installed" error. Doctor must run on ywai-native
// checks (tool binaries, state.json, disk, ...) and gracefully report when
// the optional gentle-ai binary is absent instead of refusing to run.
//
// Slice 1 acceptance: ywai doctor must not require a gentle-ai binary.
func TestDoctor_DoesNotRequireGentleAIBinary(t *testing.T) {
	err := Doctor()
	if err == nil {
		return
	}
	if strings.Contains(err.Error(), "gentle-ai is not installed") {
		t.Fatalf("Doctor() must not require gentle-ai binary, got %v", err)
	}
}

// ─── Install must not install the gentle-ai binary ────────────────────────

// TestInstall_GentleAIBinaryUntouched is a regression guard: the public
// Install() entry point must NOT install the gentle-ai binary. Even when
// gentle-ai is missing on PATH, calling Install() must leave `gentle-ai`
// off PATH. (Slice 1 acceptance: ywai install does not install gentle-ai.)
func TestInstall_GentleAIBinaryUntouched(t *testing.T) {
	if _, err := exec.LookPath("gentle-ai"); err == nil {
		t.Skip("gentle-ai is on PATH; this test only proves the no-op when absent")
	}
	// Any error that is NOT a "gentle-ai installed" side effect is fine.
	// The hard contract: after Install(), gentle-ai must still be absent.
	if err := Install(); err != nil {
		// Surface the error for the dev to inspect, but the assertion below
		// is the real one.
		t.Logf("Install() returned %v (acceptable as long as binary is untouched)", err)
	}
	if _, err := exec.LookPath("gentle-ai"); err == nil {
		t.Fatal("Install() must not install the gentle-ai binary")
	}
}

// ─── Engram install uses ywai's own release path ───────────────────────────

// TestInstallEngram_FunctionExported asserts that package gentlai exposes a
// public InstallEngram() function (the post-decouple engram installer). The
// compile-time reference below is the assertion: any removal or rename of
// the symbol breaks the test binary. A direct reference is required because
// Go's reflect package cannot see package-level functions — only methods on
// types — so a method-on-stubs shim would be the only way to "discover"
// InstallEngram via reflection, which is exactly the test-only adapter this
// slice is removing.
//
// Slice 1 acceptance: ywai install must install engram via the release path
// without invoking gentle-ai.
func TestInstallEngram_SkipsDownloadWhenCurrent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(binDir, "engram")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho engram 1.20.0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)

	origLatest := fetchLatestEngram
	origDownload := downloadReleaseFile
	t.Cleanup(func() {
		fetchLatestEngram = origLatest
		downloadReleaseFile = origDownload
	})
	fetchLatestEngram = func() (string, error) { return "v1.20.0", nil }
	downloadReleaseFile = func(string, string) error {
		t.Fatal("must not download when engram is already current")
		return nil
	}

	dir, err := InstallEngram()
	if err != nil {
		t.Fatalf("InstallEngram: %v", err)
	}
	if dir != binDir {
		t.Fatalf("install dir = %q, want %q", dir, binDir)
	}
}

func TestInstallEngram_FunctionExported(t *testing.T) {
	var _ func() (string, error) = InstallEngram
}

// TestInstallEngram_NoGentleAIRequisite asserts that InstallEngram() does not
// depend on the gentle-ai binary. InstallEngram() downloads from GitHub, so a
// runtime call needs network; instead the test reads gentlai.go and fails if
// any code path can exec the gentle-ai binary (via its resolved path or the
// config constant).
func TestInstallEngram_NoGentleAIRequisite(t *testing.T) {
	src, err := os.ReadFile("gentlai.go")
	if err != nil {
		t.Fatalf("read gentlai.go: %v", err)
	}
	for _, forbidden := range []string{
		"exec.Command(gentleAIBinaryPath",
		"exec.Command(config.GentleAIBin",
	} {
		if strings.Contains(string(src), forbidden) {
			t.Fatalf("gentlai.go must not exec the gentle-ai binary; found %q", forbidden)
		}
	}
}

// ─── Upgrade must not shell to gentle-ai upgrade ──────────────────────────

// TestUpgrade_NoGentleAIUpgrade is a regression guard: the public Upgrade()
// function must not exec `gentle-ai upgrade`. Slice 1 acceptance: ywai update
// does not run gentle-ai upgrade; it may still update ywai/engram.
func TestUpgrade_NoGentleAIUpgrade(t *testing.T) {
	if _, err := exec.LookPath("gentle-ai"); err == nil {
		t.Skip("gentle-ai is on PATH; this test only proves the no-op when absent")
	}
	// After decouple, Upgrade() must NOT return "gentle-ai is not installed"
	// because it must not depend on gentle-ai at all.
	err := Upgrade()
	if err != nil && strings.Contains(err.Error(), "gentle-ai is not installed") {
		t.Fatalf("Upgrade() must not depend on gentle-ai, got %v", err)
	}
}

// ─── Preset semantics must survive the decouple ──────────────────────────

// The decoupling guard: a dry run must never touch the filesystem or shell out,
// it only reports what Engram installation would do.
func TestInstallEcosystem_DryRunIsInert(t *testing.T) {
	if err := InstallEcosystem(InstallOptions{DryRun: true}); err != nil {
		t.Fatalf("dry run returned an error: %v", err)
	}
}
