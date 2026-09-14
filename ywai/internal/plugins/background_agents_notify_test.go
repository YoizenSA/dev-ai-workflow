package plugins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

func TestInstallBackgroundAgentsNotifyWithBundle(t *testing.T) {
	t.Run("vendors_sidecar_dir_with_tui_entry_next_to_config", func(t *testing.T) {
		bundle := writeBundle(t, "// notify sidecar source")
		configPath := writeAgentConfig(t, "opencode.json", map[string]any{})

		if err := installBackgroundAgentsNotifyWithBundle(configPath, bundle); err != nil {
			t.Fatalf("installBackgroundAgentsNotifyWithBundle() error = %v", err)
		}

		dest := filepath.Join(filepath.Dir(configPath), autoDiscoveredPluginsSubdir, BackgroundAgentsNotifyPluginDir, tuiEntryName)
		got, err := os.ReadFile(dest)
		if err != nil {
			t.Fatalf("read installed sidecar: %v", err)
		}
		if string(got) != "// notify sidecar source" {
			t.Errorf("installed sidecar contents = %q, want source bundle", string(got))
		}
	})

	t.Run("registers_sidecar_path_in_cli_json_without_mouse", func(t *testing.T) {
		bundle := writeBundle(t, "// notify")
		configPath := writeAgentConfig(t, "opencode.json", map[string]any{})

		if err := installBackgroundAgentsNotifyWithBundle(configPath, bundle); err != nil {
			t.Fatalf("installBackgroundAgentsNotifyWithBundle() error = %v", err)
		}

		tuiPath := tuiConfigPathFor(configPath)
		root := readConfigRoot(t, tuiPath)

		dest := filepath.Join(filepath.Dir(configPath), autoDiscoveredPluginsSubdir, BackgroundAgentsNotifyPluginDir)
		arr, ok := root["plugins"].([]any)
		if !ok {
			t.Fatalf("cli.json has no []any \"plugins\" array; got %T", root["plugins"])
		}
		if !containsString(arr, dest) {
			t.Errorf("cli.json plugin array %v does not contain sidecar path %q", arr, dest)
		}
		// Renders nothing: must not enable mouse capture like the logo.
		if _, hasMouse := root["mouse"]; hasMouse {
			t.Errorf("cli.json mouse = %v, want absent (sidecar needs no mouse)", root["mouse"])
		}
	})

	t.Run("missing_bundle_is_an_error", func(t *testing.T) {
		configPath := writeAgentConfig(t, "opencode.json", map[string]any{})
		missing := filepath.Join(t.TempDir(), "no-"+config.BackgroundAgentsNotifyBundleName)
		if err := installBackgroundAgentsNotifyWithBundle(configPath, missing); err == nil {
			t.Fatal("installBackgroundAgentsNotifyWithBundle() with missing bundle = nil, want error")
		}
	})
}
