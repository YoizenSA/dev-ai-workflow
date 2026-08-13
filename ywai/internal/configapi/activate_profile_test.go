package configapi

import (
	"os"
	"path/filepath"
	"testing"

	userconfig "github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

func TestActivateOrchestratorProfilePersistsName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	userconfig.ResetConfig()
	t.Cleanup(userconfig.ResetConfig)

	if err := os.MkdirAll(userconfig.DataDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := userconfig.SaveConfig(userconfig.DefaultConfig()); err != nil {
		t.Fatal(err)
	}

	if _, err := ActivateOrchestratorProfile("fast"); err != nil {
		t.Fatalf("activate fast: %v", err)
	}
	cfg, err := userconfig.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ActiveOrchestratorProfile != "fast" {
		t.Fatalf("active = %q, want fast", cfg.ActiveOrchestratorProfile)
	}
}

func TestActivateOrchestratorProfileRejectsUnknown(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	userconfig.ResetConfig()
	t.Cleanup(userconfig.ResetConfig)

	if err := os.MkdirAll(userconfig.DataDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := userconfig.SaveConfig(userconfig.DefaultConfig()); err != nil {
		t.Fatal(err)
	}

	_, err := ActivateOrchestratorProfile("nope")
	if err == nil {
		t.Fatal("expected unknown profile error")
	}
}
