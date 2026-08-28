package agents

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// frontmatterOf returns the YAML frontmatter block of a generated agent file.
func frontmatterOf(t *testing.T, md string) string {
	t.Helper()
	lines := strings.Split(md, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		t.Fatalf("no opening delimiter:\n%s", md)
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n")
		}
	}
	t.Fatalf("no closing delimiter:\n%s", md)
	return ""
}

// TestBuildOpenCodeMarkdown_FrontmatterIsValidYAML guards the regression where
// every ywai agent leaked its own frontmatter into the system prompt. The
// descriptions carry a "Trigger: ..." clause, and a plain YAML scalar may not
// contain ": " — the block failed to parse, opencode fell back to the legacy v1
// decode, and the raw file became the prompt with description and permission
// dropped. Only `advisor` survived, because it is the one description with no
// colon in it. Assert the parse, not the substring: a Contains check passed
// happily while the file was unparseable.
func TestBuildOpenCodeMarkdown_FrontmatterIsValidYAML(t *testing.T) {
	descriptions := []string{
		`Technical lead / orchestrator. Trigger: A goal or feature request, "build X", multi-step tasks.`,
		"Reviewer that shadows another agent's session.",
		"colon: at the very start",
		"trailing colon:",
		"hash # comment and a *star and a &anchor",
		"quotes \"inside\" and a backslash \\ too",
		"multi\nline\ndescription",
	}
	for _, desc := range descriptions {
		profile := AgentProfile{
			Name:        "probe",
			Description: desc,
			Prompt:      "# Probe\n\nBody.",
			Permission:  map[string]string{"read": "allow"},
			Mode:        "all",
		}
		md := BuildOpenCodeMarkdown("probe", profile)

		var fm struct {
			Description string         `yaml:"description"`
			Mode        string         `yaml:"mode"`
			Permission  map[string]any `yaml:"permission"`
		}
		if err := yaml.Unmarshal([]byte(frontmatterOf(t, md)), &fm); err != nil {
			t.Fatalf("frontmatter does not parse for %q: %v\n%s", desc, err, md)
		}
		if fm.Description == "" {
			t.Errorf("description lost for %q:\n%s", desc, md)
		}
		if fm.Mode != "all" {
			t.Errorf("mode lost for %q: got %q", desc, fm.Mode)
		}
		if len(fm.Permission) == 0 {
			t.Errorf("permission map lost for %q:\n%s", desc, md)
		}
	}
}

// A description spanning lines must not break out of the frontmatter block.
func TestBuildOpenCodeMarkdown_CollapsesMultilineDescription(t *testing.T) {
	profile := AgentProfile{
		Description: "first line\n  second line",
		Prompt:      "body",
		Permission:  map[string]string{"read": "allow"},
		Mode:        "all",
	}
	md := BuildOpenCodeMarkdown("probe", profile)
	if strings.Contains(frontmatterOf(t, md), "\n  second line") {
		t.Errorf("description newline survived into the frontmatter:\n%s", md)
	}
}
