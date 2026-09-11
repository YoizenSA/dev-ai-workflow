package skills

import (
	"os"
	"path/filepath"
	"strings"
)

// SkillMeta describes one ywai extra skill for pickers (web Envs editor).
type SkillMeta struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	// Tags come from the "tags:" line of the skill's .ywai-extra marker.
	// The first tag is the skill's category.
	Tags []string `json:"tags"`
}

// ListAvailableMeta is ListAvailable plus each skill's description (SKILL.md
// frontmatter) and tags (.ywai-extra marker).
func ListAvailableMeta() ([]SkillMeta, error) {
	names, err := ListAvailable()
	if err != nil {
		return nil, err
	}
	srcDir := skillsSourceDir()
	out := make([]SkillMeta, 0, len(names))
	for _, n := range names {
		dir := filepath.Join(srcDir, n)
		out = append(out, SkillMeta{Name: n, Description: skillDescription(dir), Tags: markerTags(dir)})
	}
	return out, nil
}

// markerTags reads "tags: a, b" from the .ywai-extra marker, lowercased and
// trimmed. A marker without a tags line (or no marker) yields an empty list.
func markerTags(dir string) []string {
	tags := []string{}
	data, err := os.ReadFile(filepath.Join(dir, extraSkillMarkerFile))
	if err != nil {
		return tags
	}
	for _, line := range strings.Split(string(data), "\n") {
		v, ok := strings.CutPrefix(strings.TrimSpace(line), "tags:")
		if !ok {
			continue
		}
		for _, t := range strings.Split(v, ",") {
			if t = strings.ToLower(strings.TrimSpace(t)); t != "" {
				tags = append(tags, t)
			}
		}
	}
	return tags
}

// skillDescription returns the frontmatter description of dir/SKILL.md, or ""
// when the file or field is missing.
func skillDescription(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return ""
	}
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			break
		}
		if v, ok := strings.CutPrefix(line, "description:"); ok {
			return strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return ""
}
