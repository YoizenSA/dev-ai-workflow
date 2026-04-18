package main

import "testing"

func TestIsInstallWarningLine(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"Cloning GA repository...", false},
		{"✓ Installed GA to /usr/local/bin/ga", false},
		{"DRY RUN: Would update .gitignore", false},
		{"no errors during install", false},
		{"completed without errors", false},

		{"warning: Failed to install lefthook hooks", true},
		{"Error: could not download GA binary", true},
		{"FATAL: permission denied writing to /etc", true},
		{"Missing skill directories in source", true},
		{"unable to resolve version, falling back to main", true},
		{"installation failed for skill biome", true},
	}

	for _, c := range cases {
		t.Run(c.line, func(t *testing.T) {
			if got := isInstallWarningLine(c.line); got != c.want {
				t.Errorf("isInstallWarningLine(%q) = %v, want %v", c.line, got, c.want)
			}
		})
	}
}

// TestBuildProjectInstallFlags_AllRecommended verifies the happy-path mode
// applies sane defaults regardless of componentValues state.
func TestBuildProjectInstallFlags_AllRecommended(t *testing.T) {
	m := newSetupModel(".", nil)
	m.installModeIdx = 0 // All recommended

	got := m.buildProjectInstallFlags()

	if !got.InstallGA {
		t.Errorf("All recommended must enable InstallGA")
	}
	if !got.InstallSDD {
		t.Errorf("All recommended must enable InstallSDD")
	}
	if !got.InstallExt {
		t.Errorf("All recommended must enable InstallExt (umbrella)")
	}
	if got.SkipHooks {
		t.Errorf("All recommended must not skip hooks")
	}
	if !got.SkipBiome {
		t.Errorf("Biome is opt-in and must be skipped under All recommended")
	}
	if got.DryRun {
		t.Errorf("All recommended must not enable DryRun")
	}
}

// TestBuildProjectInstallFlags_CustomMode exercises the 12-item index
// mapping in Custom mode and prevents index-drift regressions like the
// previous DryRun-at-index-5 bug.
func TestBuildProjectInstallFlags_CustomMode(t *testing.T) {
	m := newSetupModel(".", nil)
	m.installModeIdx = 1 // Custom

	// Defaults of the 12-item checklist: Docs, Skills, Commands, MCPs, GA,
	// Engram, Global, Hooks = ON; Biome, Metronous, SDD Engram Plugin, DryRun = OFF.
	got := m.buildProjectInstallFlags()

	if got.SkipDocs {
		t.Errorf("SkipDocs must be false when componentValues[0]=true")
	}
	if got.SkipSkills {
		t.Errorf("SkipSkills must be false when componentValues[1]=true")
	}
	if got.SkipCommands {
		t.Errorf("SkipCommands must be false when componentValues[2]=true")
	}
	if got.SkipMCPs {
		t.Errorf("SkipMCPs must be false when componentValues[3]=true")
	}
	if got.SkipGA || !got.InstallGA {
		t.Errorf("GA must be enabled when componentValues[4]=true")
	}
	if got.SkipEngram {
		t.Errorf("SkipEngram must be false when componentValues[5]=true")
	}
	if !got.InstallGlobal {
		t.Errorf("InstallGlobal must be true when componentValues[6]=true")
	}
	if got.SkipHooks {
		t.Errorf("SkipHooks must be false when componentValues[7]=true")
	}
	if !got.SkipBiome {
		t.Errorf("SkipBiome must be true when componentValues[8]=false (default)")
	}
	if got.InstallMetronous {
		t.Errorf("InstallMetronous must be false when componentValues[9]=false (default)")
	}
	if got.DryRun {
		t.Errorf("DryRun must be false when componentValues[11]=false (default)")
	}

	// Flip Biome ON, Metronous ON, DryRun ON, Docs OFF and verify propagation.
	m.componentValues[0] = false  // Docs off
	m.componentValues[8] = true   // Biome on
	m.componentValues[9] = true   // Metronous on
	m.componentValues[11] = true  // DryRun on
	got = m.buildProjectInstallFlags()
	if !got.SkipDocs {
		t.Errorf("SkipDocs must follow componentValues[0]=false")
	}
	if got.SkipBiome {
		t.Errorf("SkipBiome must follow componentValues[8]=true (enabled)")
	}
	if !got.InstallMetronous {
		t.Errorf("InstallMetronous must follow componentValues[9]=true")
	}
	if !got.DryRun {
		t.Errorf("DryRun must follow componentValues[11]=true")
	}
}
