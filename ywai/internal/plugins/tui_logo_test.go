package plugins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// writeBundle creates a temp source bundle with the given contents.
func writeBundle(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "src-"+config.TuiLogoBundleName)
	if err := os.WriteFile(src, []byte(contents), 0o644); err != nil {
		t.Fatalf("write source bundle: %v", err)
	}
	return src
}

// tuiConfigPathFor returns the tui.json path that sits next to configPath.
func tuiConfigPathFor(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "tui.json")
}

func TestInstallTuiLogoWithBundle(t *testing.T) {
	t.Run("copies_into_tui_plugins_dir_next_to_config", func(t *testing.T) {
		bundle := writeBundle(t, "// ywai logo source")
		configPath := writeAgentConfig(t, "opencode.json", map[string]any{})

		if err := installTuiLogoWithBundle(configPath, bundle); err != nil {
			t.Fatalf("installTuiLogoWithBundle() error = %v", err)
		}

		dest := filepath.Join(filepath.Dir(configPath), "tui-plugins", config.TuiLogoBundleName)
		got, err := os.ReadFile(dest)
		if err != nil {
			t.Fatalf("read installed logo: %v", err)
		}
		if string(got) != "// ywai logo source" {
			t.Errorf("installed logo contents = %q, want source bundle", string(got))
		}
	})

	t.Run("registers_logo_path_and_mouse_in_tui_json", func(t *testing.T) {
		bundle := writeBundle(t, "// logo")
		configPath := writeAgentConfig(t, "opencode.json", map[string]any{})

		if err := installTuiLogoWithBundle(configPath, bundle); err != nil {
			t.Fatalf("installTuiLogoWithBundle() error = %v", err)
		}

		tuiPath := tuiConfigPathFor(configPath)
		root := readConfigRoot(t, tuiPath)

		dest := filepath.Join(filepath.Dir(configPath), "tui-plugins", config.TuiLogoBundleName)
		arr, ok := root["plugin"].([]any)
		if !ok {
			t.Fatalf("tui.json has no []any \"plugin\" array; got %T", root["plugin"])
		}
		if !containsString(arr, dest) {
			t.Errorf("tui.json plugin array %v does not contain logo path %q", arr, dest)
		}
		if root["mouse"] != true {
			t.Errorf("tui.json mouse = %v, want true (needed for click easter eggs)", root["mouse"])
		}
	})

	t.Run("does_not_touch_opencode_config", func(t *testing.T) {
		bundle := writeBundle(t, "// logo")
		configPath := writeAgentConfig(t, "opencode.json", map[string]any{"model": "x"})

		if err := installTuiLogoWithBundle(configPath, bundle); err != nil {
			t.Fatalf("installTuiLogoWithBundle() error = %v", err)
		}

		// TUI plugins live in tui.json; opencode.json must be left untouched.
		root := readConfigRoot(t, configPath)
		if _, ok := root["plugin"]; ok {
			t.Errorf("opencode.json gained a \"plugin\" array; logo must only patch tui.json")
		}
		if root["model"] != "x" {
			t.Errorf("opencode.json model = %v, want preserved \"x\"", root["model"])
		}
	})

	t.Run("preserves_existing_tui_plugins_and_mouse_pref", func(t *testing.T) {
		bundle := writeBundle(t, "// logo")
		configPath := writeAgentConfig(t, "opencode.json", map[string]any{})
		tuiPath := tuiConfigPathFor(configPath)

		// Seed tui.json with a string entry, an array-form entry, and mouse off.
		existing := map[string]any{
			"mouse": false,
			"plugin": []any{
				"some-plugin@1.0.0",
				[]any{"@scope/parametrized", map[string]any{"opt": "v"}},
			},
		}
		if err := config.WriteJSONC(tuiPath, existing); err != nil {
			t.Fatalf("seed tui.json: %v", err)
		}

		if err := installTuiLogoWithBundle(configPath, bundle); err != nil {
			t.Fatalf("installTuiLogoWithBundle() error = %v", err)
		}

		root := readConfigRoot(t, tuiPath)
		arr := root["plugin"].([]any)
		if !containsString(arr, "some-plugin@1.0.0") {
			t.Errorf("plugin array %v dropped pre-existing string entry", arr)
		}
		// The array-form entry must survive untouched.
		foundArrayForm := false
		for _, v := range arr {
			if _, ok := v.([]any); ok {
				foundArrayForm = true
			}
		}
		if !foundArrayForm {
			t.Errorf("plugin array %v dropped the array-form entry", arr)
		}
		// Explicit user opt-out of mouse must be respected.
		if root["mouse"] != false {
			t.Errorf("tui.json mouse = %v, want false (user opt-out preserved)", root["mouse"])
		}
	})

	t.Run("idempotent_no_duplicate_plugin_entry", func(t *testing.T) {
		bundle := writeBundle(t, "// logo")
		configPath := writeAgentConfig(t, "opencode.json", map[string]any{})

		if err := installTuiLogoWithBundle(configPath, bundle); err != nil {
			t.Fatalf("first install error = %v", err)
		}
		if err := installTuiLogoWithBundle(configPath, bundle); err != nil {
			t.Fatalf("second install error = %v", err)
		}

		root := readConfigRoot(t, tuiConfigPathFor(configPath))
		dest := filepath.Join(filepath.Dir(configPath), "tui-plugins", config.TuiLogoBundleName)
		arr := root["plugin"].([]any)
		count := 0
		for _, v := range arr {
			if s, ok := v.(string); ok && s == dest {
				count++
			}
		}
		if count != 1 {
			t.Errorf("logo path appears %d times in plugin array %v, want exactly 1", count, arr)
		}
	})
}
