package config

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// The resolvers prefer the source checkout (plugins/subagent-statusline/dist/)
// over the seeded data dir. SetRepoRoot points PluginsSourceDir at a fake
// checkout so the test never depends on the real repo tree or a real home.
func TestSubagentStatuslineBundlePathsResolveFromSource(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo := t.TempDir()
	SetRepoRoot(repo)
	t.Cleanup(func() { SetRepoRoot("") })

	dist := filepath.Join(repo, "plugins", "subagent-statusline", "dist")
	if err := os.MkdirAll(dist, 0o755); err != nil {
		t.Fatal(err)
	}
	serverSrc := filepath.Join(dist, SubagentStatuslineServerBundleName)
	tuiSrc := filepath.Join(dist, SubagentStatuslineTuiSrcBundleName)
	if err := os.WriteFile(serverSrc, []byte("server"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tuiSrc, []byte("tui"), 0o644); err != nil {
		t.Fatal(err)
	}

	gotServer, err := SubagentStatuslineServerBundlePath()
	if err != nil {
		t.Fatalf("server resolver: %v", err)
	}
	if gotServer != serverSrc {
		t.Errorf("server = %q, want source %q", gotServer, serverSrc)
	}

	gotTui, err := SubagentStatuslineTuiBundlePath()
	if err != nil {
		t.Fatalf("tui resolver: %v", err)
	}
	if gotTui != tuiSrc {
		t.Errorf("tui = %q, want source %q", gotTui, tuiSrc)
	}
}

// Without a source checkout the resolvers seed from the embedded FS: the
// server bundle is flat under plugins/, the TUI bundle lands under plugins/tui/
// renamed to the .tsx name the TUI host expects.
func TestSubagentStatuslineBundlePathsSeedFromEmbedded(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// Keep the real repo's plugins dir out of the source step: with cwd inside
	// the checkout, RepoRoot() would resolve to it.
	SetRepoRoot(t.TempDir())
	t.Cleanup(func() { SetRepoRoot("") })

	prev := getEmbeddedPluginsFS
	t.Cleanup(func() { getEmbeddedPluginsFS = prev })
	getEmbeddedPluginsFS = func() fs.FS {
		return fstest.MapFS{
			"subagent-statusline-server.js":   {Data: []byte("server-bundle")},
			"tui/subagent-statusline-tui.tsx": {Data: []byte("tui-bundle")},
		}
	}

	gotServer, err := SubagentStatuslineServerBundlePath()
	if err != nil {
		t.Fatalf("server resolver: %v", err)
	}
	wantServer := filepath.Join(DataPluginsDir(), SubagentStatuslineServerBundleName)
	if gotServer != wantServer {
		t.Errorf("server = %q, want seeded %q", gotServer, wantServer)
	}
	data, err := os.ReadFile(gotServer)
	if err != nil || string(data) != "server-bundle" {
		t.Errorf("seeded server content = %q, err = %v", data, err)
	}

	gotTui, err := SubagentStatuslineTuiBundlePath()
	if err != nil {
		t.Fatalf("tui resolver: %v", err)
	}
	wantTui := filepath.Join(DataPluginsDir(), "tui", SubagentStatuslineTuiBundleName)
	if gotTui != wantTui {
		t.Errorf("tui = %q, want seeded %q", gotTui, wantTui)
	}
	data, err = os.ReadFile(gotTui)
	if err != nil || string(data) != "tui-bundle" {
		t.Errorf("seeded tui content = %q, err = %v", data, err)
	}
}

// Without a source checkout the seeded copies under DataPluginsDir are the
// second tier: when both are present the resolvers return them as-is and never
// touch the embedded FS.
func TestSubagentStatuslineBundlePathsSeedFromDataDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	SetRepoRoot(t.TempDir())
	t.Cleanup(func() { SetRepoRoot("") })

	seededServer := filepath.Join(DataPluginsDir(), SubagentStatuslineServerBundleName)
	seededTui := filepath.Join(DataPluginsDir(), "tui", SubagentStatuslineTuiBundleName)
	if err := os.MkdirAll(filepath.Dir(seededTui), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(seededServer, []byte("seeded-server"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(seededTui, []byte("seeded-tui"), 0o644); err != nil {
		t.Fatal(err)
	}

	gotServer, err := SubagentStatuslineServerBundlePath()
	if err != nil {
		t.Fatalf("server resolver: %v", err)
	}
	if gotServer != seededServer {
		t.Errorf("server = %q, want seeded %q", gotServer, seededServer)
	}

	gotTui, err := SubagentStatuslineTuiBundlePath()
	if err != nil {
		t.Fatalf("tui resolver: %v", err)
	}
	if gotTui != seededTui {
		t.Errorf("tui = %q, want seeded %q", gotTui, seededTui)
	}
}

// With no source checkout, no seeded copy, and no embedded FS the resolvers
// must fail with an error that names the missing plugin and the rebuild
// command, so the user knows what to run.
func TestSubagentStatuslineBundlePathsMissingAreActionable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	SetRepoRoot(t.TempDir())
	t.Cleanup(func() { SetRepoRoot("") })

	prev := getEmbeddedPluginsFS
	t.Cleanup(func() { getEmbeddedPluginsFS = prev })
	getEmbeddedPluginsFS = nil

	_, err := SubagentStatuslineServerBundlePath()
	if err == nil {
		t.Fatal("server resolver: want error when no bundle exists")
	}
	assertActionableBundleError(t, err.Error(), "sub-agent statusline server bundle")

	_, err = SubagentStatuslineTuiBundlePath()
	if err == nil {
		t.Fatal("tui resolver: want error when no bundle exists")
	}
	assertActionableBundleError(t, err.Error(), "sub-agent statusline TUI bundle")
}

// assertActionableBundleError pins the two parts a user needs from a missing
// bundle: which plugin is missing and the command that rebuilds it.
func assertActionableBundleError(t *testing.T, msg, plugin string) {
	t.Helper()
	if !strings.Contains(msg, plugin) {
		t.Errorf("error %q must name the plugin %q", msg, plugin)
	}
	if !strings.Contains(msg, "prepare-embedded.sh") {
		t.Errorf("error %q must name the rebuild command", msg)
	}
}
