package control

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/plugins"
)

// adoConfigMu guards concurrent reads/writes of ado.json.
var adoConfigMu sync.Mutex

// adoProfile mirrors the profile shape the `ado` CLI reads from opencode.json.
// The PAT itself is never stored here — only the name of the env var that will
// hold it at runtime. See adoPATStatus / adoSavePAT for PAT handling.
type adoProfile struct {
	Org       string   `json:"org"`
	Project   string   `json:"project"`
	PatEnvVar string   `json:"patEnvVar"`
	Repos     []string `json:"repos"`
	Default   bool     `json:"default,omitempty"`
}

// adoConfig is stored in ~/.config/opencode/ado.json. The `ado` CLI discovers
// profiles here WITHOUT registering the in-process plugin, so its 22 tools are
// never loaded into the agent's context.
type adoConfig struct {
	DefaultProfile string                `json:"defaultProfile"`
	Profiles       map[string]adoProfile `json:"profiles"`
}

// adoProfileRequest is the body for add/update/delete profile endpoints.
type adoProfileRequest struct {
	Name    string     `json:"name"`
	Profile adoProfile `json:"profile"`
}

// ─── Config helpers ───────────────────────────────────────────────────────

// adoConfigPath returns ~/.config/opencode/ado.json.
func adoConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".config", "opencode", "ado.json"), nil
}

// readAdoConfig reads profiles from ado.json. Returns an empty config (not an
// empty config (not an error) when the file or key is absent.
func readAdoConfig() (*adoConfig, error) {
	adoConfigMu.Lock()
	defer adoConfigMu.Unlock()

	path, err := adoConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &adoConfig{Profiles: map[string]adoProfile{}}, nil
		}
		return nil, fmt.Errorf("reading ado config: %w", err)
	}

	var cfg adoConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing ado config: %w", err)
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]adoProfile{}
	}
	// Normalize the default flag onto the default profile so the UI can render
	// the badge even if the file only set defaultProfile.
	for name, p := range cfg.Profiles {
		p.Default = name == cfg.DefaultProfile
		cfg.Profiles[name] = p
	}
	return &cfg, nil
}

// writeAdoConfig writes the ADO profile config to ~/.config/opencode/ado.json.
func writeAdoConfig(cfg *adoConfig) error {
	adoConfigMu.Lock()
	defer adoConfigMu.Unlock()

	if cfg == nil {
		cfg = &adoConfig{}
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]adoProfile{}
	}

	path, err := adoConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

// adoPATPath returns ~/.azure-devops-cli/pat — where the `ado` CLI stores its
// PAT (chmod 0600). The CLI reads it as a fallback when AZURE_DEVOPS_PAT is
// unset.
func adoPATPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".azure-devops-cli", "pat"), nil
}

// ─── Route registration ───────────────────────────────────────────────────

// registerAdoConfigRoutes registers the Azure DevOps config API. The config
// lives in ~/.config/opencode/ado.json (profiles) plus a PAT
// file at ~/.azure-devops-cli/pat.
func (s *Server) registerAdoConfigRoutes() {
	s.mux.HandleFunc("GET /api/ado/config", s.handleAdoGetConfig)
	s.mux.HandleFunc("POST /api/ado/config", s.handleAdoSaveConfig)
	s.mux.HandleFunc("POST /api/ado/profile", s.handleAdoUpsertProfile)
	s.mux.HandleFunc("DELETE /api/ado/profile", s.handleAdoDeleteProfile)
	s.mux.HandleFunc("GET /api/ado/pat-status", s.handleAdoPATStatus)
	s.mux.HandleFunc("POST /api/ado/pat", s.handleAdoSavePAT)
	s.mux.HandleFunc("GET /api/ado/cli-status", s.handleAdoCLIStatus)
	s.mux.HandleFunc("POST /api/ado/cli-update", s.handleAdoCLIUpdate)
}

// ─── Handlers ─────────────────────────────────────────────────────────────

// handleAdoGetConfig returns the current ADO config (profiles + default).
func (s *Server) handleAdoGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := readAdoConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// handleAdoSaveConfig replaces the whole ADO config.
func (s *Server) handleAdoSaveConfig(w http.ResponseWriter, r *http.Request) {
	var cfg adoConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	// Default patEnvVar so the CLI finds the PAT without extra config.
	for name, p := range cfg.Profiles {
		if strings.TrimSpace(p.PatEnvVar) == "" {
			p.PatEnvVar = "AZURE_DEVOPS_PAT"
		}
		p.Default = name == cfg.DefaultProfile
		cfg.Profiles[name] = p
	}
	if err := writeAdoConfig(&cfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"config": cfg})
}

// handleAdoUpsertProfile adds or updates a single profile.
func (s *Server) handleAdoUpsertProfile(w http.ResponseWriter, r *http.Request) {
	var req adoProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "profile name is required"})
		return
	}
	if strings.TrimSpace(req.Profile.Org) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization is required"})
		return
	}
	if strings.TrimSpace(req.Profile.Project) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project is required"})
		return
	}

	cfg, err := readAdoConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	p := req.Profile
	if strings.TrimSpace(p.PatEnvVar) == "" {
		p.PatEnvVar = "AZURE_DEVOPS_PAT"
	}
	if p.Repos == nil {
		p.Repos = []string{}
	}
	// First profile becomes the default automatically.
	if cfg.DefaultProfile == "" && len(cfg.Profiles) == 0 {
		cfg.DefaultProfile = name
	}
	p.Default = name == cfg.DefaultProfile
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]adoProfile{}
	}
	cfg.Profiles[name] = p

	if err := writeAdoConfig(cfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"config": cfg})
}

// handleAdoDeleteProfile removes a profile and recomputes the default if needed.
func (s *Server) handleAdoDeleteProfile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "profile name is required"})
		return
	}

	cfg, err := readAdoConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if _, ok := cfg.Profiles[name]; !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "profile not found"})
		return
	}
	delete(cfg.Profiles, name)

	// Pick a new default if we removed the current one.
	if cfg.DefaultProfile == name {
		cfg.DefaultProfile = ""
		for n := range cfg.Profiles {
			cfg.DefaultProfile = n
			break
		}
	}
	for n, p := range cfg.Profiles {
		p.Default = n == cfg.DefaultProfile
		cfg.Profiles[n] = p
	}

	if err := writeAdoConfig(cfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"config": cfg})
}

// handleAdoPATStatus reports whether a PAT is available, without exposing it.
// source is "env" (AZURE_DEVOPS_PAT set) or "file" (~/.azure-devops-cli/pat).
func (s *Server) handleAdoPATStatus(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{"hasPat": false, "source": "none"}

	if v := os.Getenv("AZURE_DEVOPS_PAT"); strings.TrimSpace(v) != "" {
		resp["hasPat"] = true
		resp["source"] = "env"
		writeJSON(w, http.StatusOK, resp)
		return
	}
	p, err := adoPATPath()
	if err == nil {
		if data, err := os.ReadFile(p); err == nil && strings.TrimSpace(string(data)) != "" {
			resp["hasPat"] = true
			resp["source"] = "file"
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleAdoSavePAT writes the PAT to ~/.azure-devops-cli/pat (chmod 0600).
func (s *Server) handleAdoSavePAT(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PAT string `json:"pat"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	pat := strings.TrimSpace(req.PAT)
	if pat == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "pat is required"})
		return
	}

	p, err := adoPATPath()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("creating pat dir: %v", err)})
		return
	}
	if err := os.WriteFile(p, []byte(pat), 0o600); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("writing pat: %v", err)})
		return
	}
	// Best-effort chmod 0600 (already set on create, but enforce on overwrite).
	_ = os.Chmod(p, 0o600)

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleAdoCLIStatus reports whether the `ado` CLI is installed, its version,
// the latest version published to npm, and whether an update is available.
// Mirrors the shape of /api/version: on registry failure `latest` is null and
// `updateAvailable` is false (non-fatal). The comparison reuses
// isNewerVersion/parseSemver from server.go (same package).
func (s *Server) handleAdoCLIStatus(w http.ResponseWriter, r *http.Request) {
	version, installed := plugins.AdoCLIInfo()

	resp := map[string]any{
		"installed":       installed,
		"version":         version,
		"latest":          nil,
		"updateAvailable": false,
	}

	latest, err := plugins.AdoCLILatestVersion()
	if err != nil {
		// Offline / npm missing — keep latest null, surface the reason.
		resp["error"] = err.Error()
		writeJSON(w, http.StatusOK, resp)
		return
	}
	resp["latest"] = latest
	resp["updateAvailable"] = installed && latest != "" && isNewerVersion(latest, version)
	writeJSON(w, http.StatusOK, resp)
}

// handleAdoCLIUpdate runs `npm i -g @cioffinahuel/opencode-ado` synchronously
// and returns the fresh CLI status. Unlike /api/update (ywai self-update), this
// does NOT kill or restart the server — npm install is independent of this
// process — so it can run inline and the frontend just refreshes its state.
func (s *Server) handleAdoCLIUpdate(w http.ResponseWriter, r *http.Request) {
	if err := plugins.InstallAdoCLI(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"installed":       false,
			"version":         "",
			"latest":          nil,
			"updateAvailable": false,
			"error":           err.Error(),
		})
		return
	}
	version, installed := plugins.AdoCLIInfo()
	resp := map[string]any{
		"installed":       installed,
		"version":         version,
		"latest":          nil,
		"updateAvailable": false,
	}
	if latest, err := plugins.AdoCLILatestVersion(); err == nil {
		resp["latest"] = latest
		resp["updateAvailable"] = installed && latest != "" && isNewerVersion(latest, version)
	}
	writeJSON(w, http.StatusOK, resp)
}

// init guards against the log import being dropped if no handler logs directly.
var _ = log.Printf
