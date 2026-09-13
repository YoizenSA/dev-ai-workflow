package skills

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// TestCurriculumDocPagesExist pins the learn-ywai curriculum to the docs tree:
// every .mdx path the curriculum mentions must exist under docs/src/content/docs.
// It catches stale pages after doc renames (kanban -> panel, commands -> cli).
func TestCurriculumDocPagesExist(t *testing.T) {
	curriculum := filepath.Join("..", "..", "skills", "learn-ywai", "references", "curriculum.md")
	data, err := os.ReadFile(curriculum)
	if err != nil {
		t.Fatalf("read curriculum: %v", err)
	}
	docsRoot := filepath.Join("..", "..", "..", "docs", "src", "content", "docs")
	if _, err := os.Stat(docsRoot); err != nil {
		t.Skip("docs tree not present in this checkout")
	}
	re := regexp.MustCompile(`[\w-]+(?:/[\w-]+)*\.mdx`)
	found := map[string]bool{}
	for _, m := range re.FindAllString(string(data), -1) {
		if found[m] {
			continue
		}
		found[m] = true
		if _, err := os.Stat(filepath.Join(docsRoot, filepath.FromSlash(m))); err != nil {
			t.Errorf("curriculum references missing docs page %q", m)
		}
	}
	if len(found) == 0 {
		t.Fatal("curriculum lists no .mdx pages; file or regex drifted")
	}
}
