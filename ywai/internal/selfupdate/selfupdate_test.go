package selfupdate

import (
	"runtime"
	"strings"
	"testing"
)

func TestReleaseRepositoryIsCanonicalOrg(t *testing.T) {
	if owner != "YoizenSA" {
		t.Fatalf("self-update owner = %q, want canonical release owner YoizenSA", owner)
	}
	if repo != "dev-ai-workflow" {
		t.Fatalf("self-update repo = %q, want dev-ai-workflow", repo)
	}
}

func TestAssetNameStripsVersionPrefix(t *testing.T) {
	name := assetName("v7.0.19")

	if strings.Contains(name, "_v7.0.19_") {
		t.Fatalf("asset name should strip leading v prefix, got %q", name)
	}
	if !strings.Contains(name, "_7.0.19_"+runtime.GOOS+"_"+runtime.GOARCH+".") {
		t.Fatalf("asset name %q does not include normalized version/os/arch", name)
	}
}

func TestPickLatestPrerelease(t *testing.T) {
	// Newest first (GitHub order)
	releases := []releaseInfo{
		{TagName: "v8.10.0", Prerelease: false},
		{TagName: "v8.10.0-beta.2", Prerelease: true},
		{TagName: "v8.10.0-beta.1", Prerelease: true},
	}
	tag, ok := pickLatestPrerelease(releases)
	if !ok || tag != "v8.10.0-beta.2" {
		t.Fatalf("got %q ok=%v, want v8.10.0-beta.2", tag, ok)
	}

	// Tag-name fallback when prerelease flag missing
	releases = []releaseInfo{
		{TagName: "v9.0.0", Prerelease: false},
		{TagName: "v9.0.0-rc.1", Prerelease: false},
	}
	tag, ok = pickLatestPrerelease(releases)
	if !ok || tag != "v9.0.0-rc.1" {
		t.Fatalf("tag fallback: got %q ok=%v, want v9.0.0-rc.1", tag, ok)
	}

	// No prerelease
	releases = []releaseInfo{
		{TagName: "v1.0.0", Prerelease: false},
	}
	if _, ok := pickLatestPrerelease(releases); ok {
		t.Fatal("expected no prerelease")
	}
}

func TestIsPrereleaseTag(t *testing.T) {
	cases := map[string]bool{
		"v8.10.0-beta.1": true,
		"v8.10.0-rc.1":   true,
		"v8.10.0-alpha":  true,
		"v8.10.0":        false,
		"8.10.0":         false,
	}
	for tag, want := range cases {
		if got := isPrereleaseTag(tag); got != want {
			t.Errorf("isPrereleaseTag(%q)=%v want %v", tag, got, want)
		}
	}
}
