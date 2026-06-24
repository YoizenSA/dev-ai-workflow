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
		// Profiles are keyed by agent name; spot-check core agents.
		for _, agent := range []string{"dev", "qa", "reviewer"} {
			if profile.Agents[agent].Model == "" {
				t.Fatalf("expected profile %q to define a model for agent %q", name, agent)
			}
		}
	}
}

func TestDefaultOrchestratorModelProfiles_FastUsesFlashEverywhere(t *testing.T) {
	profiles := DefaultOrchestratorModelProfiles()

	got := profiles["fast"].Agents["dev"]
	if got.Model != "opencode-admin/deepseek-v4-flash" {
		t.Fatalf("expected fast dev model opencode-admin/deepseek-v4-flash, got %q", got.Model)
	}
}

func TestOrchestratorProfiles_UserOverridePersistsAndWinsForActiveProfile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))

	cfg := DefaultConfig()
	cfg.ActiveOrchestratorProfile = "fast"
	cfg.OrchestratorProfiles = DefaultOrchestratorModelProfiles()
	cfg.OrchestratorProfiles["fast"].Agents["dev"] = RoleDefault{Model: "opencode-admin/custom-model"}

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
	if got := active.Agents["dev"].Model; got != "opencode-admin/custom-model" {
		t.Fatalf("expected persisted active profile override, got model=%q", got)
	}
}

func TestResyncOrchestratorModelProfiles_RestoresSeedsAndPreservesValidActiveProfile(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ActiveOrchestratorProfile = "fast"
	cfg.OrchestratorProfiles = DefaultOrchestratorModelProfiles()
	cfg.OrchestratorProfiles["fast"].Agents["dev"] = RoleDefault{Model: "opencode-admin/custom-model"}

	cfg.ResyncOrchestratorModelProfiles()

	if cfg.ActiveOrchestratorProfile != "fast" {
		t.Fatalf("expected valid active profile to be preserved, got %q", cfg.ActiveOrchestratorProfile)
	}
	got := cfg.OrchestratorProfiles["fast"].Agents["dev"]
	seed := DefaultOrchestratorModelProfiles()["fast"].Agents["dev"]
	if !reflect.DeepEqual(got, seed) {
		t.Fatalf("expected resync to restore seeded fast dev mapping, got %+v want %+v", got, seed)
	}
}

func TestResyncOrchestratorModelProfiles_FallsBackDeterministicallyWhenActiveProfileRemoved(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ActiveOrchestratorProfile = "experimental"
	cfg.OrchestratorProfiles = map[string]OrchestratorModelProfile{
		"experimental": {
			DisplayName: "Experimental",
			Agents: RoleDefaults{
				"dev": {Model: "opencode-admin/custom-model"},
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

func TestGetOrchestratorAgentModel(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ActiveOrchestratorProfile = "fast"
	if got := cfg.GetOrchestratorAgentModel("dev"); got != "opencode-admin/deepseek-v4-flash" {
		t.Fatalf("expected fast dev model, got %q", got)
	}
	if got := cfg.GetOrchestratorAgentModel("nonexistent-agent"); got != "" {
		t.Fatalf("expected empty model for unknown agent, got %q", got)
	}
}
