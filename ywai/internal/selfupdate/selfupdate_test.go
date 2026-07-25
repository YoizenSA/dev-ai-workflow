package selfupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
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

// The updater downloads a binary and runs it. These pin the checks that stand
// between "bytes off the network" and "executable on your PATH".

func TestChecksumFor_ParsesGoreleaserManifest(t *testing.T) {
	manifest := "" +
		"aaa111  ywai_1.2.3_linux_amd64.tar.gz\n" +
		"bbb222  ywai_1.2.3_darwin_arm64.tar.gz\n" +
		"ccc333 *ywai_1.2.3_windows_amd64.zip\n"

	for _, tc := range []struct{ asset, want string }{
		{"ywai_1.2.3_linux_amd64.tar.gz", "aaa111"},
		{"ywai_1.2.3_darwin_arm64.tar.gz", "bbb222"},
		{"ywai_1.2.3_windows_amd64.zip", "ccc333"}, // '*' prefix form
	} {
		got, ok := checksumFor(manifest, tc.asset)
		if !ok || got != tc.want {
			t.Errorf("checksumFor(%q) = %q,%v want %q", tc.asset, got, ok, tc.want)
		}
	}

	// An asset the manifest does not cover must not silently pass.
	if _, ok := checksumFor(manifest, "ywai_9.9.9_linux_amd64.tar.gz"); ok {
		t.Error("an absent asset must report not-found, not an empty hash")
	}
}

func TestVerifyAgainstManifest(t *testing.T) {
	const asset = "ywai_1.0.0_linux_amd64.tar.gz"
	dir := t.TempDir()
	archive := filepath.Join(dir, asset)
	payload := []byte("the real release bytes")
	if err := os.WriteFile(archive, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)
	good := hex.EncodeToString(sum[:])

	t.Run("accepts the published hash", func(t *testing.T) {
		if err := verifyAgainstManifest(archive, asset, good+"  "+asset+"\n"); err != nil {
			t.Errorf("a matching archive must verify, got %v", err)
		}
	})

	t.Run("rejects tampered bytes", func(t *testing.T) {
		other := sha256.Sum256([]byte("something else entirely"))
		err := verifyAgainstManifest(archive, asset, hex.EncodeToString(other[:])+"  "+asset+"\n")
		if err == nil {
			t.Fatal("a mismatched archive must NOT install")
		}
		if !strings.Contains(err.Error(), "checksum mismatch") {
			t.Errorf("unhelpful error: %v", err)
		}
	})

	t.Run("rejects an asset the manifest does not cover", func(t *testing.T) {
		if err := verifyAgainstManifest(archive, asset, good+"  some_other_file.tar.gz\n"); err == nil {
			t.Fatal("a missing manifest entry must fail closed, not pass")
		}
	})

	t.Run("rejects an empty manifest", func(t *testing.T) {
		if err := verifyAgainstManifest(archive, asset, ""); err == nil {
			t.Fatal("an empty manifest must fail closed")
		}
	})
}

func TestIsDevBuild(t *testing.T) {
	// Every shape a local build can carry. The one that got away —
	// "8.16.5-dev+ff82a7b" from scripts/dev.sh — let `ywai serve` replace a
	// freshly compiled binary with the published release, silently.
	for _, v := range []string{
		"dev", "", "  ",
		"1.2.3-next", "v1.2.3-next",
		"8.16.5-dev+ff82a7b", "v8.16.5-dev+ff82a7b",
		"8.16.5-dev", "1.2.3+abc1234", "1.2.3-dirty", "1.2.3-local", "1.2.3-snapshot",
	} {
		if !IsDevBuild(v) {
			t.Errorf("IsDevBuild(%q) = false, want true — auto-update would overwrite a local build", v)
		}
	}
	for _, v := range []string{"1.2.3", "v1.2.3", "1.2.3-beta.1", "v0.9.0-rc.2"} {
		if IsDevBuild(v) {
			t.Errorf("IsDevBuild(%q) = true, want false — real releases must still auto-update", v)
		}
	}
}
