package control

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	Name string `json:"name"`
	// unique: one readable copy · duplicate: 2+ byte-identical copies ·
	// shadowed: 2+ copies with different content · unreadable: no SKILL.md.
	Status  string         `json:"status"`
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
		hashes := map[string]bool{}
		readable := 0
		for _, e := range entries {
			if e.Broken || e.Hash == "" {
				continue
			}
			readable++
			hashes[e.Hash] = true
		}
		status := "unique"
		switch {
		case readable == 0:
			status = "unreadable"
		case len(hashes) > 1:
			status = "shadowed"
		case readable > 1:
			status = "duplicate"
		}
		skills = append(skills, surfaceSkill{Name: name, Status: status, Entries: entries})
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	return skillSurface{Locations: locs, Skills: skills}
}

// projectSkillChains walks from dir up to the repo root (the first dir with
// a .git) collecting the three per-project skill roots, innermost first. It
// never enters the user's home: ~/.agents, ~/.claude and ~/.config/opencode
// are the global roots, and scanning them again as "project" roots made every
// global skill look duplicated for repos living under the home dir.
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
	home := ""
	if h, err := userHomeDir(); err == nil && h != "" {
		if abs, err := filepath.Abs(h); err == nil {
			home = abs
		}
	}
	for i := 0; i < 25; i++ {
		if home != "" && strings.EqualFold(filepath.Clean(cur), filepath.Clean(home)) {
			break
		}
		out["project-opencode"] = append(out["project-opencode"], filepath.Join(cur, ".opencode", "skills"))
		out["project-claude"] = append(out["project-claude"], filepath.Join(cur, ".claude", "skills"))
		out["project-agents"] = append(out["project-agents"], filepath.Join(cur, ".agents", "skills"))
		if _, err := os.Stat(filepath.Join(cur, ".git")); err == nil {
			break // repo root: OpenCode does not look above it
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return out
}

// userHomeDir is os.UserHomeDir behind a seam: on Windows the stdlib
// resolves home via the shell API, ignoring %USERPROFILE% overrides, so
// tests point this at temp dirs instead.
var userHomeDir = os.UserHomeDir

func surfaceGlobals() map[string]string {
	home, _ := userHomeDir()
	return map[string]string{
		"global-opencode": filepath.Join(home, ".config", "opencode", "skills"),
		"global-claude":   filepath.Join(home, ".claude", "skills"),
		"global-agents":   filepath.Join(home, ".agents", "skills"),
	}
}

// surfaceRoots returns every scanned skill root for a project: the three
// globals plus the walked-up project chains.
func surfaceRoots(projectDir string) []string {
	roots := []string{}
	for _, dir := range surfaceGlobals() {
		roots = append(roots, dir)
	}
	for _, chain := range projectSkillChains(projectDir) {
		roots = append(roots, chain...)
	}
	return roots
}

// resolveSurfacePath cleans a user-supplied skill path and verifies it lives
// inside one of the scanned roots.
func resolveSurfacePath(rawPath, projectDir string) (string, error) {
	target, err := filepath.Abs(rawPath)
	if err != nil {
		return "", fmt.Errorf("unresolvable path")
	}
	for _, root := range surfaceRoots(projectDir) {
		abs, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		if rel, err := filepath.Rel(abs, target); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return target, nil
		}
	}
	return "", fmt.Errorf("path is outside every scanned skill root")
}

// removeSurfaceEntry removes one scanned skill directory: symlinks go as
// links (never followed); directories need SKILL.md unless empty.
func removeSurfaceEntry(target string) error {
	st, err := os.Lstat(target)
	if err != nil {
		return err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		return os.Remove(target)
	}
	if !st.IsDir() {
		return fmt.Errorf("not a skill directory")
	}
	if _, err := os.Stat(filepath.Join(target, "SKILL.md")); err != nil {
		entries, readErr := os.ReadDir(target)
		if readErr != nil || len(entries) > 0 {
			return fmt.Errorf("no SKILL.md at path")
		}
		return os.Remove(target)
	}
	return os.RemoveAll(target)
}

func (s *Server) handleSkillSurfaceDelete(w http.ResponseWriter, r *http.Request) {
	rawPath := strings.TrimSpace(r.URL.Query().Get("path"))
	if rawPath == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "path is required"})
		return
	}
	projectDir := strings.TrimSpace(r.URL.Query().Get("project_dir"))
	if projectDir == "" {
		var err error
		projectDir, err = os.Getwd()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	target, err := resolveSurfacePath(rawPath, projectDir)
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}
	if err := removeSurfaceEntry(target); err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "no SKILL.md") {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// standardizeAction is one planned or executed normalization step.
type standardizeAction struct {
	Kind   string `json:"kind"` // delete-broken-link | delete-empty-dir | resolve-shadow | remove-duplicate
	Name   string `json:"name"`
	Path   string `json:"path"`
	Detail string `json:"detail"`
}

// locationRank orders where the canonical copy of a shadowed skill lives:
// project intent beats global defaults, and the canonical ~/.agents/skills
// beats per-host copies (OpenCode reads it natively; ~/.claude/skills holds
// ywai compat links, ~/.config/opencode/skills only legacy duplicates).
func locationRank(location string) int {
	switch location {
	case "project-opencode":
		return 0
	case "project-claude":
		return 1
	case "project-agents":
		return 2
	case "global-agents":
		return 3
	case "global-opencode":
		return 4
	case "global-claude":
		return 5
	}
	return 6
}

// dedupeRank orders which byte-identical copy to keep: project intent first,
// then the canonical ~/.agents/skills — one physical copy there serves every
// host, while ~/.claude/skills compat links and ~/.config/opencode/skills
// legacy copies go first.
func dedupeRank(location string) int {
	switch location {
	case "project-opencode":
		return 0
	case "project-claude":
		return 1
	case "project-agents":
		return 2
	case "global-agents":
		return 3
	case "global-claude":
		return 4
	case "global-opencode":
		return 5
	}
	return 6
}

// planStandardize derives the normalization plan from a fresh scan. Broken
// links and empty debris dirs go unconditionally; shadowed names keep the
// highest-ranked readable copy and drop the rest; byte-identical duplicates
// are dropped only when dedupe is set (they are sometimes intentional compat
// copies), keeping the copy the most tools can read.
func planStandardize(surface skillSurface, dedupe bool) []standardizeAction {
	actions := []standardizeAction{}
	for _, sk := range surface.Skills {
		var readable []surfaceEntry
		for _, e := range sk.Entries {
			switch {
			case e.Broken:
				actions = append(actions, standardizeAction{
					Kind: "delete-broken-link", Name: sk.Name, Path: e.Path,
					Detail: "symlink target is gone; removing the link only",
				})
			case e.Hash == "":
				actions = append(actions, standardizeAction{
					Kind: "delete-empty-dir", Name: sk.Name, Path: e.Path,
					Detail: "no readable SKILL.md; removed only when empty",
				})
			default:
				readable = append(readable, e)
			}
		}
		if len(readable) < 2 {
			continue
		}
		var kind string
		rank := locationRank
		switch {
		case sk.Status == "shadowed":
			kind = "resolve-shadow"
		case sk.Status == "duplicate" && dedupe:
			kind, rank = "remove-duplicate", dedupeRank
		default:
			continue
		}
		best := readable[0]
		for _, e := range readable[1:] {
			if rank(e.Location) < rank(best.Location) {
				best = e
			}
		}
		for _, e := range readable {
			if e.Path == best.Path {
				continue
			}
			detail := fmt.Sprintf("kept %s (%s)", best.Path, best.Location)
			if kind == "remove-duplicate" {
				detail = fmt.Sprintf("identical copy; kept %s (%s)", best.Path, best.Location)
			}
			actions = append(actions, standardizeAction{Kind: kind, Name: sk.Name, Path: e.Path, Detail: detail})
		}
	}
	sort.Slice(actions, func(i, j int) bool {
		if actions[i].Name != actions[j].Name {
			return actions[i].Name < actions[j].Name
		}
		return actions[i].Path < actions[j].Path
	})
	return actions
}

// standardizeRequest optionally narrows an execute to the paths the user
// kept checked in the preview. Paths outside the fresh plan are ignored, so
// the endpoint can never delete something the plan did not propose.
type standardizeRequest struct {
	Paths []string `json:"paths"`
}

// handleSkillSurfaceStandardize previews (dry_run=1) or executes the
// normalization plan: drop broken links, drop empty debris, resolve
// shadowed names to one canonical copy, and — with dedupe=1 — drop
// byte-identical duplicates. A JSON body {"paths": [...]} limits an execute
// to those planned actions.
func (s *Server) handleSkillSurfaceStandardize(w http.ResponseWriter, r *http.Request) {
	projectDir := strings.TrimSpace(r.URL.Query().Get("project_dir"))
	if projectDir == "" {
		var err error
		projectDir, err = os.Getwd()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}
	surface := scanSkillSurface(
		surfaceGlobals(),
		projectSkillChains(projectDir),
	)
	actions := planStandardize(surface, r.URL.Query().Get("dedupe") == "1")
	if r.URL.Query().Get("dry_run") == "1" {
		writeJSON(w, http.StatusOK, map[string]any{"projectDir": projectDir, "actions": actions})
		return
	}

	var req standardizeRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body: " + err.Error()})
			return
		}
	}
	if req.Paths != nil {
		want := make(map[string]bool, len(req.Paths))
		for _, p := range req.Paths {
			want[filepath.Clean(p)] = true
		}
		selected := actions[:0:0]
		for _, a := range actions {
			if want[filepath.Clean(a.Path)] {
				selected = append(selected, a)
			}
		}
		actions = selected
	}

	done := []standardizeAction{}
	failed := []map[string]string{}
	for _, a := range actions {
		target, err := resolveSurfacePath(a.Path, projectDir)
		if err != nil {
			failed = append(failed, map[string]string{"path": a.Path, "error": err.Error()})
			continue
		}
		if err := removeSurfaceEntry(target); err != nil {
			failed = append(failed, map[string]string{"path": a.Path, "error": err.Error()})
			continue
		}
		done = append(done, a)
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": done, "failed": failed})
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

	surface := scanSkillSurface(
		surfaceGlobals(),
		projectSkillChains(projectDir),
	)
	surface.ProjectDir = projectDir
	writeJSON(w, http.StatusOK, surface)
}
