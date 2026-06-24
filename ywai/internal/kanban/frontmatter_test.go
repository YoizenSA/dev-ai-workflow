package kanban

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

func TestExtractMarkdownSection_DirectContent(t *testing.T) {
	content, ok := extractMarkdownSection(sampleBody, "Delegation Rules", false)
	if !ok {
		t.Fatal("expected section to be found")
	}
	// Direct content stops before the "#### Mandatory..." sub-heading.
	if strings.Contains(content, "4-file rule") {
		t.Errorf("direct slice should not include sub-section, got:\n%s", content)
	}
	if !strings.Contains(content, "core principle") {
		t.Errorf("expected direct body to contain 'core principle', got:\n%s", content)
	}
	if !strings.Contains(content, "| Action | Inline | Delegate |") {
		t.Errorf("expected the table in direct content, got:\n%s", content)
	}
}

func TestExtractMarkdownSection_WithSubsections(t *testing.T) {
	content, ok := extractMarkdownSection(sampleBody, "Delegation Rules", true)
	if !ok {
		t.Fatal("expected section to be found")
	}
	// includeSubsections=true keeps nested headings.
	if !strings.Contains(content, "4-file rule") {
		t.Errorf("expected sub-section content, got:\n%s", content)
	}
	// But stops at the next same-level heading ("### Cost...").
	if strings.Contains(content, "Cost and Context") {
		t.Errorf("should not include sibling section, got:\n%s", content)
	}
}

func TestExtractMarkdownSection_NotFound(t *testing.T) {
	_, ok := extractMarkdownSection(sampleBody, "Nonexistent Section", false)
	if ok {
		t.Error("expected ok=false for a missing section")
	}
}

func TestExtractMarkdownSection_StopsAtSiblingHeading(t *testing.T) {
	content, ok := extractMarkdownSection(sampleBody, "Mandatory Delegation Triggers", true)
	if !ok {
		t.Fatal("expected sub-section found")
	}
	if !strings.Contains(content, "4-file rule") {
		t.Errorf("expected trigger content, got:\n%s", content)
	}
	// "### Cost..." is a sibling (level 3 vs level 4) → stop boundary.
	if strings.Contains(content, "Cost and Context") {
		t.Errorf("should stop before sibling ### heading, got:\n%s", content)
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

func TestReplaceMarkdownSection_RoundTrip(t *testing.T) {
	original, _ := extractMarkdownSection(sampleBody, "Delegation Rules", false)
	out := replaceMarkdownSection(sampleBody, "Delegation Rules", "###", original, false)
	again, _ := extractMarkdownSection(out, "Delegation Rules", false)
	if strings.TrimSpace(again) != strings.TrimSpace(original) {
		t.Errorf("round-trip mismatch:\nwant:\n%s\ngot:\n%s", original, again)
	}
}
