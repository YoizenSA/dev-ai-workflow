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

// TestPlanForPreset_NonMinimalIncludesEngram is a regression guard: the
// "include engram" decision for non-minimal presets must survive the
// decouple. Engram is still selected for non-minimal presets but is
// installed via ywai's manual release path instead of `gentle-ai install`.
func TestPlanForPreset_NonMinimalIncludesEngram(t *testing.T) {
	presets := []string{"full-gentleman", "ecosystem-only", "custom", ""}
	for _, preset := range presets {
		t.Run(preset, func(t *testing.T) {
			p := PlanForPreset(preset)
			if !p.IncludeEngram {
				t.Fatalf("preset %q must keep IncludeEngram=true after decouple", preset)
			}
		})
	}
}

// TestPlanForPreset_MinimalExcludesEngram is a regression guard: the minimal
// preset must continue to skip engram.
func TestPlanForPreset_MinimalExcludesEngram(t *testing.T) {
	p := PlanForPreset("minimal")
	if p.IncludeEngram {
		t.Fatal("minimal preset must keep IncludeEngram=false")
	}
}

// TestSkillRegistryRefresh_DoesNotExecGentleAI asserts the slice 1 contract:
// SkillRegistryRefresh must NOT shell out to a `gentle-ai` binary even when one
// is reachable on PATH. It installs a fake `gentle-ai` executable first on
// PATH that increments a counter file on every invocation. If the production
// code still execs gentle-ai, the counter appears and the test fails.
//
// Any return value from SkillRegistryRefresh is acceptable; the hard contract
// is "did the fake binary get executed?". Today this test is RED because
// SkillRegistryRefresh still execs gentle-ai.
func TestSkillRegistryRefresh_DoesNotExecGentleAI(t *testing.T) {
	dir := t.TempDir()
	counter := filepath.Join(dir, "invoked")
	fakeBin := filepath.Join(dir, "gentle-ai")
	script := "#!/bin/sh\ntouch " + counter + "\n"
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	if _, err := exec.LookPath("gentle-ai"); err != nil {
		t.Fatalf("test setup: fake gentle-ai must resolve on PATH, got %v", err)
	}

	if err := SkillRegistryRefresh(""); err != nil {
		t.Logf("SkillRegistryRefresh returned %v (any outcome is fine; counter is the real assertion)", err)
	}

	if _, err := os.Stat(counter); err == nil {
		t.Fatalf("SkillRegistryRefresh must not exec gentle-ai; fake binary was invoked (counter at %s)", counter)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat counter: %v", err)
	}
}

// TestCurrentVersion_DoesNotExecGentleAI pins the slice 1 contract:
// CurrentVersion() must be ywai-owned/neutral. Full decoupling forbids
// spawning the gentle-ai binary to report a version; CurrentVersion must
// return "" (or any ywai-native value) without resolving or invoking
// `gentle-ai` from disk.
//
// Reuses the same fake-executable + counter helper shape as
// TestSkillRegistryRefresh_DoesNotExecGentleAI. The fake is named
// `gentle-ai` and placed FIRST on PATH so any process started by
// CurrentVersion via exec.Command / exec.LookPath would find and exec
// it (and the counter file would appear).
func TestCurrentVersion_DoesNotExecGentleAI(t *testing.T) {
	dir := t.TempDir()
	counter := filepath.Join(dir, "invoked")

	script := "#!/bin/sh\ntouch " + counter + "\n"
	exe := filepath.Join(dir, "gentle-ai")
	if err := os.WriteFile(exe, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	if _, err := exec.LookPath("gentle-ai"); err != nil {
		t.Fatalf("test setup: fake gentle-ai must resolve on PATH, got %v", err)
	}

	// Any return value is fine; the counter is the real assertion.
	// CurrentVersion must not exec the fake binary.
	_ = CurrentVersion()

	if _, err := os.Stat(counter); err == nil {
		t.Fatalf("CurrentVersion must not exec gentle-ai; fake binary was invoked (counter at %s)", counter)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat counter: %v", err)
	}
}
