package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFrontmatterDescription(t *testing.T) {
	tests := []struct {
		name   string
		prompt string
		want   string
	}{
		{
			name:   "inline value",
			prompt: "---\nname: dev\ndescription: A developer agent\nrole: developer\n---\n\n# Dev",
			want:   "A developer agent",
		},
		{
			name:   "folded block scalar",
			prompt: "---\nname: ask\ndescription: >\n  Research and Q&A agent.\n  Trigger: questions.\nrole: ask\n---\n\n# Ask",
			want:   "Research and Q&A agent. Trigger: questions.",
		},
		{
			name:   "literal block scalar",
			prompt: "---\nname: qa\ndescription: |\n  Line one.\n  Line two.\ntools: [Read]\n---\n\nbody",
			want:   "Line one. Line two.",
		},
		{
			name:   "no frontmatter",
			prompt: "# Just a heading\n\nbody",
			want:   "",
		},
		{
			name:   "no description key",
			prompt: "---\nname: x\nrole: y\n---\n\nbody",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := frontmatterDescription(tt.prompt); got != tt.want {
				t.Errorf("frontmatterDescription() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractDescriptionPrefersFrontmatter(t *testing.T) {
	prompt := "---\nname: ask\ndescription: >\n  The real description.\nrole: ask\n---\n\n# Ask Agent\n\nYou are a specialist body line."
	if got := extractDescription(prompt); got != "The real description." {
		t.Errorf("extractDescription() = %q, want frontmatter description", got)
	}
}

func TestExtractDescriptionFallsBackToBody(t *testing.T) {
	prompt := "---\nname: x\nrole: y\n---\n\n# Heading\n\nThe first body line."
	if got := extractDescription(prompt); got != "The first body line." {
		t.Errorf("extractDescription() = %q, want first body line", got)
	}
}

func TestParseOpenCodeTools(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tools.json")
	content := `{"allowed": ["Read", "Glob", "Grep", "ASTGrep"], "denied": ["Edit", "Write", "Bash"]}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	tools, err := parseOpenCodeTools(path)
	if err != nil {
		t.Fatalf("parseOpenCodeTools() error = %v", err)
	}

	wantTrue := []string{"read", "glob", "grep", "ast_grep"}
	for _, k := range wantTrue {
		if !tools[k] {
			t.Errorf("expected tool %q enabled", k)
		}
	}
	wantFalse := []string{"edit", "write", "bash"}
	for _, k := range wantFalse {
		if tools[k] {
			t.Errorf("expected tool %q disabled", k)
		}
	}
}

func TestClaudeToolsString(t *testing.T) {
	tools := map[string]bool{"read": true, "edit": true, "write": true, "bash": true, "glob": true, "grep": true}
	got := claudeToolsString(tools)
	want := "Read, Edit, Write, Bash, Glob, Grep"
	if got != want {
		t.Errorf("claudeToolsString() = %q, want %q", got, want)
	}

	// Read-only set stays ordered and excludes disabled tools.
	ro := map[string]bool{"read": true, "glob": true, "grep": true, "edit": false}
	if got := claudeToolsString(ro); got != "Read, Glob, Grep" {
		t.Errorf("claudeToolsString(read-only) = %q, want %q", got, "Read, Glob, Grep")
	}

	// Empty falls back to a safe default.
	if got := claudeToolsString(map[string]bool{}); got != "Read, Glob, Grep" {
		t.Errorf("claudeToolsString(empty) = %q, want default", got)
	}
}

func TestPromptWithSkills(t *testing.T) {
	base := "# Agent\n\nbody\n"
	if got := promptWithSkills(base, nil); got != base {
		t.Errorf("promptWithSkills(empty) should be a no-op, got %q", got)
	}

	got := promptWithSkills(base, []string{"typescript", "react-19"})
	if !strings.Contains(got, "## Preferred ywai Skills") {
		t.Error("expected skills section header")
	}
	if !strings.Contains(got, "`typescript`") || !strings.Contains(got, "`react-19`") {
		t.Error("expected each skill listed in backticks")
	}
}

func TestLoadProfiles(t *testing.T) {
	src := t.TempDir()
	devDir := filepath.Join(src, "dev")
	if err := os.MkdirAll(devDir, 0o755); err != nil {
		t.Fatal(err)
	}

	agentMD := "---\nname: dev\ndescription: >\n  Developer agent.\n  Trigger: code.\nrole: developer\n---\n\n# Dev Agent\n\nBody."
	if err := os.WriteFile(filepath.Join(devDir, "AGENT.md"), []byte(agentMD), 0o644); err != nil {
		t.Fatal(err)
	}
	toolsJSON := `{"allowed": ["Read", "Edit", "Write", "Bash"], "denied": []}`
	if err := os.WriteFile(filepath.Join(devDir, "tools.json"), []byte(toolsJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(devDir, "skills.txt"), []byte("typescript\n# comment\nreact-19\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A non-directory entry should be ignored.
	if err := os.WriteFile(filepath.Join(src, "README.md"), []byte("ignore me"), 0o644); err != nil {
		t.Fatal(err)
	}

	profiles, err := LoadProfiles(src)
	if err != nil {
		t.Fatalf("LoadProfiles() error = %v", err)
	}

	dev, ok := profiles["dev"]
	if !ok {
		t.Fatal("expected dev profile")
	}
	if dev.Description != "Developer agent. Trigger: code." {
		t.Errorf("description = %q", dev.Description)
	}
	if !dev.Tools["read"] || !dev.Tools["edit"] {
		t.Error("expected read/edit tools enabled")
	}
	if len(dev.Skills) != 2 || dev.Skills[0] != "typescript" || dev.Skills[1] != "react-19" {
		t.Errorf("skills = %v, want [typescript react-19] (comment skipped)", dev.Skills)
	}
	if !strings.Contains(dev.Prompt, "## Preferred ywai Skills") {
		t.Error("expected skills section appended to prompt")
	}
}

func TestInstallOpenCode(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	// Pre-existing agent must not be overwritten.
	initial := `{"agent": {"existing": {"mode": "primary", "description": "keep me"}}}`
	if err := os.WriteFile(configPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	profiles := map[string]AgentProfile{
		"dev": {
			Name:        "dev",
			Description: "Developer agent",
			Prompt:      "# Dev",
			Tools:       map[string]bool{"read": true, "edit": true},
		},
		"existing": {
			Name:        "existing",
			Description: "should be skipped",
			Prompt:      "# nope",
			Tools:       map[string]bool{"read": true},
		},
	}

	if err := InstallOpenCode(configPath, profiles); err != nil {
		t.Fatalf("InstallOpenCode() error = %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	agents := root["agent"].(map[string]any)

	if _, ok := agents["dev"]; !ok {
		t.Error("expected dev agent injected")
	}
	existing := agents["existing"].(map[string]any)
	if existing["description"] != "keep me" {
		t.Errorf("existing agent was overwritten: %v", existing["description"])
	}
}
