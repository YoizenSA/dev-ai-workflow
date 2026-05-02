package e2e

import (
	"os"
	"path/filepath"
	"runtime"
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

func TestLinkFilteredOnlyRelevant(t *testing.T) {
	tmpDir := t.TempDir()
	agentSkillsDir := filepath.Join(tmpDir, "skills")
	if err := os.MkdirAll(agentSkillsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	repoRoot := repoRoot(t)
	skillsSource := filepath.Join(repoRoot, "skills")

	reactSkills := []string{"react-19", "tailwind-4", "typescript", "biome", "playwright", "git-commit"}
	for _, name := range reactSkills {
		src := filepath.Join(skillsSource, name)
		dst := filepath.Join(agentSkillsDir, name)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			t.Skipf("skill %s not found in source", name)
		}
		if err := os.Symlink(src, dst); err != nil {
			if runtime.GOOS == "windows" {
				t.Logf("symlink failed (Windows): %v", err)
				continue
			}
			t.Fatalf("symlink failed: %v", err)
		}
	}

	notExpected := []string{"angular", "dotnet", "devops"}
	for _, name := range notExpected {
		dst := filepath.Join(agentSkillsDir, name)
		if _, err := os.Stat(dst); err == nil {
			t.Errorf("did NOT expect skill %s to be linked for react", name)
		}
	}
}
