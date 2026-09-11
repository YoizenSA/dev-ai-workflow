package skills

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMarkerTagsAndDescription(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, extraSkillMarkerFile), []byte("managed-by: ywai\r\ntags: Testing, e2e , ,frontend\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\r\nname: x\r\ndescription: \"Does x. Trigger: x.\"\r\n---\r\n\r\ndescription: body, not frontmatter\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, want := markerTags(dir), []string{"testing", "e2e", "frontend"}; !reflect.DeepEqual(got, want) {
		t.Errorf("markerTags = %v, want %v", got, want)
	}
	if got := skillDescription(dir); got != "Does x. Trigger: x." {
		t.Errorf("skillDescription = %q", got)
	}
}

func TestMarkerTagsLegacyMarker(t *testing.T) {
	dir := t.TempDir()
	// Older markers carry only managed-by (or nothing): no tags, never nil.
	if err := os.WriteFile(filepath.Join(dir, extraSkillMarkerFile), []byte("managed-by: ywai\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := markerTags(dir); got == nil || len(got) != 0 {
		t.Errorf("markerTags legacy = %#v, want empty non-nil", got)
	}
	if got := markerTags(t.TempDir()); got == nil || len(got) != 0 {
		t.Errorf("markerTags missing = %#v, want empty non-nil", got)
	}
}
