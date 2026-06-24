package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDefaultOrchestratorModelProfiles_SeedsThreeProfiles(t *testing.T) {
	profiles := DefaultOrchestratorModelProfiles()

	if len(profiles) != 3 {
		t.Fatalf("expected exactly 3 seeded orchestrator profiles, got %d", len(profiles))
	}

	for _, name := range []string{"balanced", "fast", "deep"} {
		profile, ok := profiles[name]
		if !ok {
			t.Fatalf("expected seeded profile %q to exist; profiles=%v", name, profiles)
		}
		if profile.DisplayName == "" {
			t.Fatalf("expected profile %q to have a display name", name)
		}
		if profile.RoleDefaults["explore"].Model == "" {
			t.Fatalf("expected profile %q to define an explore model", name)
		}
		if profile.RoleDefaults["scout"].Model == "" {
			t.Fatalf("expected profile %q to define a scout model", name)
		}
		if profile.RoleDefaults["finder"].Model == "" {
			t.Fatalf("expected profile %q to define a finder model", name)
		}
	}
}

func TestDefaultOrchestratorModelProfiles_IncludesRequestedFastScoutMapping(t *testing.T) {
	profiles := DefaultOrchestratorModelProfiles()

	got := profiles["fast"].RoleDefaults["scout"]
	if got.Agent != "opencode-admin" {
		t.Fatalf("expected fast scout agent opencode-admin, got %q", got.Agent)
	}
	if got.Model != "deepseek-v4-flash" {
		t.Fatalf("expected fast scout model deepseek-v4-flash, got %q", got.Model)
	}
}

func TestOrchestratorProfiles_UserOverridePersistsAndWinsForActiveProfile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))

	cfg := DefaultConfig()
	cfg.ActiveOrchestratorProfile = "fast"
	cfg.OrchestratorProfiles = DefaultOrchestratorModelProfiles()
	cfg.OrchestratorProfiles["fast"].RoleDefaults["scout"] = RoleDefault{
		Agent: "custom-agent",
		Model: "custom/model",
	}

	if err := os.MkdirAll(DataDir(), 0o755); err != nil {
		t.Fatalf("create data dir: %v", err)
	}
	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	active := loaded.GetActiveOrchestratorProfile()
	got := active.RoleDefaults["scout"]
	if got.Agent != "custom-agent" || got.Model != "custom/model" {
		t.Fatalf("expected persisted active profile override, got agent=%q model=%q", got.Agent, got.Model)
	}
}

func TestResyncOrchestratorModelProfiles_RestoresSeedsAndPreservesValidActiveProfile(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ActiveOrchestratorProfile = "fast"
	cfg.OrchestratorProfiles = DefaultOrchestratorModelProfiles()
	cfg.OrchestratorProfiles["fast"].RoleDefaults["scout"] = RoleDefault{Agent: "custom-agent", Model: "custom/model"}

	cfg.ResyncOrchestratorModelProfiles()

	if cfg.ActiveOrchestratorProfile != "fast" {
		t.Fatalf("expected valid active profile to be preserved, got %q", cfg.ActiveOrchestratorProfile)
	}
	got := cfg.OrchestratorProfiles["fast"].RoleDefaults["scout"]
	seed := DefaultOrchestratorModelProfiles()["fast"].RoleDefaults["scout"]
	if !reflect.DeepEqual(got, seed) {
		t.Fatalf("expected resync to restore seeded fast scout mapping, got %+v want %+v", got, seed)
	}
}

func TestResyncOrchestratorModelProfiles_FallsBackDeterministicallyWhenActiveProfileRemoved(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ActiveOrchestratorProfile = "experimental"
	cfg.OrchestratorProfiles = map[string]OrchestratorModelProfile{
		"experimental": {
			DisplayName: "Experimental",
			RoleDefaults: RoleDefaults{
				"scout": {Agent: "custom-agent", Model: "custom/model"},
			},
		},
	}

	cfg.ResyncOrchestratorModelProfiles()

	if cfg.ActiveOrchestratorProfile != DefaultOrchestratorModelProfileName {
		t.Fatalf("expected missing active profile to fall back to %q, got %q", DefaultOrchestratorModelProfileName, cfg.ActiveOrchestratorProfile)
	}
	if _, ok := cfg.OrchestratorProfiles["experimental"]; ok {
		t.Fatalf("expected resync to drop non-seed experimental profile")
	}
}
