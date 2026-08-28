package selfupdate

import "testing"

// TestIsPrerelease_PicksTheChannelTheBinaryCameFrom guards the regression where
// `ywai serve` called Run (stable) unconditionally. Because serve restarts on
// every `ywai update`, a beta install was silently pulled back to the latest
// stable on the next server start — `ywai update --beta` reported
// "8.22.3 -> v8.24.0-beta.3" while the binary on disk went back to 8.22.3.
// Only one install is live at a time, so the channel must follow the running
// binary: a beta follows betas, a stable follows stables.
func TestIsPrerelease_PicksTheChannelTheBinaryCameFrom(t *testing.T) {
	beta := []string{
		"v8.24.0-beta.3", "8.24.0-beta.3", "v8.24.0-beta.1",
		"v9.0.0-rc.1", "v9.0.0-alpha.2", "v9.0.0-pre.1",
		"V8.24.0-BETA.3",
	}
	stable := []string{
		"v8.22.3", "8.22.3", "v9.0.0", "v10.2.1",
	}
	for _, v := range beta {
		if !IsPrerelease(v) {
			t.Errorf("IsPrerelease(%q) = false, want true — a beta must stay on the beta channel", v)
		}
	}
	for _, v := range stable {
		if IsPrerelease(v) {
			t.Errorf("IsPrerelease(%q) = true, want false — a stable must not jump to betas", v)
		}
	}
}

// A build metadata suffix is not a prerelease: v1.2.3+build.5 is still stable.
func TestIsPrerelease_BuildMetadataIsNotAPrerelease(t *testing.T) {
	if IsPrerelease("v8.22.3+build.5") {
		t.Error("build metadata must not be read as a prerelease")
	}
}
