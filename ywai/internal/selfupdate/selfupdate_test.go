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
