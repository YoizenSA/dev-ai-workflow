package main

import "testing"

// TestBuildProjectInstallFlags_ComponentIndicesStayInSync guards against a
// previous regression where buildProjectInstallFlags read DryRun from
// componentValues[5] (the Hooks entry) causing the installer to silently
// skip all repo writes.
func TestBuildProjectInstallFlags_ComponentIndicesStayInSync(t *testing.T) {
	m := newSetupModel(".", nil)

	// Default values mirror a brand-new user hitting Enter through the
	// wizard: GA, SDD, VSCode, Ext, (not global), hooks, (not dry run).
	got := m.buildProjectInstallFlags()

	if got.DryRun {
		t.Errorf("DryRun must default to false when componentValues[6]=false; index drift detected")
	}
	if got.SkipHooks {
		t.Errorf("SkipHooks must default to false when componentValues[5]=true")
	}
	if !got.InstallGA {
		t.Errorf("InstallGA must follow componentValues[0]")
	}
	if !got.InstallSDD {
		t.Errorf("InstallSDD must follow componentValues[1]")
	}
	if !got.InstallVSCode {
		t.Errorf("InstallVSCode must follow componentValues[2]")
	}
	if !got.InstallExt {
		t.Errorf("InstallExt must follow componentValues[3]")
	}

	// Flip dry-run via the correct index and ensure it takes effect.
	m.componentValues[6] = true
	got = m.buildProjectInstallFlags()
	if !got.DryRun {
		t.Errorf("DryRun must become true when componentValues[6]=true")
	}

	// Flip hooks off and ensure SkipHooks is set.
	m.componentValues[5] = false
	m.componentValues[6] = false
	got = m.buildProjectInstallFlags()
	if !got.SkipHooks {
		t.Errorf("SkipHooks must become true when componentValues[5]=false")
	}
	if got.DryRun {
		t.Errorf("DryRun must remain false when only hooks was flipped")
	}
}
