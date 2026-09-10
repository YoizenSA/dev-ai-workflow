package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
)

func TestPlanManaged_AlwaysDoesTheYwaiManagedWork(t *testing.T) {
	t.Parallel()

	for _, mode := range []applyMode{applyInstall, applyUpdate} {
		p := planManaged(mode)
		if !p.CopyExtraSkills {
			t.Fatalf("mode=%v must always copy ywai skills: %+v", mode, p)
		}
		if !p.InstallProfiles || !p.WriteAgentsMd || !p.InstallPlugins {
			t.Fatalf("mode=%v must keep ywai managed work: %+v", mode, p)
		}
		if !p.SetDefaultAgent || !p.SetDefaultModel {
			t.Fatalf("mode=%v must set default_agent and default model: %+v", mode, p)
		}
	}
}

func TestCountApplySteps_Stable(t *testing.T) {
	t.Parallel()

	plan := planManaged(applyInstall)
	o := applyOpts{Autostart: true}
	n := countApplySteps(plan, o)
	if n < 10 {
		t.Fatalf("full install with autostart should have many steps, got %d", n)
	}

	// No autostart/restart reduces count. The gentle-ai binary step is gone.
	o2 := applyOpts{}
	n2 := countApplySteps(plan, o2)
	if n2 >= n {
		t.Fatalf("no autostart should reduce steps: n=%d n2=%d", n, n2)
	}
}

func TestApplyResult_ExitCode(t *testing.T) {
	t.Parallel()

	var ok applyResult
	if ok.exitCode() != 0 {
		t.Fatal("empty result should be exit 0")
	}

	warn := applyResult{Warnings: []string{"x"}}
	if warn.exitCode() != 1 {
		t.Fatal("warnings should be exit 1")
	}

	fatal := applyResult{Fatal: errSentinel{}}
	if fatal.exitCode() != 1 {
		t.Fatal("fatal should be exit 1")
	}
}

type errSentinel struct{}

func (errSentinel) Error() string { return "fatal" }

// The v2 permissions array is native to OpenCode v2, so the v1-only
// permissions sweep must be a no-op on a v2 host and target both the isolate
// and the canonical user config on v1.
func TestV1PermissionSweepPaths(t *testing.T) {
	t.Run("v2 host is a no-op", func(t *testing.T) {
		home := t.TempDir()
		pinOCEnv(t, home)
		t.Setenv(agent.OpenCodeOverrideEnv, "opencode2")
		if paths := v1PermissionSweepPaths(); len(paths) != 0 {
			t.Fatalf("v2 host must skip the v1 sweep, got %v", paths)
		}
	})

	t.Run("v1 host hits the home config", func(t *testing.T) {
		home := t.TempDir()
		pinOCEnv(t, home)
		t.Setenv(agent.OpenCodeOverrideEnv, "opencode")
		paths := v1PermissionSweepPaths()
		if len(paths) != 1 || paths[0] != filepath.Join(home, ".config", "opencode", "opencode.json") {
			t.Fatalf("v1 sweep paths = %v, want the HOME config only", paths)
		}
	})

	t.Run("v1 host sweeps the isolate and the canonical config", func(t *testing.T) {
		home := t.TempDir()
		pinOCEnv(t, home)
		t.Setenv(agent.OpenCodeOverrideEnv, "opencode")
		isolate := filepath.Join(t.TempDir(), "isolate")
		t.Setenv("OPENCODE_CONFIG_DIR", isolate)
		paths := v1PermissionSweepPaths()
		want := []string{
			filepath.Join(isolate, "opencode.json"),
			filepath.Join(home, ".config", "opencode", "opencode.json"),
		}
		if len(paths) != 2 || paths[0] != want[0] || paths[1] != want[1] {
			t.Fatalf("v1 sweep paths = %v, want %v", paths, want)
		}
	})

	t.Run("jsonc config is found", func(t *testing.T) {
		home := t.TempDir()
		pinOCEnv(t, home)
		t.Setenv(agent.OpenCodeOverrideEnv, "opencode")
		configDir := filepath.Join(home, ".config", "opencode")
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			t.Fatal(err)
		}
		jsoncPath := filepath.Join(configDir, "opencode.jsonc")
		if err := os.WriteFile(jsoncPath, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		paths := v1PermissionSweepPaths()
		if len(paths) != 1 || paths[0] != jsoncPath {
			t.Fatalf("v1 sweep paths = %v, want the .jsonc file %v", paths, jsoncPath)
		}
	})
}
