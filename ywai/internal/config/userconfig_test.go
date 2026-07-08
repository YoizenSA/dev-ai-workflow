package config

import (
	"testing"
)

func TestGetRoleDefault_NilMapReturnsSeed(t *testing.T) {
	cfg := &UserConfig{}
	rd := cfg.GetRoleDefault(RolePlanning)
	if rd.Model == "" {
		t.Fatalf("expected seeded model for planning, got empty")
	}
	if len(rd.Fallbacks) == 0 {
		t.Fatalf("expected seeded fallbacks for planning, got none")
	}
}

func TestGetRoleDefault_UserOverrideWins(t *testing.T) {
	cfg := &UserConfig{
		RoleDefaults: RoleDefaults{
			RoleQA: {Agent: "qa-agent", Model: "custom/qa-model"},
		},
	}
	rd := cfg.GetRoleDefault(RoleQA)
	if rd.Model != "custom/qa-model" {
		t.Fatalf("expected user-supplied model, got %q", rd.Model)
	}
	if rd.Agent != "qa-agent" {
		t.Fatalf("expected user-supplied agent, got %q", rd.Agent)
	}
}

func TestGetRoleDefault_EmptyOverrideFallsBackToSeed(t *testing.T) {
	cfg := &UserConfig{
		RoleDefaults: RoleDefaults{
			RoleDev: {}, // explicitly empty, should fall back to seed
		},
	}
	rd := cfg.GetRoleDefault(RoleDev)
	if rd.Model == "" {
		t.Fatalf("expected seed to populate model for empty override, got empty")
	}
}

func TestGetRoleDefault_UnknownRoleReturnsZero(t *testing.T) {
	cfg := &UserConfig{}
	rd := cfg.GetRoleDefault("not-a-role")
	if !rd.isEmpty() {
		t.Fatalf("expected empty RoleDefault for unknown role, got %+v", rd)
	}
}

func TestDefaultConfig_SeedsAllCanonicalRoles(t *testing.T) {
	cfg := DefaultConfig()
	for _, role := range CanonicalRoles {
		rd, ok := cfg.RoleDefaults[role]
		if !ok {
			t.Errorf("role %q missing from default RoleDefaults", role)
			continue
		}
		if rd.Model == "" {
			t.Errorf("role %q has empty model in defaults", role)
		}
		if rd.Agent == "" {
			t.Errorf("role %q has empty agent in defaults", role)
		}
	}
}

func TestGetRoleDefault_NilReceiver(t *testing.T) {
	var cfg *UserConfig
	rd := cfg.GetRoleDefault(RolePlanning)
	if rd.Model == "" {
		t.Fatalf("expected seed model when receiver is nil, got empty")
	}
}

func TestGetVisionModel(t *testing.T) {
	t.Run("default_value", func(t *testing.T) {
		cfg := &UserConfig{}
		got := cfg.GetVisionModel()
		if got != DefaultVisionModel {
			t.Errorf("GetVisionModel() = %q, want %q", got, DefaultVisionModel)
		}
	})

	t.Run("override_wins", func(t *testing.T) {
		cfg := &UserConfig{
			VisionModel:         "gpt-4o",
			VisionModelOverride: "claude-3-opus",
		}
		got := cfg.GetVisionModel()
		if got != "claude-3-opus" {
			t.Errorf("GetVisionModel() = %q, want %q", got, "claude-3-opus")
		}
	})

	t.Run("vision_model_when_no_override", func(t *testing.T) {
		cfg := &UserConfig{
			VisionModel: "gpt-4o",
		}
		got := cfg.GetVisionModel()
		if got != "gpt-4o" {
			t.Errorf("GetVisionModel() = %q, want %q", got, "gpt-4o")
		}
	})

	t.Run("nil_receiver_returns_default", func(t *testing.T) {
		var cfg *UserConfig
		got := cfg.GetVisionModel()
		if got != DefaultVisionModel {
			t.Errorf("GetVisionModel() = %q, want %q", got, DefaultVisionModel)
		}
	})
}
