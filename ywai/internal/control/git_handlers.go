package control

import (
	"net/http"
	"sync"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/git"
)

// gitStatusCache holds a cached git status result.
var (
	gitStatusMu    sync.RWMutex
	gitStatusCache *git.GitStatus
	gitStatusTime  time.Time
	gitStatusTTL   = 10 * time.Second
)

// registerGitRoutes adds git-related API endpoints.
func (s *Server) registerGitRoutes() {
	s.mux.HandleFunc("GET /api/git/status", s.handleGitStatus)
}

func (s *Server) handleGitStatus(w http.ResponseWriter, r *http.Request) {
	gitStatusMu.RLock()
	if gitStatusCache != nil && time.Since(gitStatusTime) < gitStatusTTL {
		st := gitStatusCache
		gitStatusMu.RUnlock()
		writeJSON(w, http.StatusOK, st)
		return
	}
	gitStatusMu.RUnlock()

	st, err := git.GetStatus(".")
	if err != nil {
		writeJSON(w, http.StatusOK, &git.GitStatus{Branch: "n/a"})
		return
	}

	gitStatusMu.Lock()
	gitStatusCache = st
	gitStatusTime = time.Now()
	gitStatusMu.Unlock()

	writeJSON(w, http.StatusOK, st)
}
