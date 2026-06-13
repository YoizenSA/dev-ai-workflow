package e2e

import (
	"strings"
	"testing"
)

func TestSkillsList(t *testing.T) {
	bin := buildBinary(t)
	out := runYwai(t, bin, "skills")

	expected := []string{
		"angular", "devops", "docker", "git-commit",
		"tailwind-4", "yz-ui",
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
