package missions

import (
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

func TestDetectRole_CoversAllCanonical(t *testing.T) {
	cases := []struct {
		desc  string
		hints generationHint
		want  string
	}{
		{"Audit the auth diff for security issues", generationHint{}, config.RoleReviewer},
		{"Add unit tests for login flow", generationHint{}, config.RoleQA},
		{"Build the dashboard UI component", generationHint{}, config.RoleFrontend},
		{"Create POST /api/users endpoint", generationHint{}, config.RoleBackend},
		{"Set up docker-compose for staging", generationHint{}, config.RoleDevops},
		{"Implement the calculation engine", generationHint{}, config.RoleDev},
		{"Plan the release", generationHint{Technologies: []string{"react"}}, config.RoleFrontend},
		{"Generic feature", generationHint{Technologies: []string{"go", "backend"}}, config.RoleBackend},
	}
	for _, c := range cases {
		got := detectRole(c.desc, c.hints)
		if got != c.want {
			t.Errorf("detectRole(%q)=%q, want %q", c.desc, got, c.want)
		}
	}
}

func TestApplyRoleDefaults_PopulatesEmptyFields(t *testing.T) {
	cfg := config.DefaultConfig()
	features := []PlanFeature{
		{ID: "1", Description: "Build React component", Milestone: "m1"},
		{ID: "2", Description: "Test the API", Milestone: "m1"},
	}
	applyRoleDefaults(features, generationHint{}, cfg)

	if features[0].Role != config.RoleFrontend {
		t.Errorf("expected frontend role, got %q", features[0].Role)
	}
	if features[0].Model == "" {
		t.Errorf("expected role-default model populated")
	}
	if features[0].Agent == "" {
		t.Errorf("expected role-default agent populated")
	}
	if len(features[0].Fallbacks) == 0 {
		t.Errorf("expected role-default fallbacks populated")
	}
	if features[1].Role != config.RoleQA {
		t.Errorf("expected qa role for feature 2, got %q", features[1].Role)
	}
}

func TestApplyRoleDefaults_KeepsExplicitOverrides(t *testing.T) {
	cfg := config.DefaultConfig()
	features := []PlanFeature{
		{
			ID:          "1",
			Description: "Anything",
			Role:        config.RoleDev,
			Model:       "user/explicit",
			Agent:       "user-agent",
			Fallbacks:   []string{"user/fb"},
		},
	}
	applyRoleDefaults(features, generationHint{}, cfg)

	if features[0].Model != "user/explicit" {
		t.Errorf("explicit model overwritten: %q", features[0].Model)
	}
	if features[0].Agent != "user-agent" {
		t.Errorf("explicit agent overwritten: %q", features[0].Agent)
	}
	if len(features[0].Fallbacks) != 1 || features[0].Fallbacks[0] != "user/fb" {
		t.Errorf("explicit fallbacks overwritten: %v", features[0].Fallbacks)
	}
}
