package control

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/envprofile"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/evals"
)

// Eval environments: named OpenCode targets for the evals UI. One selector
// above the three tabs picks the pair; analytics reads that env's database,
// benches run against that env's server, history filters to that env's runs.
// The implicit "local" environment (empty fields) preserves the historical
// behavior exactly: default server resolution, default database path.

// loadEvalEnvironments is a seam: production reads the user config, tests
// substitute temp data.
var loadEvalEnvironments = func() []config.EvalEnvironment {
	cfg, err := config.LoadConfig()
	if err != nil || len(cfg.EvalEnvironments) == 0 {
		return []config.EvalEnvironment{{Name: "local"}}
	}
	return cfg.EvalEnvironments
}

// saveEvalEnvironments persists the list through the user config file.
var saveEvalEnvironments = func(envs []config.EvalEnvironment) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		cfg = config.DefaultConfig()
	}
	cfg.EvalEnvironments = envs
	return config.SaveConfig(cfg)
}

// resolveEvalEnv maps a requested name to its entry. Empty or unknown names
// resolve to the zero entry, whose empty fields mean "default resolution".
func resolveEvalEnv(envs []config.EvalEnvironment, name string) config.EvalEnvironment {
	name = strings.TrimSpace(name)
	for _, e := range envs {
		if e.Name == name {
			return e
		}
	}
	return config.EvalEnvironment{Name: "local"}
}

// mergeEvalEnvironments appends one auto entry per ywai environment (dev, qa,
// personal, ...) to the registered list: their managed server port and
// opencode database are known from the profile, so they are eval targets
// without manual registration. Registered entries win on name collisions.
func mergeEvalEnvironments(registered []config.EvalEnvironment, profiles []envprofile.Profile) []config.EvalEnvironment {
	envs := registered
	for _, p := range profiles {
		dup := false
		for _, e := range envs {
			if e.Name == p.Name {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		envs = append(envs, config.EvalEnvironment{
			Name:      p.Name,
			ServerURL: fmt.Sprintf("http://127.0.0.1:%d", p.Port),
			DBPath:    filepath.Join(envprofile.Dirs(p)["data"], "opencode", "opencode.db"),
		})
	}
	return envs
}

// effectiveEvalEnvironments is the list the evals UI and the ?env= resolver
// see: registered environments plus the auto ywai-environment entries.
func effectiveEvalEnvironments() []config.EvalEnvironment {
	profiles, err := envprofile.List()
	if err != nil {
		return loadEvalEnvironments()
	}
	return mergeEvalEnvironments(loadEvalEnvironments(), profiles)
}

// evalServerURL returns the bench server for an env: explicit URL wins,
// otherwise the default OPENCODE_URL/probe resolution.
func evalServerURL(env config.EvalEnvironment) string {
	if u := strings.TrimSpace(env.ServerURL); u != "" {
		return strings.TrimRight(u, "/")
	}
	return opencodeURLForBench()
}

// evalDBPath returns the analytics database for an env: explicit path wins,
// otherwise the default location. Empty string preserves the historical
// call convention (LoadSessionAnalytics defaults it internally).
func evalDBPath(env config.EvalEnvironment) string {
	return strings.TrimSpace(env.DBPath)
}

// filterRunsByEnv keeps stored runs of one environment. Empty want means no
// filter (historical shape). Pre-environment runs carry "" and read as local.
func filterRunsByEnv(runs []evals.Run, want string) []evals.Run {
	want = strings.TrimSpace(want)
	if want == "" {
		return runs
	}
	out := make([]evals.Run, 0, len(runs))
	for _, r := range runs {
		env := r.Environment
		if env == "" {
			env = "local"
		}
		if env == want {
			out = append(out, r)
		}
	}
	return out
}

// GET /api/evals/environments — the configured list (implicit local alone
// when nothing is configured).
func (s *Server) handleEvalEnvironments(w http.ResponseWriter, r *http.Request) {
	envs := effectiveEvalEnvironments()
	if len(envs) == 0 {
		envs = []config.EvalEnvironment{{Name: "local"}}
	}
	writeJSON(w, http.StatusOK, map[string]any{"environments": envs})
}

// POST /api/evals/environments {name, serverUrl?, dbPath?} — upsert by name.
func (s *Server) handleSetEvalEnvironment(w http.ResponseWriter, r *http.Request) {
	var req config.EvalEnvironment
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid body: " + err.Error()})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 64 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "name is required (max 64 chars)"})
		return
	}
	req.ServerURL = strings.TrimSpace(req.ServerURL)
	req.DBPath = strings.TrimSpace(req.DBPath)

	envs := loadEvalEnvironments()
	replaced := false
	for i, e := range envs {
		if e.Name == req.Name {
			envs[i] = req
			replaced = true
		}
	}
	if !replaced {
		envs = append(envs, req)
	}
	if err := saveEvalEnvironments(envs); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"environments": envs})
}

// DELETE /api/evals/environments?name= — remove one (missing is 404).
func (s *Server) handleDeleteEvalEnvironment(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "name is required"})
		return
	}
	envs := loadEvalEnvironments()
	kept := make([]config.EvalEnvironment, 0, len(envs))
	found := false
	for _, e := range envs {
		if e.Name == name {
			found = true
			continue
		}
		kept = append(kept, e)
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "environment not found"})
		return
	}
	if err := saveEvalEnvironments(kept); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if len(kept) == 0 {
		kept = []config.EvalEnvironment{{Name: "local"}}
	}
	writeJSON(w, http.StatusOK, map[string]any{"environments": kept})
}
