package control

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Skill surface: every SKILL.md OpenCode can load, per the six documented
// locations (project .opencode/.claude/.agents walking up to the repo root,
// plus the three global config dirs). Names must be unique across all of
// them; anything else is a shadowing report, not an error.

// surfaceLocation is one scanned root.
type surfaceLocation struct {
	Key   string `json:"key"`   // global-opencode | global-claude | global-agents | project-opencode | ...
	Label string `json:"label"` // human line for the UI
	Path  string `json:"path"`
	Found bool   `json:"found"`
}

type surfaceEntry struct {
	Location string `json:"location"` // surfaceLocation.Key
	Path     string `json:"path"`     // skill dir
	Hash     string `json:"hash"`     // sha256 of SKILL.md, 12 hex chars; "" when unreadable
	Symlink  string `json:"symlink,omitempty"`
	Broken   bool   `json:"broken,omitempty"`
}

type surfaceSkill struct {
	Name    string         `json:"name"`
	Status  string         `json:"status"` // unique | shadowed | unreadable
	Entries []surfaceEntry `json:"entries"`
}

type skillSurface struct {
	ProjectDir string            `json:"projectDir"`
	Locations  []surfaceLocation `json:"locations"`
	Skills     []surfaceSkill    `json:"skills"`
}

// scanSkillSurface is pure over explicit roots (home-independent, temp-dir
// testable). globalRoots maps location key to dir; projectChains lists, per
// location key, the candidate dirs from innermost to outermost.
func scanSkillSurface(globalRoots map[string]string, projectChains map[string][]string) skillSurface {
	locs := make([]surfaceLocation, 0, len(globalRoots)+len(projectChains))
	byName := map[string][]surfaceEntry{}

	add := func(key, label, dir string) {
		st, err := os.Stat(dir)
		locs = append(locs, surfaceLocation{Key: key, Label: label, Path: dir, Found: err == nil && st.IsDir()})
		if err != nil || !st.IsDir() {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() && e.Type()&os.ModeSymlink == 0 {
				continue
			}
			skillDir := filepath.Join(dir, e.Name())
			body, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
			entry := surfaceEntry{Location: key, Path: skillDir}
			if err != nil {
				entry.Hash = ""
			} else {
				sum := sha256.Sum256(body)
				entry.Hash = hex.EncodeToString(sum[:])[:12]
			}
			if target, err := os.Readlink(skillDir); err == nil {
				entry.Symlink = target
				if _, err := os.Stat(skillDir); err != nil {
					entry.Broken = true
				}
			}
			byName[e.Name()] = append(byName[e.Name()], entry)
		}
	}

	keys := make([]string, 0, len(globalRoots))
	for k := range globalRoots {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		add(k, "Global "+strings.TrimPrefix(k, "global-"), globalRoots[k])
	}

	pkeys := make([]string, 0, len(projectChains))
	for k := range projectChains {
		pkeys = append(pkeys, k)
	}
	sort.Strings(pkeys)
	for _, k := range pkeys {
		for _, dir := range projectChains[k] {
			add(k, "Project "+strings.TrimPrefix(k, "project-"), dir)
		}
	}

	skills := make([]surfaceSkill, 0, len(byName))
	for name, entries := range byName {
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].Location != entries[j].Location {
				return entries[i].Location < entries[j].Location
			}
			return entries[i].Path < entries[j].Path
		})
		status := "unique"
		hashes := map[string]bool{}
		readable := false
		for _, e := range entries {
			if e.Broken {
				continue
			}
			if e.Hash == "" {
				continue
			}
			readable = true
			hashes[e.Hash] = true
		}
		switch {
		case !readable:
			status = "unreadable"
		case len(hashes) > 1:
			status = "shadowed"
		}
		skills = append(skills, surfaceSkill{Name: name, Status: status, Entries: entries})
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	return skillSurface{Locations: locs, Skills: skills}
}

// projectSkillChains walks from dir up to the filesystem root collecting the
// three per-project skill roots, innermost first.
func projectSkillChains(dir string) map[string][]string {
	out := map[string][]string{
		"project-opencode": {},
		"project-claude":   {},
		"project-agents":   {},
	}
	cur, err := filepath.Abs(dir)
	if err != nil {
		return out
	}
	for i := 0; i < 25; i++ {
		out["project-opencode"] = append(out["project-opencode"], filepath.Join(cur, ".opencode", "skills"))
		out["project-claude"] = append(out["project-claude"], filepath.Join(cur, ".claude", "skills"))
		out["project-agents"] = append(out["project-agents"], filepath.Join(cur, ".agents", "skills"))
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return out
}

// handleSkillSurface returns every skill OpenCode can load for a project.
func (s *Server) handleSkillSurface(w http.ResponseWriter, r *http.Request) {
	projectDir := strings.TrimSpace(r.URL.Query().Get("project_dir"))
	if projectDir == "" {
		var err error
		projectDir, err = os.Getwd()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	home, _ := os.UserHomeDir()
	surface := scanSkillSurface(
		map[string]string{
			"global-opencode": filepath.Join(home, ".config", "opencode", "skills"),
			"global-claude":   filepath.Join(home, ".claude", "skills"),
			"global-agents":   filepath.Join(home, ".agents", "skills"),
		},
		projectSkillChains(projectDir),
	)
	surface.ProjectDir = projectDir
	writeJSON(w, http.StatusOK, surface)
}
