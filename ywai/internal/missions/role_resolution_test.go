package missions

import (
	"context"
	"errors"
	"net"
	"reflect"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

func TestResolveExecution_FeatureOverrideWins(t *testing.T) {
	cfg := config.DefaultConfig()
	feat := &Feature{Role: config.RoleQA, Model: "user/feature-model", Agent: "feat-agent"}
	mission := &Mission{Model: "user/mission-model", ExecutionAgent: "mission-agent"}

	model, agent, chain := ResolveExecution(feat, mission, cfg)
	if model != "user/feature-model" {
		t.Fatalf("expected feature model to win, got %q", model)
	}
	if agent != "feat-agent" {
		t.Fatalf("expected feature agent to win, got %q", agent)
	}
	if len(chain) == 0 || chain[0] != "user/feature-model" {
		t.Fatalf("expected chain[0]=feature model, got %v", chain)
	}
}

func TestResolveExecution_MissionFillsWhenFeatureEmpty(t *testing.T) {
	cfg := config.DefaultConfig()
	feat := &Feature{Role: config.RoleDev}
	mission := &Mission{Model: "user/mission-model", ExecutionAgent: "mission-agent"}

	model, agent, _ := ResolveExecution(feat, mission, cfg)
	if model != "user/mission-model" {
		t.Fatalf("expected mission model fallback, got %q", model)
	}
	if agent != "mission-agent" {
		t.Fatalf("expected mission agent fallback, got %q", agent)
	}
}

func TestResolveExecution_RoleDefaultFillsWhenAllEmpty(t *testing.T) {
	cfg := config.DefaultConfig()
	feat := &Feature{Role: config.RoleQA}
	mission := &Mission{}

	model, agent, chain := ResolveExecution(feat, mission, cfg)
	if model == "" {
		t.Fatalf("expected role-default model, got empty")
	}
	if agent == "" {
		t.Fatalf("expected role-default agent, got empty")
	}
	if len(chain) < 2 {
		t.Fatalf("expected role default to contribute fallbacks, chain=%v", chain)
	}
}

func TestResolveExecution_InferRoleFromSkillName(t *testing.T) {
	cfg := config.DefaultConfig()
	feat := &Feature{SkillName: "frontend-worker"}
	mission := &Mission{}

	model, _, _ := ResolveExecution(feat, mission, cfg)
	if model == "" {
		t.Fatalf("expected SkillNameToRole inference to populate frontend model")
	}
	// Inference must resolve to the frontend role's seeded primary model,
	// whatever it is configured to be (no hardcoded provider coupling).
	wantModel := cfg.GetRoleDefault(config.RoleFrontend).Model
	if model != wantModel {
		t.Fatalf("expected frontend seed model %q, got %q", wantModel, model)
	}
}

func TestResolveExecution_ChainDedupAndCap(t *testing.T) {
	cfg := &config.UserConfig{
		RoleDefaults: config.RoleDefaults{
			config.RoleDev: {
				Model:     "primary",
				Fallbacks: []string{"fb1", "fb2", "fb3", "fb4"},
			},
		},
	}
	feat := &Feature{Role: config.RoleDev, Fallbacks: []string{"primary", "fb1"}}
	mission := &Mission{}

	_, _, chain := ResolveExecution(feat, mission, cfg)
	if len(chain) > MaxFallbackChainLen {
		t.Fatalf("chain exceeds cap: %v", chain)
	}
	// Primary should appear exactly once and be first.
	if chain[0] != "primary" {
		t.Fatalf("expected primary first, got %v", chain)
	}
	seen := map[string]int{}
	for _, m := range chain {
		seen[m]++
	}
	if seen["primary"] != 1 {
		t.Fatalf("primary not deduplicated: %v", chain)
	}
	if seen["fb1"] != 1 {
		t.Fatalf("fb1 not deduplicated: %v", chain)
	}
}

func TestResolveExecution_NilFeature(t *testing.T) {
	cfg := config.DefaultConfig()
	mission := &Mission{Model: "user/mission", ExecutionAgent: "agent"}

	model, agent, chain := ResolveExecution(nil, mission, cfg)
	if model != "user/mission" {
		t.Fatalf("expected mission model when feature nil, got %q", model)
	}
	if agent != "agent" {
		t.Fatalf("expected mission agent, got %q", agent)
	}
	if !reflect.DeepEqual(chain, []string{"user/mission"}) {
		t.Fatalf("expected chain=[user/mission], got %v", chain)
	}
}

func TestIsRetriableModelError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"deadline", context.DeadlineExceeded, true},
		{"net error", &net.OpError{Op: "dial", Err: errors.New("no route")}, true},
		{"rate limit", errors.New("HTTP 429 rate limit exceeded"), true},
		{"model not found", errors.New("model_not_found: anthropic/unknown"), true},
		{"unavailable", errors.New("service unavailable"), true},
		{"overloaded", errors.New("provider overloaded, retry later"), true},
		{"timeout", errors.New("request timed out after 30s"), true},
		{"auth error", errors.New("401 unauthorized"), false},
		{"validation", errors.New("invalid prompt: empty messages"), false},
		{"content policy", errors.New("content blocked by safety filter"), false},
	}
	for _, c := range cases {
		got := isRetriableModelError(c.err)
		if got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestRoleToSkillName_Coverage(t *testing.T) {
	expected := map[string]string{
		config.RoleDev:      "implementation",
		config.RoleFrontend: "frontend-worker",
		config.RoleBackend:  "backend-worker",
		config.RoleQA:       "qa-worker",
		config.RoleReviewer: "reviewer-worker",
		config.RoleDevops:   "devops-worker",
		config.RolePlanning: "planner",
		"unknown-role":      "implementation",
	}
	for role, want := range expected {
		if got := RoleToSkillName(role); got != want {
			t.Errorf("RoleToSkillName(%q)=%q, want %q", role, got, want)
		}
	}
}

func TestSkillNameToRole_Coverage(t *testing.T) {
	expected := map[string]string{
		"frontend-worker": config.RoleFrontend,
		"backend-worker":  config.RoleBackend,
		"qa-worker":       config.RoleQA,
		"reviewer-worker": config.RoleReviewer,
		"devops-worker":   config.RoleDevops,
		"planner":         config.RolePlanning,
		"implementation":  config.RoleDev,
		"":                config.RoleDev,
		"custom":          config.RoleDev,
	}
	for skill, want := range expected {
		if got := SkillNameToRole(skill); got != want {
			t.Errorf("SkillNameToRole(%q)=%q, want %q", skill, got, want)
		}
	}
}
