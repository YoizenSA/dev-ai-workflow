package control

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
)

// adoConfigMu guards concurrent reads/writes to the ADO plugin config.
var adoConfigMu sync.Mutex

// AdoProfile represents a single ADO profile configuration.
type AdoProfile struct {
	Org       string   `json:"org"`
	Project   string   `json:"project"`
	PatEnvVar string   `json:"patEnvVar"`
	Repos     []string `json:"repos"`
}

// AdoPluginConfig represents the full ADO plugin configuration.
type AdoPluginConfig struct {
	Enabled         bool                  `json:"enabled"`
	DefaultProfile  string                `json:"defaultProfile"`
	Profiles        map[string]AdoProfile `json:"profiles"`
}

// registerAdoConfigRoutes registers all ADO config API routes.
func (s *Server) registerAdoConfigRoutes() {
	s.mux.HandleFunc("GET /api/ado/config", s.handleAdoGetConfig)
	s.mux.HandleFunc("POST /api/ado/config", s.handleAdoSaveConfig)
	s.mux.HandleFunc("POST /api/ado/profile", s.handleAdoAddProfile)
	s.mux.HandleFunc("DELETE /api/ado/profile", s.handleAdoDeleteProfile)
	s.mux.HandleFunc("POST /api/ado/toggle", s.handleAdoToggle)
}

// readAdoConfig reads the ADO plugin section from opencode.json.
func readAdoConfig() (*AdoPluginConfig, error) {
	adoConfigMu.Lock()
	defer adoConfigMu.Unlock()

	path, err := configFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &AdoPluginConfig{
				Enabled:        false,
				DefaultProfile: "",
				Profiles:       map[string]AdoProfile{},
			}, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var full map[string]interface{}
	if err := json.Unmarshal(data, &full); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	plugins, ok := full["plugins"].(map[string]interface{})
	if !ok {
		return &AdoPluginConfig{
			Enabled:        false,
			DefaultProfile: "",
			Profiles:       map[string]AdoProfile{},
		}, nil
	}

	adoRaw, ok := plugins["@cioffinahuel/opencode-ado"].(map[string]interface{})
	if !ok {
		return &AdoPluginConfig{
			Enabled:        false,
			DefaultProfile: "",
			Profiles:       map[string]AdoProfile{},
		}, nil
	}

	// Marshal back and unmarshal into struct for clean parsing.
	adoBytes, err := json.Marshal(adoRaw)
	if err != nil {
		return nil, fmt.Errorf("marshaling ado section: %w", err)
	}

	cfg := &AdoPluginConfig{
		Enabled:        false,
		DefaultProfile: "",
		Profiles:       map[string]AdoProfile{},
	}
	if err := json.Unmarshal(adoBytes, cfg); err != nil {
		return nil, fmt.Errorf("parsing ado config: %w", err)
	}

	return cfg, nil
}

// writeAdoConfig writes the ADO plugin section back to opencode.json,
// preserving all other config keys.
func writeAdoConfig(cfg *AdoPluginConfig) error {
	adoConfigMu.Lock()
	defer adoConfigMu.Unlock()

	path, err := configFilePath()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Create new config with just the ADO plugin.
			full := map[string]interface{}{
				"plugins": map[string]interface{}{
					"@cioffinahuel/opencode-ado": cfg,
				},
			}
			out, marshalErr := json.MarshalIndent(full, "", "  ")
			if marshalErr != nil {
				return fmt.Errorf("marshaling config: %w", marshalErr)
			}
			if writeErr := os.WriteFile(path, out, 0644); writeErr != nil {
				return fmt.Errorf("writing config: %w", writeErr)
			}
			return nil
		}
		return fmt.Errorf("reading config: %w", err)
	}

	var full map[string]interface{}
	if err := json.Unmarshal(data, &full); err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	plugins, _ := full["plugins"].(map[string]interface{})
	if plugins == nil {
		plugins = map[string]interface{}{}
	}
	plugins["@cioffinahuel/opencode-ado"] = cfg
	full["plugins"] = plugins

	out, err := json.MarshalIndent(full, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, out, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

// handleAdoGetConfig returns the current ADO plugin config.
func (s *Server) handleAdoGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := readAdoConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to read config: %v", err)})
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// handleAdoSaveConfig saves the full ADO config.
func (s *Server) handleAdoSaveConfig(w http.ResponseWriter, r *http.Request) {
	var cfg AdoPluginConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if cfg.Profiles == nil {
		cfg.Profiles = map[string]AdoProfile{}
	}

	if err := writeAdoConfig(&cfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to write config: %v", err)})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "ADO config saved successfully",
		"config":  cfg,
	})
}

// adoProfileRequest is the POST/DELETE body for profile operations.
type adoProfileRequest struct {
	Name    string      `json:"name"`
	Profile AdoProfile  `json:"profile"`
}

// handleAdoAddProfile adds or updates a profile.
func (s *Server) handleAdoAddProfile(w http.ResponseWriter, r *http.Request) {
	var req adoProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "profile name is required"})
		return
	}

	if req.Profile.Org == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization is required"})
		return
	}

	if req.Profile.Project == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project is required"})
		return
	}

	if req.Profile.PatEnvVar == "" {
		req.Profile.PatEnvVar = "AZURE_DEVOPS_PAT"
	}

	if req.Profile.Repos == nil {
		req.Profile.Repos = []string{}
	}

	cfg, err := readAdoConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to read config: %v", err)})
		return
	}

	if cfg.Profiles == nil {
		cfg.Profiles = map[string]AdoProfile{}
	}
	cfg.Profiles[req.Name] = req.Profile

	// If this is the first profile, set it as default.
	if cfg.DefaultProfile == "" {
		cfg.DefaultProfile = req.Name
	}

	if err := writeAdoConfig(cfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to write config: %v", err)})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Profile %q saved successfully", req.Name),
		"config":  cfg,
	})
}

// handleAdoDeleteProfile removes a profile.
func (s *Server) handleAdoDeleteProfile(w http.ResponseWriter, r *http.Request) {
	var req adoProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "profile name is required"})
		return
	}

	cfg, err := readAdoConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to read config: %v", err)})
		return
	}

	if _, exists := cfg.Profiles[req.Name]; !exists {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("profile %q not found", req.Name)})
		return
	}

	delete(cfg.Profiles, req.Name)

	// If we deleted the default profile, pick another one.
	if cfg.DefaultProfile == req.Name {
		cfg.DefaultProfile = ""
		for name := range cfg.Profiles {
			cfg.DefaultProfile = name
			break
		}
	}

	if err := writeAdoConfig(cfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to write config: %v", err)})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Profile %q deleted successfully", req.Name),
		"config":  cfg,
	})
}

// handleAdoToggle enables or disables the ADO plugin.
func (s *Server) handleAdoToggle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	cfg, err := readAdoConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to read config: %v", err)})
		return
	}

	cfg.Enabled = req.Enabled

	if err := writeAdoConfig(cfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to write config: %v", err)})
		return
	}

	state := map[bool]string{true: "enabled", false: "disabled"}[req.Enabled]
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("ADO plugin %s", state),
		"config":  cfg,
	})
}

// GetAdoConfig is an exported helper to get the ADO config from other packages.
func GetAdoConfig() (*AdoPluginConfig, error) {
	return readAdoConfig()
}
