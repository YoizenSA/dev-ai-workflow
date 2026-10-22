package versionfile

import (
	"testing"
	"time"
)

func TestComputeUpdate(t *testing.T) {
	cases := []struct {
		installed, latest string
		want              bool
	}{
		{"8.6.0", "v8.6.1", true},  // newer available, leading v trimmed
		{"8.6.1", "v8.6.1", false}, // same after normalization
		{"8.6.1", "8.6.1", false},  // same, no leading v
		{"dev", "v9.9.9", false},   // dev builds never flag updates
		{"dev-abc", "v9.9.9", false},
		{"8.6.1", "", false}, // unknown latest
		// Beta on a newer line than GitHub's stable: not an update.
		{"8.26.0-beta.17", "v8.24.8", false},
		{"8.26.0-beta.17", "v8.26.0-beta.18", true},
		{"8.26.0-beta.17", "v8.26.0-beta.17", false},
	}
	for _, c := range cases {
		if got := computeUpdate(c.installed, c.latest); got != c.want {
			t.Errorf("computeUpdate(%q, %q) = %v, want %v", c.installed, c.latest, got, c.want)
		}
	}
}

func isolateDataDir(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // Windows: os.UserHomeDir reads USERPROFILE
}

func TestRefreshThrottlesNetworkCheck(t *testing.T) {
	isolateDataDir(t)

	calls := 0
	orig := latestFn
	origBeta := latestBetaFn
	latestFn = func() (string, error) {
		calls++
		return "v2.0.0", nil
	}
	latestBetaFn = func() (string, error) {
		t.Fatal("stable install must not fetch the beta channel")
		return "", nil
	}
	defer func() { latestFn = orig; latestBetaFn = origBeta }()

	if err := Refresh("1.0.0", time.Hour); err != nil {
		t.Fatalf("first Refresh error = %v", err)
	}
	// Within ttl: must reuse the cached latest, no second network call.
	if err := Refresh("1.0.0", time.Hour); err != nil {
		t.Fatalf("second Refresh error = %v", err)
	}
	if calls != 1 {
		t.Errorf("latestFn called %d times, want 1 (throttled within ttl)", calls)
	}

	got, err := load(Path())
	if err != nil {
		t.Fatalf("load error = %v", err)
	}
	if got.Installed != "1.0.0" || got.Latest != "v2.0.0" || !got.UpdateAvailable {
		t.Errorf("info = %+v, want installed=1.0.0 latest=v2.0.0 updateAvailable=true", got)
	}
	if got.LatestStable != "v2.0.0" || got.Channel != "stable" {
		t.Errorf("channels = stable=%q beta=%q channel=%q", got.LatestStable, got.LatestBeta, got.Channel)
	}
}

func TestRefreshBetaTracksBetaNotOlderStable(t *testing.T) {
	isolateDataDir(t)

	orig := latestFn
	origBeta := latestBetaFn
	latestFn = func() (string, error) { return "v8.24.8", nil }
	latestBetaFn = func() (string, error) { return "v8.26.0-beta.18", nil }
	defer func() { latestFn = orig; latestBetaFn = origBeta }()

	if err := Refresh("8.26.0-beta.17", time.Hour); err != nil {
		t.Fatalf("Refresh error = %v", err)
	}
	got, err := load(Path())
	if err != nil {
		t.Fatalf("load error = %v", err)
	}
	if got.Channel != "beta" || got.UpdateCommand != "ywai update --beta" {
		t.Errorf("channel=%q command=%q", got.Channel, got.UpdateCommand)
	}
	if got.Latest != "v8.26.0-beta.18" || !got.UpdateAvailable {
		t.Errorf("latest=%q available=%v, want newer beta", got.Latest, got.UpdateAvailable)
	}
	if got.StableNewer {
		t.Error("v8.24.8 must not be advertised as newer than 8.26.0-beta.17")
	}

	// Same beta head: no update, still not a stable downgrade.
	latestBetaFn = func() (string, error) { return "v8.26.0-beta.17", nil }
	if err := save(Path(), Info{}); err != nil { // force ttl miss via empty CheckedAt
		t.Fatalf("reset: %v", err)
	}
	if err := Refresh("8.26.0-beta.17", time.Hour); err != nil {
		t.Fatalf("Refresh error = %v", err)
	}
	got, _ = load(Path())
	if got.UpdateAvailable || got.StableNewer {
		t.Errorf("same beta must be up to date, got %+v", got)
	}
}

func TestTouchMigratesLegacyLatestAndDoesNotDowngradeBeta(t *testing.T) {
	isolateDataDir(t)

	orig := latestFn
	origBeta := latestBetaFn
	latestFn = func() (string, error) {
		t.Fatal("Touch must not call the network")
		return "", nil
	}
	latestBetaFn = func() (string, error) {
		t.Fatal("Touch must not call the network")
		return "", nil
	}
	defer func() { latestFn = orig; latestBetaFn = origBeta }()

	// Legacy file: only `latest` (GitHub stable), written before channel fields.
	if err := save(Path(), Info{Installed: "8.26.0-beta.16", Latest: "v8.24.8", CheckedAt: 123}); err != nil {
		t.Fatalf("seed save error = %v", err)
	}
	if err := Touch("8.26.0-beta.17"); err != nil {
		t.Fatalf("Touch error = %v", err)
	}
	got, _ := load(Path())
	if got.LatestStable != "v8.24.8" {
		t.Errorf("latestStable = %q, want migrated v8.24.8", got.LatestStable)
	}
	if got.UpdateAvailable {
		t.Errorf("beta vs older stable must not flag an update, got %+v", got)
	}
	if got.Channel != "beta" || got.UpdateCommand != "ywai update --beta" {
		t.Errorf("channel=%q command=%q", got.Channel, got.UpdateCommand)
	}
	if got.CheckedAt != 123 {
		t.Errorf("checkedAt = %d, want preserved 123", got.CheckedAt)
	}
}

func TestTouchDoesNotCallNetwork(t *testing.T) {
	isolateDataDir(t)

	orig := latestFn
	origBeta := latestBetaFn
	latestFn = func() (string, error) {
		t.Fatal("Touch must not call the network")
		return "", nil
	}
	latestBetaFn = func() (string, error) {
		t.Fatal("Touch must not call the network")
		return "", nil
	}
	defer func() { latestFn = orig; latestBetaFn = origBeta }()

	// Seed a cached latest via load/save by writing through Refresh-less path.
	if err := save(Path(), Info{Installed: "1.0.0", Latest: "v2.0.0", CheckedAt: 123}); err != nil {
		t.Fatalf("seed save error = %v", err)
	}
	if err := Touch("1.5.0"); err != nil {
		t.Fatalf("Touch error = %v", err)
	}

	got, _ := load(Path())
	if got.Installed != "1.5.0" {
		t.Errorf("installed = %q, want 1.5.0", got.Installed)
	}
	if !got.UpdateAvailable {
		t.Errorf("updateAvailable = false, want true (1.5.0 < cached 2.0.0)")
	}
	if got.CheckedAt != 123 {
		t.Errorf("checkedAt = %d, want preserved 123", got.CheckedAt)
	}
}

func TestRefreshBetaBackfillsEvenWhenStableFresh(t *testing.T) {
	isolateDataDir(t)

	betaCalls := 0
	orig := latestFn
	origBeta := latestBetaFn
	latestFn = func() (string, error) {
		t.Fatal("stable check is still fresh; must not refetch")
		return "", nil
	}
	latestBetaFn = func() (string, error) {
		betaCalls++
		return "v8.26.0-beta.18", nil
	}
	defer func() { latestFn = orig; latestBetaFn = origBeta }()

	now := time.Now().Unix()
	if err := save(Path(), Info{Installed: "8.26.0-beta.17", Latest: "v8.24.8", CheckedAt: now}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := Refresh("8.26.0-beta.17", time.Hour); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if betaCalls != 1 {
		t.Errorf("beta fetch calls = %d, want 1", betaCalls)
	}
	got, _ := load(Path())
	if got.LatestBeta != "v8.26.0-beta.18" || !got.UpdateAvailable {
		t.Errorf("got %+v, want newer beta advertised", got)
	}

	if err := Refresh("8.26.0-beta.17", time.Hour); err != nil {
		t.Fatalf("second Refresh: %v", err)
	}
	if betaCalls != 1 {
		t.Errorf("second Refresh must not refetch beta, calls = %d", betaCalls)
	}
}
