package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ─── InstallADOOpenCode ───────────────────────────────────────────────────

func TestInstallADOOpenCode_WritesToPluginConfigFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	adoConfig := ADOPluginConfig{
		DefaultProfile: "work",
		Profiles: map[string]ADOProfile{
			"work": {Org: "myorg", Project: "myproject"},
		},
	}

	if err := InstallADOOpenCode(adoConfig); err != nil {
		t.Fatalf("InstallADOOpenCode() error = %v", err)
	}

	pluginPath := filepath.Join(home, ".config", "opencode", "ado-plugin.json")
	data, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatalf("failed to read ado-plugin.json: %v", err)
	}

	var config ADOPluginConfig
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("failed to unmarshal ado-plugin.json: %v", err)
	}

	if config.DefaultProfile != "work" {
		t.Fatalf("defaultProfile = %q, want work", config.DefaultProfile)
	}
	if config.Profiles["work"].Org != "myorg" {
		t.Fatalf("org = %q, want myorg", config.Profiles["work"].Org)
	}
}

func TestInstallADOOpenCode_DoesNotModifyOpenCodeJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	configPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeJSON(t, configPath, map[string]any{
		"theme": "dark",
	})

	adoConfig := ADOPluginConfig{
		DefaultProfile: "work",
		Profiles:       map[string]ADOProfile{},
	}

	if err := InstallADOOpenCode(adoConfig); err != nil {
		t.Fatalf("InstallADOOpenCode() error = %v", err)
	}

	var result map[string]any
	readJSON(t, configPath, &result)

	if _, ok := result["plugin"]; ok {
		t.Fatal("opencode.json should not have a \"plugin\" key")
	}
	if result["theme"] != "dark" {
		t.Fatalf("opencode.json was modified: theme = %v", result["theme"])
	}
}

func TestInstallADOOpenCode_OverwritesExistingPluginConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	adoConfig := ADOPluginConfig{
		DefaultProfile: "old",
		Profiles: map[string]ADOProfile{
			"old": {Org: "oldorg"},
		},
	}
	if err := InstallADOOpenCode(adoConfig); err != nil {
		t.Fatalf("InstallADOOpenCode() error = %v", err)
	}

	newConfig := ADOPluginConfig{
		DefaultProfile: "work",
		Profiles: map[string]ADOProfile{
			"work": {Org: "neworg", Project: "newproject"},
		},
	}
	if err := InstallADOOpenCode(newConfig); err != nil {
		t.Fatalf("InstallADOOpenCode() error = %v", err)
	}

	pluginPath := filepath.Join(home, ".config", "opencode", "ado-plugin.json")
	data, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatalf("failed to read ado-plugin.json: %v", err)
	}

	var config ADOPluginConfig
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("failed to unmarshal ado-plugin.json: %v", err)
	}

	if config.DefaultProfile != "work" {
		t.Fatalf("defaultProfile = %q, want work", config.DefaultProfile)
	}
}

// ─── RemoveADOOpenCode ─────────────────────────────────────────────────────

func TestRemoveADOOpenCode_DeletesPluginConfigFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	adoConfig := ADOPluginConfig{
		DefaultProfile: "work",
		Profiles:       map[string]ADOProfile{},
	}
	if err := InstallADOOpenCode(adoConfig); err != nil {
		t.Fatalf("InstallADOOpenCode() error = %v", err)
	}

	pluginPath := filepath.Join(home, ".config", "opencode", "ado-plugin.json")
	if _, err := os.Stat(pluginPath); err != nil {
		t.Fatalf("ado-plugin.json should exist: %v", err)
	}

	if err := RemoveADOOpenCode(); err != nil {
		t.Fatalf("RemoveADOOpenCode() error = %v", err)
	}

	if _, err := os.Stat(pluginPath); !os.IsNotExist(err) {
		t.Fatal("ado-plugin.json should have been deleted")
	}
}

func TestRemoveADOOpenCode_NoOpWhenFileDoesNotExist(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	if err := RemoveADOOpenCode(); err != nil {
		t.Fatalf("RemoveADOOpenCode() should not error when file does not exist: %v", err)
	}
}

// ─── ReadExistingADOConfig (new file) ──────────────────────────────────────

func TestReadExistingADOConfig_ReadsFromPluginConfigFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	adoConfig := ADOPluginConfig{
		DefaultProfile: "work",
		Profiles: map[string]ADOProfile{
			"work": {Org: "myorg", PATEnvVar: "MY_PAT", Project: "myproject"},
		},
	}
	if err := InstallADOOpenCode(adoConfig); err != nil {
		t.Fatalf("InstallADOOpenCode() error = %v", err)
	}

	config, err := ReadExistingADOConfig()
	if err != nil {
		t.Fatalf("ReadExistingADOConfig() error = %v", err)
	}
	if config == nil {
		t.Fatal("expected config, got nil")
	}
	if config.DefaultProfile != "work" {
		t.Fatalf("defaultProfile = %q, want work", config.DefaultProfile)
	}
	if config.Profiles["work"].Org != "myorg" {
		t.Fatalf("org = %q, want myorg", config.Profiles["work"].Org)
	}
}

func TestReadExistingADOConfig_ReadsFromLegacyOpenCodeJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	// Simulate legacy opencode.json with plugin entry
	configPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeJSON(t, configPath, map[string]any{
		"plugin": []any{
			[]any{"@nahuelcio/opencode-ado", map[string]any{
				"defaultProfile": "legacy",
				"profiles": map[string]any{
					"legacy": map[string]any{
						"org":       "legacyorg",
						"patEnvVar": "LEGACY_PAT",
						"project":   "legacyproject",
					},
				},
			}},
		},
	})

	config, err := ReadExistingADOConfig()
	if err != nil {
		t.Fatalf("ReadExistingADOConfig() error = %v", err)
	}
	if config == nil {
		t.Fatal("expected config from legacy opencode.json, got nil")
	}
	if config.DefaultProfile != "legacy" {
		t.Fatalf("defaultProfile = %q, want legacy", config.DefaultProfile)
	}
}

func TestReadExistingADOConfig_PrefersNewFileOverLegacy(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	// Write legacy opencode.json
	configPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeJSON(t, configPath, map[string]any{
		"plugin": []any{
			[]any{"@nahuelcio/opencode-ado", map[string]any{
				"defaultProfile": "legacy",
				"profiles":       map[string]any{},
			}},
		},
	})

	// Write new plugin config
	newConfig := ADOPluginConfig{
		DefaultProfile: "new",
		Profiles: map[string]ADOProfile{
			"new": {Org: "neworg"},
		},
	}
	if err := InstallADOOpenCode(newConfig); err != nil {
		t.Fatalf("InstallADOOpenCode() error = %v", err)
	}

	config, err := ReadExistingADOConfig()
	if err != nil {
		t.Fatalf("ReadExistingADOConfig() error = %v", err)
	}
	if config == nil {
		t.Fatal("expected config, got nil")
	}
	if config.DefaultProfile != "new" {
		t.Fatalf("defaultProfile = %q, want new (should prefer new file over legacy)", config.DefaultProfile)
	}
}

// ─── InstallADOPi ─────────────────────────────────────────────────────────

func TestInstallADOPi_AddsToEmptySettings(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	writeJSON(t, settingsPath, map[string]any{
		"packages": []any{},
	})

	if err := InstallADOPi(settingsPath); err != nil {
		t.Fatalf("InstallADOPi() error = %v", err)
	}

	var result map[string]any
	readJSON(t, settingsPath, &result)

	packages := result["packages"].([]any)
	found := false
	for _, p := range packages {
		if p == "npm:@nahuelcio/opencode-ado" {
			found = true
		}
	}
	if !found {
		t.Fatalf("packages = %v, want npm:@nahuelcio/opencode-ado included", packages)
	}
}

func TestInstallADOPi_SkipsIfAlreadyInstalled(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	writeJSON(t, settingsPath, map[string]any{
		"packages": []any{"npm:@nahuelcio/opencode-ado"},
	})

	if err := InstallADOPi(settingsPath); err != nil {
		t.Fatalf("InstallADOPi() error = %v", err)
	}

	var result map[string]any
	readJSON(t, settingsPath, &result)

	packages := result["packages"].([]any)
	if len(packages) != 1 {
		t.Fatalf("expected 1 package (no duplicate), got %d", len(packages))
	}
}

func TestInstallADOPi_CreatesFileIfNotExists(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	// Don't create the file — InstallADOPi should create it
	// But actually the current impl fails to read, so it creates from scratch
	if err := InstallADOPi(settingsPath); err != nil {
		t.Fatalf("InstallADOPi() error = %v", err)
	}

	var result map[string]any
	readJSON(t, settingsPath, &result)

	packages := result["packages"].([]any)
	if len(packages) != 1 || packages[0] != "npm:@nahuelcio/opencode-ado" {
		t.Fatalf("packages = %v, want [npm:@nahuelcio/opencode-ado]", packages)
	}
}

// ─── InstallADODefaultConfig ──────────────────────────────────────────────

func TestInstallADODefaultConfig_CreatesConfigFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	config := ADOPluginConfig{
		DefaultProfile: "work",
		Profiles: map[string]ADOProfile{
			"work": {
				Org:     "myorg",
				Project: "myproject",
				Repos:   []string{"backend"},
			},
		},
	}

	if err := InstallADODefaultConfig(config); err != nil {
		t.Fatalf("InstallADODefaultConfig() error = %v", err)
	}

	configPath := filepath.Join(home, ".azure-devops-cli", "config.json")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	var result map[string]any
	readJSON(t, configPath, &result)

	ado := result["ado"].(map[string]any)
	if ado["defaultProfile"] != "work" {
		t.Fatalf("defaultProfile = %v, want work", ado["defaultProfile"])
	}
}

func TestInstallADODefaultConfig_MergesProfiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	// Write initial config
	configDir := filepath.Join(home, ".azure-devops-cli")
	os.MkdirAll(configDir, 0o755)
	writeJSON(t, filepath.Join(configDir, "config.json"), map[string]any{
		"ado": map[string]any{
			"defaultProfile": "old",
			"profiles": map[string]any{
				"old": map[string]any{
					"org":     "oldorg",
					"project": "oldproject",
				},
			},
		},
	})

	config := ADOPluginConfig{
		DefaultProfile: "new",
		Profiles: map[string]ADOProfile{
			"new": {Org: "neworg", Project: "newproject"},
		},
	}

	if err := InstallADODefaultConfig(config); err != nil {
		t.Fatalf("InstallADODefaultConfig() error = %v", err)
	}

	readConfig, err := ReadExistingADOConfig()
	if err != nil {
		t.Fatalf("ReadExistingADOConfig() error = %v", err)
	}

	if readConfig.DefaultProfile != "new" {
		t.Fatalf("defaultProfile = %q, want new", readConfig.DefaultProfile)
	}
	if _, ok := readConfig.Profiles["old"]; !ok {
		t.Fatal("old profile should be preserved after merge")
	}
	if _, ok := readConfig.Profiles["new"]; !ok {
		t.Fatal("new profile should be added after merge")
	}
}

// ─── ReadExistingADOConfig ────────────────────────────────────────────────

func TestReadExistingADOConfig_ReturnsNilWhenNoConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	config, err := ReadExistingADOConfig()
	if err != nil {
		t.Fatalf("ReadExistingADOConfig() error = %v", err)
	}
	if config != nil {
		t.Fatalf("expected nil config when no file, got %+v", config)
	}
}

func TestReadExistingADOConfig_ParsesConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	configDir := filepath.Join(home, ".azure-devops-cli")
	os.MkdirAll(configDir, 0o755)
	writeJSON(t, filepath.Join(configDir, "config.json"), map[string]any{
		"ado": map[string]any{
			"defaultProfile": "work",
			"profiles": map[string]any{
				"work": map[string]any{
					"org":       "myorg",
					"patEnvVar": "MY_PAT",
					"project":   "myproject",
					"repos":     []any{"backend"},
				},
			},
		},
	})

	config, err := ReadExistingADOConfig()
	if err != nil {
		t.Fatalf("ReadExistingADOConfig() error = %v", err)
	}
	if config == nil {
		t.Fatal("expected config, got nil")
	}
	if config.DefaultProfile != "work" {
		t.Fatalf("defaultProfile = %q, want work", config.DefaultProfile)
	}
	if config.Profiles["work"].Org != "myorg" {
		t.Fatalf("org = %q, want myorg", config.Profiles["work"].Org)
	}
	if config.Profiles["work"].PATEnvVar != "MY_PAT" {
		t.Fatalf("patEnvVar = %q, want MY_PAT", config.Profiles["work"].PATEnvVar)
	}
}

// ─── ADODefaultConfigExists ───────────────────────────────────────────────

func TestADODefaultConfigExists_FalseWhenNoConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	if ADODefaultConfigExists() {
		t.Fatal("expected false when no config dir exists")
	}
}

func TestADODefaultConfigExists_TrueWhenConfigExists(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	configDir := filepath.Join(home, ".azure-devops-cli")
	os.MkdirAll(configDir, 0o755)
	os.WriteFile(filepath.Join(configDir, "config.json"), []byte("{}"), 0o644)

	if !ADODefaultConfigExists() {
		t.Fatal("expected true when config.json exists")
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────

func writeJSON(t *testing.T, path string, data any) {
	t.Helper()
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	b = append(b, '\n')
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readJSON(t *testing.T, path string, target any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(b, target); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
}
