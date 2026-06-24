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
	}
	for _, c := range cases {
		if got := computeUpdate(c.installed, c.latest); got != c.want {
			t.Errorf("computeUpdate(%q, %q) = %v, want %v", c.installed, c.latest, got, c.want)
		}
	}
}

func TestRefreshThrottlesNetworkCheck(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // DataDir() resolves under HOME

	calls := 0
	orig := latestFn
	latestFn = func() (string, error) {
		calls++
		return "v2.0.0", nil
	}
	defer func() { latestFn = orig }()

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
}

func TestTouchDoesNotCallNetwork(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	orig := latestFn
	latestFn = func() (string, error) {
		t.Fatal("Touch must not call the network")
		return "", nil
	}
	defer func() { latestFn = orig }()

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
