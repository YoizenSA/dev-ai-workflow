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

	t.Run("does_not_touch_opencode_config", func(t *testing.T) {
		bundle := writeBundle(t, "// logo")
		configPath := writeAgentConfig(t, "opencode.json", map[string]any{"model": "x"})

		if err := installTuiLogoWithBundle(configPath, bundle); err != nil {
			t.Fatalf("installTuiLogoWithBundle() error = %v", err)
		}

		// The TUI logo is auto-discovered from tui-plugins/; the config must be
		// left untouched (no plugin array, no permission changes).
		root := readConfigRoot(t, configPath)
		if _, ok := root["plugin"]; ok {
			t.Errorf("config gained a \"plugin\" array; TUI logo must not patch config")
		}
		if root["model"] != "x" {
			t.Errorf("config model = %v, want preserved \"x\"", root["model"])
		}
	})

	t.Run("idempotent_overwrite", func(t *testing.T) {
		configPath := writeAgentConfig(t, "opencode.json", map[string]any{})

		if err := installTuiLogoWithBundle(configPath, writeBundle(t, "// v1")); err != nil {
			t.Fatalf("first install error = %v", err)
		}
		if err := installTuiLogoWithBundle(configPath, writeBundle(t, "// v2")); err != nil {
			t.Fatalf("second install error = %v", err)
		}

		dest := filepath.Join(filepath.Dir(configPath), "tui-plugins", config.TuiLogoBundleName)
		got, _ := os.ReadFile(dest)
		if string(got) != "// v2" {
			t.Errorf("installed logo = %q, want latest \"// v2\"", string(got))
		}
	})
}
