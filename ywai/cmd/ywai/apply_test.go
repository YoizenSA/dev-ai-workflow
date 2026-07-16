package main

import "testing"

func TestPlanManaged_YwaiSkillsAlwaysRegardlessOfPreset(t *testing.T) {
	t.Parallel()

	for _, preset := range []string{"full-gentleman", "ecosystem-only", "minimal", ""} {
		for _, mode := range []applyMode{applyInstall, applyUpdate} {
			p := planManaged(mode, preset)
			if !p.CopyExtraSkills {
				t.Fatalf("mode=%v preset=%q must always copy ywai skills: %+v", mode, preset, p)
			}
			if !p.InstallProfiles || !p.WriteAgentsMd || !p.InstallPlugins {
				t.Fatalf("mode=%v preset=%q must keep ywai managed work: %+v", mode, preset, p)
			}
		}
	}
}

func TestCountApplySteps_Stable(t *testing.T) {
	t.Parallel()

	plan := planManaged(applyInstall, "full-gentleman")
	o := applyOpts{Autostart: true}
	n := countApplySteps(plan, o)
	if n < 10 {
		t.Fatalf("full install with autostart should have many steps, got %d", n)
	}

	// Skipping binary + no autostart/restart reduces count.
	o2 := applyOpts{SkipGentleAIBinary: true}
	n2 := countApplySteps(plan, o2)
	if n2 >= n {
		t.Fatalf("skipping binary/autostart should reduce steps: n=%d n2=%d", n, n2)
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
