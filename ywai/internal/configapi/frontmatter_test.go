package configapi

import (
	"strings"
	"testing"
)

const sampleBody = `# Title

intro line.

### Delegation Rules

core principle: does this inflate my context?

| Action | Inline | Delegate |
| ------ | ------ | -------- |
| Read to decide | Yes | No |
| Read to explore | No | Yes |

#### Mandatory Delegation Triggers

1. **4-file rule**: delegate exploration.
2. **Multi-file write rule**: delegate one writer.

### Cost and Context Balance

some other content.`

func TestHeadingLevel(t *testing.T) {
	cases := map[string]int{
		"# H1":           1,
		"## H2":          2,
		"### H3":         3,
		"###### H6":      6,
		"####### H7":     0, // 7 hashes is not a valid heading
		"plain text":     0,
		"#No space":      0,
		"  ### indented": 3,
		"":               0,
	}
	for line, want := range cases {
		if got := headingLevel(line); got != want {
			t.Errorf("headingLevel(%q) = %d, want %d", line, got, want)
		}
	}
}

func TestReplaceMarkdownSection_Existing(t *testing.T) {
	newContent := "REPLACEMENT BODY\n| A | B |"
	out := replaceMarkdownSection(sampleBody, "Delegation Rules", "###", newContent, false)

	// Heading preserved.
	if !strings.Contains(out, "### Delegation Rules") {
		t.Error("heading line should be preserved")
	}
	// New content present.
	if !strings.Contains(out, "REPLACEMENT BODY") {
		t.Error("new content should be present")
	}
	// Old content gone (direct slice replaced).
	if strings.Contains(out, "core principle") {
		t.Error("old direct content should be gone")
	}
	// Sibling section preserved.
	if !strings.Contains(out, "Cost and Context Balance") {
		t.Error("sibling section should be untouched")
	}
}

func TestReplaceMarkdownSection_AppendsWhenAbsent(t *testing.T) {
	out := replaceMarkdownSection(sampleBody, "Brand New Section", "###", "fresh body", false)
	if !strings.Contains(out, "### Brand New Section") {
		t.Error("appended heading should be present")
	}
	if !strings.Contains(out, "fresh body") {
		t.Error("appended content should be present")
	}
	// Original content still intact.
	if !strings.Contains(out, "Cost and Context Balance") {
		t.Error("original content should remain")
	}
}

// directDelegationRulesSlice reads the direct (no-subsections) content of the
// "### Delegation Rules" section: every line between that heading and the
// next heading of any level, without the heading lines themselves.
func directDelegationRulesSlice(body string) string {
	lines := strings.Split(body, "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "### Delegation Rules" {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return ""
	}
	var out []string
	for _, l := range lines[start:] {
		if strings.HasPrefix(strings.TrimSpace(l), "#") {
			break
		}
		out = append(out, l)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func TestReplaceMarkdownSection_RoundTrip(t *testing.T) {
	original := directDelegationRulesSlice(sampleBody)
	if original == "" {
		t.Fatal("could not read the Delegation Rules slice from sampleBody")
	}
	out := replaceMarkdownSection(sampleBody, "Delegation Rules", "###", original, false)
	again := directDelegationRulesSlice(out)
	if strings.TrimSpace(again) != strings.TrimSpace(original) {
		t.Errorf("round-trip mismatch:\nwant:\n%s\ngot:\n%s", original, again)
	}
}

// Regression: repeated scalar edits used to add a pair of blank lines to the
// frontmatter on every call, because parseFrontmatter returned the delimiter
// newlines and Split/Join round-tripped them into real lines. Installed agent
// files accumulated dozens of blank lines between "---" and the first key.
func TestSetScalarFrontmatterField_Idempotent(t *testing.T) {
	content := "---\ndescription: agent\nmode: all\n---\n\nbody\n"

	once := setScalarFrontmatterField(content, "model", "prov/a")
	if strings.HasPrefix(once, "---\n\n") {
		t.Fatalf("blank line after opening delimiter:\n%q", once)
	}

	got := once
	for i := 0; i < 5; i++ {
		got = setScalarFrontmatterField(got, "model", "prov/a")
	}
	if got != once {
		t.Fatalf("not idempotent after 5 rewrites:\nfirst: %q\nlast:  %q", once, got)
	}
}
