package e2e

import (
	"strings"
	"testing"
)

func TestSkillsList(t *testing.T) {
	bin := buildBinary(t)
	out := runYwai(t, bin, "skills")

	expected := []string{
		"angular", "biome", "devops", "dotnet", "git-commit",
		"playwright", "react-19", "tailwind-4", "typescript", "yz-ui",
	}
	for _, skill := range expected {
		if !strings.Contains(out, skill) {
			t.Errorf("expected skill %q in skills output", skill)
		}
	}

	if !strings.Contains(out, "Total:") {
		t.Errorf("expected 'Total:' in skills output, got: %s", out)
	}
}

func TestSkillsShowsProfiles(t *testing.T) {
	bin := buildBinary(t)
	out := runYwai(t, bin, "skills")

	if !strings.Contains(out, "Skills by profile:") {
		t.Errorf("expected 'Skills by profile:' header, got: %s", out)
	}

	expectedProfiles := map[string]string{
		"react":  "react-19",
		"nest":   "typescript",
		"dotnet": "dotnet",
		"devops": "devops",
	}
	for ptype, expectedSkill := range expectedProfiles {
		if !strings.Contains(out, ptype) {
			t.Errorf("expected profile %q in output", ptype)
		}
		if !strings.Contains(out, expectedSkill) {
			t.Errorf("expected skill %q for profile %q", expectedSkill, ptype)
		}
	}
}

func TestSkillsFilteredByType(t *testing.T) {
	bin := buildBinary(t)
	out := runYwai(t, bin, "skills", "--type", "react")

	reactSkills := []string{"react-19", "tailwind-4", "typescript", "biome", "git-commit"}
	for _, skill := range reactSkills {
		if !strings.Contains(out, skill) {
			t.Errorf("expected skill %q for react type", skill)
		}
	}

	notExpected := []string{"angular", "dotnet", "devops", "yz-ui"}
	for _, skill := range notExpected {
		if strings.Contains(out, skill) {
			t.Errorf("did NOT expect skill %q for react type", skill)
		}
	}
}

func TestSkillsFilteredGeneric(t *testing.T) {
	bin := buildBinary(t)
	out := runYwai(t, bin, "skills", "--type", "generic")
	if !strings.Contains(out, "all skills") {
		t.Errorf("expected 'all skills' for generic, got: %s", out)
	}
}
