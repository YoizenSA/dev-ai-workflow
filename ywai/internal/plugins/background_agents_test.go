package plugins

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// containsString reports whether want is present in the []any slice.
func containsString(slice []any, want string) bool {
	for _, v := range slice {
		if s, ok := v.(string); ok && s == want {
			return true
		}
	}
	return false
}

// pluginArray returns the root v2 "plugins" array as []any (fatal if absent/wrong type).
func pluginArray(t *testing.T, path string) []any {
	t.Helper()
	root := readConfigRoot(t, path)
	if _, ok := root["plugins"]; ok {
		t.Fatalf("config still has legacy \"plugins\" key: %v", root["plugins"])
	}
	arr, ok := root["plugin"].([]any)
	if !ok {
		t.Fatalf("config has no []any \"plugin\" array; got %T", root["plugin"])
	}
	return arr
}

// permissionRules returns the root v2 "permissions" rule array as action →
// effect (fatal if absent/wrong type).
func permissionRules(t *testing.T, path string) map[string]string {
	t.Helper()
	root := readConfigRoot(t, path)
	perms, ok := root["permission"].(map[string]any)
	if !ok {
		t.Fatalf("config has no map \"permission\"; got %T", root["permission"])
	}
	out := map[string]string{}
	for action, raw := range perms {
		if effect, ok := raw.(string); ok {
			out[action] = effect
		}
	}
	return out
}

func TestPatchOpenCodeBackgroundAgents(t *testing.T) {
	const jsPath = "/home/u/.config/opencode/ywai-plugins/background-agents-v2.js"

	// Case A — empty config: creates plugin array + permissions rules.
	t.Run("creates_plugin_and_permissions_when_missing", func(t *testing.T) {
		path := writeAgentConfig(t, "opencode.json", map[string]any{})

		if err := patchOpenCodeBackgroundAgents(path, jsPath); err != nil {
			t.Fatalf("patchOpenCodeBackgroundAgents() error = %v", err)
		}

		if arr := pluginArray(t, path); !containsString(arr, jsPath) {
			t.Errorf("plugin array %v does not contain %q", arr, jsPath)
		}
		rules := permissionRules(t, path)
		if rules["delegate"] != "allow" {
			t.Errorf("permissions[delegate] = %v, want allow", rules["delegate"])
		}
		if rules["delegation_*"] != "allow" {
			t.Errorf("permissions[delegation_*] = %v, want allow", rules["delegation_*"])
		}
	})

	// Case B — preserves existing plugins and v1 permission entries; deletes a leftover v2 permissions array.
	t.Run("preserves_existing_entries", func(t *testing.T) {
		path := writeAgentConfig(t, "opencode.json", map[string]any{
			"plugin": []any{"some-other-plugin"},
			"permissions": []any{map[string]any{
				"action": "shell", "resource": "*", "effect": "ask",
			}},
			"permission": map[string]any{
				"bash": "ask",
			},
		})

		if err := patchOpenCodeBackgroundAgents(path, jsPath); err != nil {
			t.Fatalf("patchOpenCodeBackgroundAgents() error = %v", err)
		}

		arr := pluginArray(t, path)
		if !containsString(arr, "some-other-plugin") {
			t.Errorf("plugin array %v dropped pre-existing entry", arr)
		}
		if !containsString(arr, jsPath) {
			t.Errorf("plugin array %v does not contain %q", arr, jsPath)
		}
		rules := permissionRules(t, path)
		if rules["bash"] != "ask" {
			t.Errorf("permission[bash] = %v, want ask (clobbered)", rules["bash"])
		}
		if rules["delegate"] != "allow" {
			t.Errorf("permission[delegate] = %v, want allow", rules["delegate"])
		}
		root := readConfigRoot(t, path)
		if _, ok := root["permissions"]; ok {
			t.Errorf("v2 \"permissions\" array must be deleted, got %v", root["permissions"])
		}
	})

	// Case C — idempotent: running twice does not duplicate the plugin entry.
	t.Run("idempotent_no_duplicate", func(t *testing.T) {
		path := writeAgentConfig(t, "opencode.json", map[string]any{})

		for i := 0; i < 2; i++ {
			if err := patchOpenCodeBackgroundAgents(path, jsPath); err != nil {
				t.Fatalf("patchOpenCodeBackgroundAgents() call %d error = %v", i, err)
			}
		}

		arr := pluginArray(t, path)
		count := 0
		for _, v := range arr {
			if s, _ := v.(string); s == jsPath {
				count++
			}
		}
		if count != 1 {
			t.Errorf("plugin array %v contains %q %d times, want exactly 1", arr, jsPath, count)
		}
	})
}

// TestInstallBackgroundAgentsWithBundle covers the copy + patch glue: the
// bundle JS is written next to the config under ywai-plugins/, and the config
// references it.
func TestInstallBackgroundAgentsWithBundle(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	writeJSON(t, configPath, map[string]any{})

	// Fake bundle source.
	bundleSrc := filepath.Join(dir, "src-bundle.js")
	if err := os.WriteFile(bundleSrc, []byte("// fake plugin bundle\n"), 0o644); err != nil {
		t.Fatalf("write fake bundle: %v", err)
	}

	if err := installBackgroundAgentsWithBundle(configPath, bundleSrc); err != nil {
		t.Fatalf("installBackgroundAgentsWithBundle() error = %v", err)
	}

	destJS := filepath.Join(dir, "ywai-plugins", "background-agents-v2.js")
	got, err := os.ReadFile(destJS)
	if err != nil {
		t.Fatalf("expected bundle copied to %s: %v", destJS, err)
	}
	if want := "// fake plugin bundle\n"; string(got) != want {
		t.Errorf("copied bundle = %q, want %q", string(got), want)
	}

	arr := pluginArray(t, configPath)
	if !containsString(arr, destJS) {
		t.Errorf("plugin array %v does not reference %q", arr, destJS)
	}

	// Idempotent end-to-end.
	if err := installBackgroundAgentsWithBundle(configPath, bundleSrc); err != nil {
		t.Fatalf("second installBackgroundAgentsWithBundle() error = %v", err)
	}
	arr2 := pluginArray(t, configPath)
	if !reflect.DeepEqual(arr, arr2) {
		t.Errorf("plugin array changed on second run: %v -> %v", arr, arr2)
	}
}

// TestInstallBackgroundAgents_Integration exercises the full public installer
// against the real resolved bundle (source checkout dist/). It is skipped when
// no bundle has been built (e.g. CI without bun), so it never fails spuriously.
func TestInstallBackgroundAgents_Integration(t *testing.T) {
	if _, err := config.BackgroundAgentsBundlePath(); err != nil {
		t.Skipf("no background-agents bundle built: %v", err)
	}

	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	writeJSON(t, configPath, map[string]any{})

	if err := InstallBackgroundAgents(configPath); err != nil {
		t.Fatalf("InstallBackgroundAgents() error = %v", err)
	}

	destJS := filepath.Join(dir, "ywai-plugins", config.BackgroundAgentsBundleName)
	info, err := os.Stat(destJS)
	if err != nil {
		t.Fatalf("expected bundle at %s: %v", destJS, err)
	}
	// The real bundle is ~0.5 MB; guard against an empty/truncated copy.
	if info.Size() < 1024 {
		t.Errorf("copied bundle size = %d bytes, want a real bundle (>1KB)", info.Size())
	}

	if arr := pluginArray(t, configPath); !containsString(arr, destJS) {
		t.Errorf("plugin array %v does not reference %q", arr, destJS)
	}
	if rules := permissionRules(t, configPath); rules["delegate"] != "allow" {
		t.Errorf("permissions[delegate] = %v, want allow", rules["delegate"])
	}
}
