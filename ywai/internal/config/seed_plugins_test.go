package config

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

// An upgrade must replace a plugin bundle seeded by an older install; the
// resolvers only seed when the file is missing, so a stale copy would
// otherwise be served forever (the v1 background-agents bundle survived the
// v2 port and opencode v2 refused to load it).
func TestSeedPluginsFromEmbeddedOverwritesStaleBundle(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	prev := getEmbeddedPluginsFS
	t.Cleanup(func() { getEmbeddedPluginsFS = prev })
	getEmbeddedPluginsFS = func() fs.FS {
		return fstest.MapFS{BackgroundAgentsBundleName: {Data: []byte("new bundle")}}
	}

	seeded := filepath.Join(DataPluginsDir(), BackgroundAgentsBundleName)
	if err := os.MkdirAll(DataPluginsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(seeded, []byte("stale bundle"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := SeedPluginsFromEmbedded(); err != nil {
		t.Fatalf("seed: %v", err)
	}

	got, err := os.ReadFile(seeded)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new bundle" {
		t.Fatalf("stale bundle kept: %q", got)
	}
}
