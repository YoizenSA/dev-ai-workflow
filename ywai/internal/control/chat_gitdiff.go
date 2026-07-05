package control

import (
	"net/http"
	"os"
	"os/exec"
	"strings"
)

// handleGitDiff returns a unified diff of the working tree for a workspace
// directory, scoped to unstaged, staged, or all branch changes. This exposes
// real repo changes the OpenCode session diff does not track.
// GET /api/chat/gitdiff?dir=<path>&scope=unstaged|staged|branch
func (cp *ChatProxy) handleGitDiff(w http.ResponseWriter, r *http.Request) {
	dir := r.URL.Query().Get("dir")
	scope := r.URL.Query().Get("scope")
	if dir == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "dir is required"})
		return
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "dir is not a directory"})
		return
	}

	var args []string
	switch scope {
	case "staged":
		args = []string{"diff", "--cached"}
	case "branch":
		base := gitDefaultBase(dir)
		if base == "" {
			writeJSON(w, http.StatusOK, map[string]string{"diff": ""})
			return
		}
		args = []string{"diff", base + "...HEAD"}
	default: // "unstaged"
		args = []string{"diff"}
	}

	out, err := gitRun(dir, args...)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"diff": "", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"diff": out})
}

// gitRun executes a git command in dir and returns stdout (stderr is ignored;
// a non-zero exit surfaces as err).
func gitRun(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	return string(out), err
}

// gitDefaultBase resolves the default branch ref to diff a feature branch
// against. Tries origin/HEAD, then common main/master fallbacks.
// ponytail: naive default-branch detection; good enough for the diff panel.
func gitDefaultBase(dir string) string {
	if ref, err := gitRun(dir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		if r := strings.TrimSpace(ref); r != "" {
			return r
		}
	}
	for _, cand := range []string{"origin/main", "origin/master", "main", "master"} {
		if _, err := gitRun(dir, "rev-parse", "--verify", cand); err == nil {
			return cand
		}
	}
	return ""
}
