package globalagents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBundleResolution(t *testing.T) {
	tmp := t.TempDir()
	bundlesPath := filepath.Join(tmp, "bundles.json")
	writeFile(t, bundlesPath, `{
  "defaults": {
    "devops": ["devops"],
    "sdd-orchestrator": ["sdd-init", "sdd-apply"]
  },
  "by_project_type": {
    "nest": {
      "devops": ["devops", "biome"]
    }
  }
}`)

	cfg, err := LoadBundleConfig(bundlesPath)
	if err != nil {
		t.Fatal(err)
	}

	if got := cfg.Bundle("nest", "devops"); !equal(got, []string{"devops", "biome"}) {
		t.Errorf("nest override failed: %v", got)
	}
	if got := cfg.Bundle("dotnet", "devops"); !equal(got, []string{"devops"}) {
		t.Errorf("dotnet fallback failed: %v", got)
	}
	if got := cfg.Bundle("nest", "sdd-orchestrator"); !equal(got, []string{"sdd-init", "sdd-apply"}) {
		t.Errorf("sdd fallback to defaults failed: %v", got)
	}
	if got := cfg.Bundle("nest", "ghost"); got != nil {
		t.Errorf("unknown agent should return nil: %v", got)
	}
}

func TestAutoInvokePatterns(t *testing.T) {
	skillsDir := t.TempDir()

	writeFile(t, filepath.Join(skillsDir, "react-19", "SKILL.md"), `---
name: react-19
metadata:
  auto_invoke:
    - Writing React code
    - Components
    - Hooks
---

# React 19 skill
`)

	writeFile(t, filepath.Join(skillsDir, "biome", "SKILL.md"), `---
name: biome
metadata:
  auto_invoke: ["lint", "format", "code quality"]
---
`)

	writeFile(t, filepath.Join(skillsDir, "empty", "SKILL.md"), `---
name: empty
---
`)

	cases := []struct {
		skill string
		want  []string
	}{
		{"react-19", []string{"Writing React code", "Components", "Hooks"}},
		{"biome", []string{"lint", "format", "code quality"}},
		{"empty", nil},
		{"missing", nil},
	}
	for _, tc := range cases {
		got := AutoInvokePatterns(skillsDir, tc.skill)
		if !equal(got, tc.want) {
			t.Errorf("AutoInvokePatterns(%q)=%v, want %v", tc.skill, got, tc.want)
		}
	}
}

func TestRenderIncludesSkillsSections(t *testing.T) {
	content := Render(RenderInput{
		AgentName:   "devops",
		ProjectType: "nest",
		Target:      TargetOpenCode,
		Template:    "---\nmode: subagent\n---\n\n## Role\nDevOps role body\n",
		Bundle:      []string{"devops", "sdd-apply"},
		SkillsTriggers: map[string]string{
			"devops":    "pipeline | helm | docker",
			"sdd-apply": "apply tasks",
		},
	})

	got := string(content)

	for _, substr := range []string{
		"description: devops global agent for nest projects",
		"# devops",
		"Project type scope: nest",
		"## Base directives (from extensions)",
		"DevOps role body",
		"## Skills bundle (global)",
		"- `devops`",
		"- `sdd-apply`",
		"## Skills invoke",
		"Use `devops` when tasks match: pipeline | helm | docker.",
		"## SDD quick commands",
		"/sdd:apply",
		"## DevOps trigger keywords",
		"kubernetes",
	} {
		if !strings.Contains(got, substr) {
			t.Errorf("Render output missing %q\n---\n%s", substr, got)
		}
	}

	if strings.Contains(got, "mode: subagent") {
		t.Errorf("template frontmatter leaked into body")
	}
}

func TestRenderCopilotPromptFrontmatter(t *testing.T) {
	content := string(Render(RenderInput{
		AgentName:   "sdd-orchestrator",
		ProjectType: "generic",
		Target:      TargetCopilotPrompt,
		Template:    "",
		Bundle:      []string{"sdd-init"},
		SkillsTriggers: map[string]string{
			"sdd-init": "bootstrap",
		},
	}))

	if !strings.HasPrefix(content, "---\nname: sdd-orchestrator\n") {
		t.Errorf("Copilot prompt frontmatter missing name: %q", content[:80])
	}
	if !strings.Contains(content, "applyTo: \"**\"") {
		t.Errorf("Copilot prompt missing applyTo")
	}
}

func TestInstallAllPreservesUserFiles(t *testing.T) {
	repoRoot := t.TempDir()
	home := t.TempDir()

	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))

	extDir := filepath.Join(repoRoot, "extensions", "install-steps", "global-agents")
	tmplDir := filepath.Join(extDir, "templates")
	writeFile(t, filepath.Join(tmplDir, "sdd-orchestrator.md"), "## Role\nOrchestrator\n")
	writeFile(t, filepath.Join(tmplDir, "devops.md"), "## Role\nDevOps\n")
	writeFile(t, filepath.Join(extDir, "bundles.json"), `{"defaults":{"sdd-orchestrator":["sdd-init"],"devops":["devops"]},"by_project_type":{}}`)

	skillsDir := filepath.Join(repoRoot, "skills")
	writeFile(t, filepath.Join(skillsDir, "sdd-init", "SKILL.md"), "---\nauto_invoke:\n  - bootstrap\n---\n")
	writeFile(t, filepath.Join(skillsDir, "devops", "SKILL.md"), "---\nauto_invoke: [\"pipeline\"]\n---\n")

	// Pre-populate OpenCode dir with a user-owned agent that must be preserved.
	opencodeDir := filepath.Join(home, ".config", "opencode", "agent")
	if err := os.MkdirAll(opencodeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	userAgent := filepath.Join(opencodeDir, "my-custom.md")
	writeFile(t, userAgent, "user content")

	gen := Generator{
		ExtensionDir: extDir,
		SkillsDir:    skillsDir,
		ProjectType:  "generic",
	}

	written, err := gen.InstallAll()
	if err != nil {
		t.Fatal(err)
	}
	if written == 0 {
		t.Fatal("no files written")
	}

	// Managed files should exist.
	for _, name := range []string{"sdd-orchestrator.md", "devops.md"} {
		if _, err := os.Stat(filepath.Join(opencodeDir, name)); err != nil {
			t.Errorf("missing managed file %s: %v", name, err)
		}
	}

	// User custom file preserved.
	data, err := os.ReadFile(userAgent)
	if err != nil {
		t.Fatalf("user file was wiped: %v", err)
	}
	if string(data) != "user content" {
		t.Errorf("user file content changed")
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
