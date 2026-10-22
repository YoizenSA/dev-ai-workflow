package control

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// runIDPattern: the slug an evidence run directory must match. Same shape as
// the workflow name rules, so a run id is always filesystem-safe.
var runIDPattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

// registerEvidenceRoutes mounts the read-only Evidence API: it lists the
// .evidence/<run-id>/ folders the qa-evidence skill leaves in the user's
// project. ?project_dir= selects the project; it defaults to the server cwd.
func (s *Server) registerEvidenceRoutes() {
	s.mux.HandleFunc("GET /api/evidence", s.handleEvidenceList)
	s.mux.HandleFunc("GET /api/evidence/{run}", s.handleEvidenceRun)
}

// evidenceProjectDir resolves ?project_dir= with the cwd fallback — the same
// convention as the skills surface API.
func evidenceProjectDir(r *http.Request) (string, error) {
	dir := strings.TrimSpace(r.URL.Query().Get("project_dir"))
	if dir == "" {
		return os.Getwd()
	}
	return dir, nil
}

// evidenceRunDir resolves the {run} path value to a directory inside the
// .evidence root. Containment is checked before the slug so a traversal
// attempt answers 403 and a mere bad slug answers 400. The bool is false when
// the error response is already written.
func evidenceRunDir(w http.ResponseWriter, evidenceRoot, runID string) (string, bool) {
	abs, err := filepath.Abs(filepath.Join(evidenceRoot, runID))
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "unresolvable run path"})
		return "", false
	}
	rel, relErr := filepath.Rel(evidenceRoot, abs)
	if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "run path escapes .evidence"})
		return "", false
	}
	if !runIDPattern.MatchString(runID) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid run id"})
		return "", false
	}
	return abs, true
}

// evidenceRunSummary is one row of the list endpoint.
type evidenceRunSummary struct {
	RunID    string    `json:"runId"`
	Files    int       `json:"files"`
	Modified time.Time `json:"modified"`
}

// handleEvidenceList answers [{runId, files, modified}] for the run
// directories under <projectDir>/.evidence, newest first. An absent or empty
// .evidence yields [] — never a 500.
func (s *Server) handleEvidenceList(w http.ResponseWriter, r *http.Request) {
	projectDir, err := evidenceProjectDir(r)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	root := filepath.Join(projectDir, ".evidence")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, []evidenceRunSummary{})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	runs := []evidenceRunSummary{}
	for _, e := range entries {
		if !e.IsDir() || !runIDPattern.MatchString(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files, err := os.ReadDir(filepath.Join(root, e.Name()))
		if err != nil {
			continue
		}
		runs = append(runs, evidenceRunSummary{RunID: e.Name(), Files: len(files), Modified: info.ModTime()})
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i].Modified.After(runs[j].Modified) })
	writeJSON(w, http.StatusOK, runs)
}

// evidenceFile is one file in a run directory.
type evidenceFile struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// evidenceRunDetail is the detail endpoint's payload. Report is the contents
// of report.md (optional per the evidence contract) or "".
type evidenceRunDetail struct {
	RunID  string         `json:"runId"`
	Report string         `json:"report"`
	Files  []evidenceFile `json:"files"`
}

// handleEvidenceRun answers {runId, report, files} for one run directory.
func (s *Server) handleEvidenceRun(w http.ResponseWriter, r *http.Request) {
	projectDir, err := evidenceProjectDir(r)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	root, err := filepath.Abs(filepath.Join(projectDir, ".evidence"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	runID := r.PathValue("run")
	dir, ok := evidenceRunDir(w, root, runID)
	if !ok {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such evidence run"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	detail := evidenceRunDetail{RunID: runID, Files: []evidenceFile{}}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		detail.Files = append(detail.Files, evidenceFile{Name: e.Name(), Size: info.Size()})
	}
	// report.md is optional; any read problem just means an empty report.
	if b, err := os.ReadFile(filepath.Join(dir, "report.md")); err == nil {
		detail.Report = string(b)
	}
	writeJSON(w, http.StatusOK, detail)
}
