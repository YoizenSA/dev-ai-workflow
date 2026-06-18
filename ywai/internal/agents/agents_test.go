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

func TestParsePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "permissions.json")
	content := `{"read": "allow", "glob": "allow", "grep": "allow", "ast_grep": "allow", "edit": "deny", "write": "deny", "bash": "deny"}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	perms, err := parsePermissions(path)
	if err != nil {
		t.Fatalf("parsePermissions() error = %v", err)
	}

	wantAllow := []string{"read", "glob", "grep", "ast_grep"}
	for _, k := range wantAllow {
		if perms[k] != "allow" {
			t.Errorf("expected permission %q = allow, got %q", k, perms[k])
		}
	}
	wantDeny := []string{"edit", "write", "bash"}
	for _, k := range wantDeny {
		if perms[k] != "deny" {
			t.Errorf("expected permission %q = deny, got %q", k, perms[k])
		}
	}
}

func TestClaudeToolsString(t *testing.T) {
	perms := map[string]string{"read": "allow", "edit": "allow", "write": "allow", "bash": "allow", "glob": "allow", "grep": "allow"}
	got := claudeToolsString(perms)
	want := "Read, Edit, Write, Bash, Glob, Grep"
	if got != want {
		t.Errorf("claudeToolsString() = %q, want %q", got, want)
	}

	// Read-only set stays ordered and excludes denied tools.
	ro := map[string]string{"read": "allow", "glob": "allow", "grep": "allow", "edit": "deny"}
	if got := claudeToolsString(ro); got != "Read, Glob, Grep" {
		t.Errorf("claudeToolsString(read-only) = %q, want %q", got, "Read, Glob, Grep")
	}

	// Empty falls back to a safe default.
	if got := claudeToolsString(map[string]string{}); got != "Read, Glob, Grep" {
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

func TestExtractRole(t *testing.T) {
	tests := []struct {
		name   string
		prompt string
		want   string
	}{
		{"developer", "---\nrole: developer\n---\n\nbody", "developer"},
		{"orchestrator", "---\nrole: orchestrator\n---\n\nbody", "orchestrator"},
		{"explorer", "---\nname: finder\nrole: explorer\n---\n\nbody", "explorer"},
		{"no role", "---\nname: dev\n---\n\nbody", ""},
		{"no frontmatter", "# Agent\n\nbody", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractRole(tt.prompt); got != tt.want {
				t.Errorf("extractRole() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractSections(t *testing.T) {
	tests := []struct {
		name   string
		prompt string
		want   []string
	}{
		{"handoff only", "---\nsections: [handoff]\n---\n\nbody", []string{"handoff"}},
		{"multiple", "---\nsections: [handoff, kanban]\n---\n\nbody", []string{"handoff", "kanban"}},
		{"no sections", "---\nname: dev\n---\n\nbody", nil},
		{"no frontmatter", "# Agent\n\nbody", nil},
		{"empty array", "---\nsections: []\n---\n\nbody", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSections(tt.prompt)
			if len(got) != len(tt.want) {
				t.Errorf("extractSections() = %v, want %v", got, tt.want)
				return
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("extractSections()[%d] = %q, want %q", i, v, tt.want[i])
				}
			}
		})
	}
}

func TestAppendSections(t *testing.T) {
	// Create a temp dir with sections.
	dir := t.TempDir()
	sectionsDir := filepath.Join(dir, "sections")
	if err := os.MkdirAll(sectionsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sectionsDir, "handoff.md"), []byte("## Handoff\n\nReport back."), 0o644); err != nil {
		t.Fatal(err)
	}

	base := "# Agent\n\nbody"

	// No sections -> no-op.
	if got := appendSections(base, nil, dir); got != base {
		t.Errorf("appendSections(nil) should be a no-op")
	}

	// Existing section -> appended.
	got := appendSections(base, []string{"handoff"}, dir)
	if !strings.Contains(got, "## Handoff") {
		t.Error("expected handoff section appended")
	}
	if !strings.Contains(got, "Report back.") {
		t.Error("expected handoff content")
	}

	// Missing section -> silently skipped.
	got = appendSections(base, []string{"nonexistent"}, dir)
	if got != base {
		t.Errorf("appendSections(missing) should be a no-op, got %q", got)
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
	permsJSON := `{"read": "allow", "edit": "allow", "write": "allow", "bash": "allow"}`
	if err := os.WriteFile(filepath.Join(devDir, "permissions.json"), []byte(permsJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(devDir, "skills.txt"), []byte("typescript\n# comment\nreact-19\n"), 0o644); err != nil {
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
	if dev.Permission["read"] != "allow" || dev.Permission["edit"] != "allow" {
		t.Error("expected read/edit permissions allowed")
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
			Permission:  map[string]string{"read": "allow", "edit": "allow"},
		},
		"existing": {
			Name:        "existing",
			Description: "should be skipped",
			Prompt:      "# nope",
			Permission:  map[string]string{"read": "allow"},
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

func TestInstallOpenCodeCreatesMissingFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	// File does not exist yet.

	profiles := map[string]AgentProfile{
		"ask": {
			Name:        "ask",
			Description: "Research agent",
			Prompt:      "# Ask\n\nClean body.",
			Permission:  map[string]string{"read": "allow"},
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
	ask := agents["ask"].(map[string]any)
	if ask["description"] != "Research agent" {
		t.Errorf("description = %v", ask["description"])
	}
}

func TestInstallOpenCodeMigratesFrontmatter(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	// Simulate an old buggy agent whose prompt contains leaked frontmatter.
	initial := `{"agent": {"ask": {"mode": "all", "description": "old", "prompt": "---\nname: ask\ndescription: old\n---\n\n# Ask\nBody."}}}`
	if err := os.WriteFile(configPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	profiles := map[string]AgentProfile{
		"ask": {
			Name:        "ask",
			Description: "Research agent",
			Prompt:      "# Ask\n\nClean body.",
			Permission:  map[string]string{"read": "allow"},
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
	ask := agents["ask"].(map[string]any)

	prompt := ask["prompt"].(string)
	if strings.HasPrefix(prompt, "---") {
		t.Errorf("migrated prompt should not start with frontmatter, got: %s", prompt)
	}
	if !strings.Contains(prompt, "Clean body") {
		t.Errorf("migrated prompt should contain new body, got: %s", prompt)
	}
	if ask["description"] != "Research agent" {
		t.Errorf("description should be updated, got: %v", ask["description"])
	}
}

func TestStripFrontmatter(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "basic frontmatter",
			content: "---\nname: dev\n---\n\n# Hello",
			want:    "# Hello",
		},
		{
			name:    "no frontmatter",
			content: "# Hello\n\nbody",
			want:    "# Hello\n\nbody",
		},
		{
			name:    "frontmatter with separator in body",
			content: "---\nname: dev\n---\n\n# Hello\n\n---\n\nMore body",
			want:    "# Hello\n\n---\n\nMore body",
		},
		{
			name:    "unclosed frontmatter",
			content: "---\nname: dev\n# Hello",
			want:    "---\nname: dev\n# Hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripFrontmatter(tt.content)
			if got != tt.want {
				t.Errorf("stripFrontmatter() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadProfilesStripsFrontmatter(t *testing.T) {
	src := t.TempDir()
	devDir := filepath.Join(src, "dev")
	if err := os.MkdirAll(devDir, 0o755); err != nil {
		t.Fatal(err)
	}

	agentMD := "---\nname: dev\ndescription: >\n  Developer agent.\n  Trigger: code.\nrole: developer\nmode: all\ntools: [Read, Edit]\n---\n\n# Dev Agent\n\nBody."
	if err := os.WriteFile(filepath.Join(devDir, "AGENT.md"), []byte(agentMD), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(devDir, "permissions.json"), []byte(`{"read": "allow", "edit": "allow"}`), 0o644); err != nil {
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
	if strings.Contains(dev.Prompt, "---") {
		t.Errorf("Prompt should not contain frontmatter delimiters, got:\n%s", dev.Prompt)
	}
	if strings.HasPrefix(dev.Prompt, "name:") {
		t.Errorf("Prompt should not start with frontmatter keys, got:\n%s", dev.Prompt)
	}
	if !strings.HasPrefix(dev.Prompt, "# Dev Agent") {
		t.Errorf("Prompt should start with body heading, got:\n%s", dev.Prompt)
	}
}

func TestBuildOpenCodeMarkdown(t *testing.T) {
	profile := AgentProfile{
		Name:        "dev",
		Description: "Developer agent",
		Prompt:      "# Dev Agent\n\nYou are a developer.",
		Permission:  map[string]string{"read": "allow", "edit": "allow", "write": "allow", "bash": "allow"},
		Mode:        "subagent",
	}

	markdown := buildOpenCodeMarkdown("dev", profile)

	if !strings.Contains(markdown, "---") {
		t.Error("markdown should start with frontmatter delimiter")
	}
	if !strings.Contains(markdown, "description: Developer agent") {
		t.Error("markdown should contain description")
	}
	if !strings.Contains(markdown, "mode: subagent") {
		t.Error("markdown should contain mode")
	}
	if !strings.Contains(markdown, "temperature: 0.1") {
		t.Error("markdown should contain temperature")
	}
	if !strings.Contains(markdown, "permission:") {
		t.Error("markdown should contain permission section")
	}
	if !strings.Contains(markdown, "read: allow") {
		t.Error("markdown should contain read permission")
	}
	if !strings.Contains(markdown, "# Dev Agent") {
		t.Error("markdown should contain prompt body")
	}
}

func TestInstallOpenCodeMarkdown(t *testing.T) {
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, "agents")

	profiles := map[string]AgentProfile{
		"dev": {
			Name:        "dev",
			Description: "Developer agent",
			Prompt:      "# Dev\n\nBody.",
			Permission:  map[string]string{"read": "allow", "edit": "allow"},
			Mode:        "subagent",
		},
	}

	if err := InstallOpenCodeMarkdown(agentsDir, profiles, false); err != nil {
		t.Fatalf("InstallOpenCodeMarkdown() error = %v", err)
	}

	// Check file was created
	devPath := filepath.Join(agentsDir, "dev.md")
	data, err := os.ReadFile(devPath)
	if err != nil {
		t.Fatalf("failed to read dev.md: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "description: Developer agent") {
		t.Error("markdown should contain description")
	}
	if !strings.Contains(content, "# Dev") {
		t.Error("markdown should contain prompt")
	}
}

func TestInstallOpenCodeMarkdownSkipsExisting(t *testing.T) {
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, "agents")

	// Pre-existing file
	existingPath := filepath.Join(agentsDir, "dev.md")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existingPath, []byte("# Existing\n\nKeep me"), 0o644); err != nil {
		t.Fatal(err)
	}

	profiles := map[string]AgentProfile{
		"dev": {
			Name:        "dev",
			Description: "New description",
			Prompt:      "# New\n\nShould not overwrite",
			Permission:  map[string]string{"read": "allow"},
			Mode:        "subagent",
		},
	}

	if err := InstallOpenCodeMarkdown(agentsDir, profiles, false); err != nil {
		t.Fatalf("InstallOpenCodeMarkdown() error = %v", err)
	}

	// Check file was NOT overwritten
	data, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "Keep me") {
		t.Error("existing file should not be overwritten")
	}
	if strings.Contains(content, "New description") {
		t.Error("existing file should not contain new content")
	}
}

func TestInstallOpenCodeMarkdownOverwriteExisting(t *testing.T) {
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, "agents")

	// Create an existing dev.md
	existingPath := filepath.Join(agentsDir, "dev.md")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existingPath, []byte("---\ndescription: Keep me\n---\n\n# Keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	profiles := map[string]AgentProfile{
		"dev": {
			Name:        "dev",
			Description: "New description",
			Prompt:      "# New\n\nShould overwrite",
			Permission:  map[string]string{"read": "allow"},
			Mode:        "subagent",
		},
	}

	if err := InstallOpenCodeMarkdown(agentsDir, profiles, true); err != nil {
		t.Fatalf("InstallOpenCodeMarkdown() error = %v", err)
	}

	data, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if strings.Contains(content, "Keep me") {
		t.Error("existing file should be overwritten")
	}
	if !strings.Contains(content, "New description") {
		t.Error("overwritten file should contain new content")
	}
	if !strings.Contains(content, "Should overwrite") {
		t.Error("overwritten file should contain new prompt")
	}
}

func TestMigrateOpenCodeAgents(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	agentsDir := filepath.Join(dir, "agents")

	// Pre-existing JSON with agents
	initial := `{"agent": {"dev": {"mode": "subagent", "description": "Dev agent", "prompt": "# Dev\n\nBody.", "tools": {"read": true, "edit": true}}}}`
	if err := os.WriteFile(configPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := MigrateOpenCodeAgents(configPath, agentsDir); err != nil {
		t.Fatalf("MigrateOpenCodeAgents() error = %v", err)
	}

	// Check markdown file was created
	devPath := filepath.Join(agentsDir, "dev.md")
	data, err := os.ReadFile(devPath)
	if err != nil {
		t.Fatalf("failed to read dev.md: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "description: Dev agent") {
		t.Error("migrated markdown should contain description")
	}

	// Check JSON was cleaned
	configData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(configData, &root); err != nil {
		t.Fatal(err)
	}
	if _, ok := root["agent"]; ok {
		t.Error("agent section should be removed from JSON after migration")
	}
}

func TestMigrateOpenCodeAgentsSkipsExisting(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	agentsDir := filepath.Join(dir, "agents")

	// Pre-existing markdown file
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existingPath := filepath.Join(agentsDir, "dev.md")
	if err := os.WriteFile(existingPath, []byte("# Existing\n\nKeep me"), 0o644); err != nil {
		t.Fatal(err)
	}

	// JSON with same agent
	initial := `{"agent": {"dev": {"mode": "subagent", "description": "Dev agent", "prompt": "# Dev\n\nBody.", "tools": {"read": true}}}}`
	if err := os.WriteFile(configPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := MigrateOpenCodeAgents(configPath, agentsDir); err != nil {
		t.Fatalf("MigrateOpenCodeAgents() error = %v", err)
	}

	// Check existing markdown was NOT overwritten
	data, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "Keep me") {
		t.Error("existing markdown should not be overwritten")
	}

	// Check JSON was still cleaned
	configData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(configData, &root); err != nil {
		t.Fatal(err)
	}
	if _, ok := root["agent"]; ok {
		t.Error("agent section should be removed from JSON even if markdown exists")
	}
}

func TestMapToAgentProfile(t *testing.T) {
	m := map[string]any{
		"prompt":      "# Test\n\nBody",
		"description": "Test agent",
		"mode":        "primary",
		"permission":  map[string]any{"read": "allow", "edit": "deny", "bash": "allow"},
	}

	profile := mapToAgentProfile("test", m)

	if profile.Name != "test" {
		t.Errorf("Name = %q, want test", profile.Name)
	}
	if profile.Description != "Test agent" {
		t.Errorf("Description = %q, want Test agent", profile.Description)
	}
	if profile.Mode != "primary" {
		t.Errorf("Mode = %q, want primary", profile.Mode)
	}
	if profile.Prompt != "# Test\n\nBody" {
		t.Errorf("Prompt = %q", profile.Prompt)
	}
	if profile.Permission["read"] != "allow" {
		t.Error("read permission should be allow")
	}
	if profile.Permission["edit"] != "deny" {
		t.Error("edit permission should be deny")
	}
	if profile.Permission["bash"] != "allow" {
		t.Error("bash permission should be allow")
	}
}

// ─── Agent Group tests ──────────────────────────────────────────────────────

// writeGroupsJSON is a test helper that writes a groups.json file to dir.
func writeGroupsJSON(t *testing.T, dir, content string) {
	t.Helper()
	path := filepath.Join(dir, "groups.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write groups.json: %v", err)
	}
}

// writeLocalGroupsJSON is a test helper that writes a groups.local.json file to dir.
func writeLocalGroupsJSON(t *testing.T, dir, content string) {
	t.Helper()
	path := filepath.Join(dir, "groups.local.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write groups.local.json: %v", err)
	}
}

// writeAgentDir creates a minimal agent directory that LoadProfiles will recognise.
func writeAgentDir(t *testing.T, parentDir, name string) {
	t.Helper()
	agentDir := filepath.Join(parentDir, name)
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("failed to create agent dir %s: %v", agentDir, err)
	}
	agendPath := filepath.Join(agentDir, "AGENT.md")
	content := "---\nname: " + name + "\ndescription: " + name + " agent\ntools: [Read]\n---\n\nBody"
	if err := os.WriteFile(agendPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write AGENT.md for %s: %v", name, err)
	}
	permPath := filepath.Join(agentDir, "permissions.json")
	permContent := `{"read": true, "edit": false}`
	if err := os.WriteFile(permPath, []byte(permContent), 0o644); err != nil {
		t.Fatalf("failed to write permissions.json for %s: %v", name, err)
	}
}

func TestLoadGroupManifest_Valid(t *testing.T) {
	dir := t.TempDir()
	writeGroupsJSON(t, dir, `{
		"groups": {
			"core": {
				"description": "Core development agents",
				"agents": ["orchestrator", "dev", "qa"]
			},
			"social-refactor": {
				"description": "Social refactoring agents",
				"agents": ["reviewer", "architect"]
			}
		}
	}`)

	manifest, err := LoadGroupManifest(dir)
	if err != nil {
		t.Fatalf("LoadGroupManifest() unexpected error: %v", err)
	}
	if manifest == nil {
		t.Fatal("LoadGroupManifest() returned nil")
	}

	// Check groups exist
	core, ok := manifest.Groups["core"]
	if !ok {
		t.Fatal("expected 'core' group")
	}
	if core.Description != "Core development agents" {
		t.Errorf("core description = %q, want %q", core.Description, "Core development agents")
	}
	if len(core.Agents) != 3 || core.Agents[0] != "orchestrator" {
		t.Errorf("core agents = %v, want [orchestrator dev qa]", core.Agents)
	}

	social, ok := manifest.Groups["social-refactor"]
	if !ok {
		t.Fatal("expected 'social-refactor' group")
	}
	if social.Description != "Social refactoring agents" {
		t.Errorf("social-refactor description = %q, want %q", social.Description, "Social refactoring agents")
	}
	if len(social.Agents) != 2 || social.Agents[0] != "reviewer" {
		t.Errorf("social-refactor agents = %v, want [reviewer architect]", social.Agents)
	}
}

func TestLoadGroupManifest_MissingFile(t *testing.T) {
	dir := t.TempDir()
	// No groups.json written

	_, err := LoadGroupManifest(dir)
	if err == nil {
		t.Fatal("LoadGroupManifest() expected error for missing groups.json, got nil")
	}
	if !strings.Contains(err.Error(), "groups.json") {
		t.Errorf("error should mention groups.json, got: %v", err)
	}
}

func TestLoadGroupManifest_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	writeGroupsJSON(t, dir, `{invalid json here}`)

	_, err := LoadGroupManifest(dir)
	if err == nil {
		t.Fatal("LoadGroupManifest() expected error for invalid JSON, got nil")
	}
	if !strings.Contains(err.Error(), "invalid groups.json") {
		t.Errorf("error should mention invalid groups.json, got: %v", err)
	}
}

func TestLoadGroupManifest_LocalOverride(t *testing.T) {
	dir := t.TempDir()
	// Base groups.json
	writeGroupsJSON(t, dir, `{
		"groups": {
			"core": {
				"description": "Core agents",
				"agents": ["dev"]
			}
		}
	}`)
	// Local override adds a group
	writeLocalGroupsJSON(t, dir, `{
		"groups": {
			"custom": {
				"description": "Custom agents",
				"agents": ["qa", "reviewer"]
			}
		}
	}`)

	manifest, err := LoadGroupManifest(dir)
	if err != nil {
		t.Fatalf("LoadGroupManifest() unexpected error: %v", err)
	}

	// Both groups should exist
	if _, ok := manifest.Groups["core"]; !ok {
		t.Error("expected 'core' group after merge")
	}
	custom, ok := manifest.Groups["custom"]
	if !ok {
		t.Fatal("expected 'custom' group from local override")
	}
	if custom.Description != "Custom agents" {
		t.Errorf("custom description = %q, want %q", custom.Description, "Custom agents")
	}
}

func TestLoadGroupManifest_LocalOverride_MergesWithBase(t *testing.T) {
	dir := t.TempDir()
	writeGroupsJSON(t, dir, `{
		"groups": {
			"core": {
				"description": "Core agents",
				"agents": ["dev"]
			},
			"extra": {
				"description": "Extra agents",
				"agents": ["reviewer"]
			}
		}
	}`)
	// Local override overrides "extra" description
	writeLocalGroupsJSON(t, dir, `{
		"groups": {
			"extra": {
				"description": "Overridden extra agents",
				"agents": ["reviewer", "architect"]
			}
		}
	}`)

	manifest, err := LoadGroupManifest(dir)
	if err != nil {
		t.Fatalf("LoadGroupManifest() unexpected error: %v", err)
	}

	// core should still be present
	if _, ok := manifest.Groups["core"]; !ok {
		t.Error("expected 'core' group to survive local override")
	}
	// extra should have overridden description
	extra, ok := manifest.Groups["extra"]
	if !ok {
		t.Fatal("expected 'extra' group")
	}
	if extra.Description != "Overridden extra agents" {
		t.Errorf("extra description = %q, want %q", extra.Description, "Overridden extra agents")
	}
	if len(extra.Agents) != 2 {
		t.Errorf("extra agents = %v, want [reviewer architect]", extra.Agents)
	}
}

func TestLoadProfilesByGroup_AllGroups(t *testing.T) {
	dir := t.TempDir()
	// Create a few agent dirs
	writeAgentDir(t, dir, "orchestrator")
	writeAgentDir(t, dir, "dev")
	writeAgentDir(t, dir, "qa")

	// No groups.json — AllGroups=true should skip groups and load all
	filter := GroupFilter{AllGroups: true}
	profiles, err := LoadProfilesByGroup(dir, filter)
	if err != nil {
		t.Fatalf("LoadProfilesByGroup() unexpected error: %v", err)
	}
	if len(profiles) != 3 {
		t.Errorf("expected 3 profiles with AllGroups=true, got %d", len(profiles))
	}
	for _, name := range []string{"orchestrator", "dev", "qa"} {
		if _, ok := profiles[name]; !ok {
			t.Errorf("expected profile %q with AllGroups=true", name)
		}
	}
}

func TestLoadProfilesByGroup_CoreOnly(t *testing.T) {
	dir := t.TempDir()
	// Agent dirs — some in core, some not
	writeAgentDir(t, dir, "orchestrator")
	writeAgentDir(t, dir, "dev")
	writeAgentDir(t, dir, "qa")
	writeAgentDir(t, dir, "external-agent")

	// groups.json: only core agents
	writeGroupsJSON(t, dir, `{
		"groups": {
			"core": {
				"description": "Core agents",
				"agents": ["orchestrator", "dev", "qa"]
			}
		}
	}`)

	filter := GroupFilter{}
	profiles, err := LoadProfilesByGroup(dir, filter)
	if err != nil {
		t.Fatalf("LoadProfilesByGroup() unexpected error: %v", err)
	}

	// Core agents must be present
	for _, name := range []string{"orchestrator", "dev", "qa"} {
		if _, ok := profiles[name]; !ok {
			t.Errorf("expected core agent %q in profiles", name)
		}
	}
	// external-agent should NOT be present
	if _, ok := profiles["external-agent"]; ok {
		t.Error("external-agent should NOT be in core-only profiles")
	}
	if len(profiles) != 3 {
		t.Errorf("expected 3 core agents, got %d: %v", len(profiles), profiles)
	}
}

func TestLoadProfilesByGroup_WithAdditionalGroup(t *testing.T) {
	dir := t.TempDir()
	writeAgentDir(t, dir, "orchestrator")
	writeAgentDir(t, dir, "dev")
	writeAgentDir(t, dir, "reviewer")
	writeAgentDir(t, dir, "architect")

	writeGroupsJSON(t, dir, `{
		"groups": {
			"core": {
				"description": "Core agents",
				"agents": ["orchestrator", "dev"]
			},
			"social-refactor": {
				"description": "Social refactoring",
				"agents": ["reviewer", "architect"]
			}
		}
	}`)

	filter := GroupFilter{Groups: []string{"social-refactor"}}
	profiles, err := LoadProfilesByGroup(dir, filter)
	if err != nil {
		t.Fatalf("LoadProfilesByGroup() unexpected error: %v", err)
	}

	// Core + social-refactor agents must be present
	for _, name := range []string{"orchestrator", "dev", "reviewer", "architect"} {
		if _, ok := profiles[name]; !ok {
			t.Errorf("expected agent %q in profiles", name)
		}
	}
	if len(profiles) != 4 {
		t.Errorf("expected 4 agents, got %d: %v", len(profiles), profiles)
	}
}

func TestLoadProfilesByGroup_BrokenGroupsJSON_Fallback(t *testing.T) {
	dir := t.TempDir()
	writeAgentDir(t, dir, "orchestrator")
	writeAgentDir(t, dir, "dev")
	writeAgentDir(t, dir, "qa")

	// Invalid JSON = broken groups file
	writeGroupsJSON(t, dir, `{broken`)

	filter := GroupFilter{Groups: []string{"nonexistent"}}
	profiles, err := LoadProfilesByGroup(dir, filter)
	if err != nil {
		t.Fatalf("LoadProfilesByGroup() should fallback on broken JSON, got error: %v", err)
	}
	// Should fall back to all profiles
	if len(profiles) != 3 {
		t.Errorf("expected all 3 profiles in fallback, got %d", len(profiles))
	}
}

func TestLoadProfilesByGroup_MissingGroupsJSON_Fallback(t *testing.T) {
	dir := t.TempDir()
	writeAgentDir(t, dir, "orchestrator")
	writeAgentDir(t, dir, "dev")

	// No groups.json at all
	filter := GroupFilter{Groups: []string{"nonexistent"}}
	profiles, err := LoadProfilesByGroup(dir, filter)
	if err != nil {
		t.Fatalf("LoadProfilesByGroup() should fallback on missing groups.json, got error: %v", err)
	}
	if len(profiles) != 2 {
		t.Errorf("expected all 2 profiles in fallback, got %d", len(profiles))
	}
}

func TestLoadProfilesByGroup_UnknownGroup(t *testing.T) {
	dir := t.TempDir()
	writeAgentDir(t, dir, "orchestrator")
	writeAgentDir(t, dir, "dev")
	writeAgentDir(t, dir, "qa")

	writeGroupsJSON(t, dir, `{
		"groups": {
			"core": {
				"description": "Core agents",
				"agents": ["orchestrator", "dev", "qa"]
			}
		}
	}`)

	// Request a non-existent group — should just return core agents (no error)
	filter := GroupFilter{Groups: []string{"does-not-exist"}}
	profiles, err := LoadProfilesByGroup(dir, filter)
	if err != nil {
		t.Fatalf("LoadProfilesByGroup() unexpected error for unknown group: %v", err)
	}
	// Should still return core
	if len(profiles) != 3 {
		t.Errorf("expected 3 core agents with unknown group filter, got %d", len(profiles))
	}
}

func TestGroupFilter_ZeroValue(t *testing.T) {
	// Zero-value GroupFilter should have AllGroups=false and nil Groups
	var f GroupFilter
	if f.AllGroups {
		t.Error("zero-value GroupFilter should have AllGroups=false")
	}
	if f.Groups != nil {
		t.Error("zero-value GroupFilter should have nil Groups")
	}

	// A zero-value filter should only include core agents
	dir := t.TempDir()
	writeAgentDir(t, dir, "core-agent")
	writeAgentDir(t, dir, "non-core-agent")

	writeGroupsJSON(t, dir, `{
		"groups": {
			"core": {
				"description": "Core agents",
				"agents": ["core-agent"]
			}
		}
	}`)

	profiles, err := LoadProfilesByGroup(dir, f)
	if err != nil {
		t.Fatalf("LoadProfilesByGroup() unexpected error: %v", err)
	}
	if _, ok := profiles["core-agent"]; !ok {
		t.Error("expected core-agent with zero-value GroupFilter")
	}
	if _, ok := profiles["non-core-agent"]; ok {
		t.Error("non-core-agent should NOT be in zero-value GroupFilter profiles")
	}
}

func TestListGroups_Sorted(t *testing.T) {
	dir := t.TempDir()
	writeGroupsJSON(t, dir, `{
		"groups": {
			"social-refactor": {
				"description": "Social refactoring",
				"agents": []
			},
			"core": {
				"description": "Core agents",
				"agents": []
			},
			"alpha": {
				"description": "Alpha group",
				"agents": []
			}
		}
	}`)

	names, err := ListGroups(dir)
	if err != nil {
		t.Fatalf("ListGroups() unexpected error: %v", err)
	}

	// Must be sorted alphabetically: alpha, core, social-refactor
	expected := []string{"alpha", "core", "social-refactor"}
	if len(names) != len(expected) {
		t.Fatalf("ListGroups() = %v (len=%d), want %v (len=%d)", names, len(names), expected, len(expected))
	}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("ListGroups()[%d] = %q, want %q", i, name, expected[i])
		}
	}
}

func TestListGroups_Empty(t *testing.T) {
	dir := t.TempDir()
	writeGroupsJSON(t, dir, `{
		"groups": {}
	}`)

	names, err := ListGroups(dir)
	if err != nil {
		t.Fatalf("ListGroups() unexpected error: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("expected empty list for no groups, got %v", names)
	}
}

func TestListGroups_MissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := ListGroups(dir)
	if err == nil {
		t.Fatal("ListGroups() expected error for missing groups.json")
	}
}

func TestLoadProfilesByGroup_NestedAgents(t *testing.T) {
	// Reproduce bug: agents in subdirectories (like qa-automation/qa-orchestrator)
	// must match the names referenced in groups.json (e.g. "qa-automation/qa-orchestrator").
	dir := t.TempDir()

	// Create core agents (flat structure)
	writeAgentDir(t, dir, "orchestrator")
	writeAgentDir(t, dir, "dev")

	// Create nested agents (like qa-automation/)
	nestedDir := filepath.Join(dir, "qa-automation")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeAgentDir(t, nestedDir, "qa-orchestrator")
	writeAgentDir(t, nestedDir, "qa-analyst")

	writeGroupsJSON(t, dir, `{
		"groups": {
			"core": {
				"description": "Core agents",
				"agents": ["orchestrator", "dev"]
			},
			"qa-automation": {
				"description": "QA automation agents",
				"agents": ["qa-automation/qa-orchestrator", "qa-automation/qa-analyst"]
			}
		}
	}`)

	// Test 1: core only — nested agents should NOT be included
	coreProfiles, err := LoadProfilesByGroup(dir, GroupFilter{})
	if err != nil {
		t.Fatalf("core-only: unexpected error: %v", err)
	}
	if len(coreProfiles) != 2 {
		t.Errorf("core-only: expected 2 profiles, got %d: %v", len(coreProfiles), keys(coreProfiles))
	}
	for _, name := range []string{"orchestrator", "dev"} {
		if _, ok := coreProfiles[name]; !ok {
			t.Errorf("core-only: expected %q in profiles", name)
		}
	}

	// Test 2: core + qa-automation — nested agents MUST be included
	qaProfiles, err := LoadProfilesByGroup(dir, GroupFilter{Groups: []string{"qa-automation"}})
	if err != nil {
		t.Fatalf("with qa-automation: unexpected error: %v", err)
	}
	if len(qaProfiles) != 4 {
		t.Errorf("with qa-automation: expected 4 profiles, got %d: %v", len(qaProfiles), keys(qaProfiles))
	}
	expected := []string{"orchestrator", "dev", "qa-automation/qa-orchestrator", "qa-automation/qa-analyst"}
	for _, name := range expected {
		if _, ok := qaProfiles[name]; !ok {
			t.Errorf("with qa-automation: expected %q in profiles", name)
		}
	}
}

func keys[M ~map[string]V, V any](m M) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestPiToolsString(t *testing.T) {
	tests := []struct {
		name  string
		perms map[string]string
		want  string
	}{
		{
			name:  "all allowed",
			perms: map[string]string{"read": "allow", "edit": "allow", "write": "allow", "bash": "allow", "glob": "allow", "grep": "allow", "webfetch": "allow", "websearch": "allow"},
			want:  "read, edit, write, bash, glob, grep, webfetch, websearch",
		},
		{
			name:  "some allowed some denied",
			perms: map[string]string{"read": "allow", "edit": "deny", "write": "deny", "bash": "allow", "glob": "allow", "grep": "allow", "webfetch": "deny"},
			want:  "read, bash, glob, grep",
		},
		{
			name:  "ask included",
			perms: map[string]string{"read": "ask", "bash": "allow", "glob": "allow", "grep": "allow"},
			want:  "read, bash, glob, grep",
		},
		{
			name:  "read-only",
			perms: map[string]string{"read": "allow"},
			want:  "read",
		},
		{
			name:  "empty fallback",
			perms: map[string]string{},
			want:  "read, glob, grep",
		},
		{
			name:  "lowercase output",
			perms: map[string]string{"read": "allow", "glob": "allow", "grep": "allow"},
			want:  "read, glob, grep",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := piToolsString(tt.perms); got != tt.want {
				t.Errorf("piToolsString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInstallPi(t *testing.T) {
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, "pi", "agent", "agents")

	profiles := map[string]AgentProfile{
		"dev": {
			Name:        "dev",
			Description: "Developer agent",
			Prompt:      "# Dev\n\nBody.",
			Permission:  map[string]string{"read": "allow", "edit": "allow", "bash": "allow"},
			Mode:        "subagent",
		},
	}

	if err := InstallPi(agentsDir, profiles, false); err != nil {
		t.Fatalf("InstallPi() error = %v", err)
	}

	devPath := filepath.Join(agentsDir, "dev.md")
	data, err := os.ReadFile(devPath)
	if err != nil {
		t.Fatalf("failed to read dev.md: %v", err)
	}

	content := string(data)

	// Frontmatter: lowercase name, description, tools
	if !strings.Contains(content, "name: dev") {
		t.Error("PI markdown should contain name: dev")
	}
	if !strings.Contains(content, "description: >") || !strings.Contains(content, "Developer agent") {
		t.Error("PI markdown should contain a folded-block description")
	}
	if !strings.Contains(content, "tools: read, edit, bash") {
		t.Errorf("PI markdown should contain lowercase tools, got content:\n%s", content)
	}

	// No mode: or permission: block
	if strings.Contains(content, "mode:") {
		t.Error("PI markdown should NOT contain mode:")
	}
	if strings.Contains(content, "permission:") {
		t.Error("PI markdown should NOT contain permission:")
	}

	// Body present
	if !strings.Contains(content, "# Dev") {
		t.Error("PI markdown should contain prompt body")
	}
}

func TestInstallPiSkipsExisting(t *testing.T) {
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	existingPath := filepath.Join(agentsDir, "dev.md")
	if err := os.WriteFile(existingPath, []byte("---\nname: dev\ndescription: Old\ntools: read\n---\n\nKeep me"), 0o644); err != nil {
		t.Fatal(err)
	}

	profiles := map[string]AgentProfile{
		"dev": {
			Name:        "dev",
			Description: "New description",
			Prompt:      "Should not overwrite",
			Permission:  map[string]string{"read": "allow"},
			Mode:        "subagent",
		},
	}

	if err := InstallPi(agentsDir, profiles, false); err != nil {
		t.Fatalf("InstallPi() error = %v", err)
	}

	data, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "Keep me") {
		t.Error("existing file should not be overwritten")
	}
	if strings.Contains(content, "New description") {
		t.Error("existing file should not contain new content")
	}
}

func TestInstallPiOverwriteExisting(t *testing.T) {
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	existingPath := filepath.Join(agentsDir, "dev.md")
	if err := os.WriteFile(existingPath, []byte("---\nname: dev\ndescription: Old\ntools: read\n---\n\nKeep me"), 0o644); err != nil {
		t.Fatal(err)
	}

	profiles := map[string]AgentProfile{
		"dev": {
			Name:        "dev",
			Description: "New description",
			Prompt:      "Should overwrite",
			Permission:  map[string]string{"read": "allow", "edit": "allow"},
			Mode:        "subagent",
		},
	}

	if err := InstallPi(agentsDir, profiles, true); err != nil {
		t.Fatalf("InstallPi() error = %v", err)
	}

	data, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if strings.Contains(content, "Keep me") {
		t.Error("existing file should be overwritten")
	}
	if !strings.Contains(content, "New description") {
		t.Error("overwritten file should contain new content")
	}
	if !strings.Contains(content, "Should overwrite") {
		t.Error("overwritten file should contain new prompt")
	}
}
