package plugins

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// setPluginCacheRoots points pluginCacheRoots at dirs and restores it after.
func setPluginCacheRoots(t *testing.T, dirs ...string) {
	t.Helper()
	orig := pluginCacheRoots
	pluginCacheRoots = func() []string { return dirs }
	t.Cleanup(func() { pluginCacheRoots = orig })
}

// mkCacheEntry creates a plugin cache dir root/name (and parents).
func mkCacheEntry(t *testing.T, root, name string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	return dir
}

func TestClearPluginCache_RemovesManagedEntries(t *testing.T) {
	root := t.TempDir()
	setPluginCacheRoots(t, root)

	mkCacheEntry(t, root, "opencode-subagent-statusline@latest")
	mkCacheEntry(t, root, "@dietrichgebert/ponytail@latest")
	mkCacheEntry(t, root, "@dietrichgebert/ponytail")
	mkCacheEntry(t, root, "@cioffinahuel/opencode-ado@0.8.2")
	mkCacheEntry(t, root, "@nahuelcio/opencode-ado@latest")
	mkCacheEntry(t, root, "@slkiser/opencode-quota@latest")
	// Unrelated user plugins must survive.
	mkCacheEntry(t, root, "@user/other-plugin@latest")
	mkCacheEntry(t, root, "user-plugin@1.0.0")

	cleared, err := ClearPluginCache(false)
	if err != nil {
		t.Fatalf("ClearPluginCache() error = %v", err)
	}

	want := []string{
		"opencode-subagent-statusline@latest",
		"@dietrichgebert/ponytail@latest",
		"@dietrichgebert/ponytail",
		"@cioffinahuel/opencode-ado@0.8.2",
		"@nahuelcio/opencode-ado@latest",
		"@slkiser/opencode-quota@latest",
	}
	sort.Strings(cleared)
	sort.Strings(want)
	if !reflect.DeepEqual(cleared, want) {
		t.Errorf("cleared = %v, want %v", cleared, want)
	}

	for _, name := range want {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Errorf("%s still exists (stat err = %v), want removed", name, err)
		}
	}
	for _, name := range []string{"@user/other-plugin@latest", "user-plugin@1.0.0"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Errorf("%s removed, want kept: %v", name, err)
		}
	}
}

func TestClearPluginCache_DryRunRemovesNothing(t *testing.T) {
	root := t.TempDir()
	setPluginCacheRoots(t, root)

	statusline := mkCacheEntry(t, root, "opencode-subagent-statusline@latest")

	cleared, err := ClearPluginCache(true)
	if err != nil {
		t.Fatalf("ClearPluginCache(dryRun) error = %v", err)
	}
	if len(cleared) != 1 || cleared[0] != "opencode-subagent-statusline@latest" {
		t.Fatalf("cleared = %v, want [opencode-subagent-statusline@latest]", cleared)
	}
	if _, err := os.Stat(statusline); err != nil {
		t.Errorf("dry run removed %s: %v", statusline, err)
	}
}

func TestClearPluginCache_MissingRootIsNoop(t *testing.T) {
	setPluginCacheRoots(t, t.TempDir()) // empty root, nothing created

	cleared, err := ClearPluginCache(false)
	if err != nil {
		t.Fatalf("ClearPluginCache() error = %v", err)
	}
	if len(cleared) != 0 {
		t.Errorf("cleared = %v, want none", cleared)
	}
}

func TestClearPluginCache_MultipleRootsDedupes(t *testing.T) {
	rootA, rootB := t.TempDir(), t.TempDir()
	setPluginCacheRoots(t, rootA, rootB)

	mkCacheEntry(t, rootA, "opencode-subagent-statusline@latest")
	mkCacheEntry(t, rootB, "opencode-subagent-statusline@latest")
	mkCacheEntry(t, rootB, "@dietrichgebert/ponytail@latest")

	cleared, err := ClearPluginCache(false)
	if err != nil {
		t.Fatalf("ClearPluginCache() error = %v", err)
	}

	want := []string{"opencode-subagent-statusline@latest", "@dietrichgebert/ponytail@latest"}
	if !reflect.DeepEqual(cleared, want) {
		t.Errorf("cleared = %v, want %v", cleared, want)
	}
}
