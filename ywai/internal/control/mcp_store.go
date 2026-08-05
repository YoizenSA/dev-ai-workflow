package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/mcp"
)

// McpEnvSpec is the JSON shape of a credential the install UI must collect.
type McpEnvSpec struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Secret      bool   `json:"secret"`
}

// McpCatalogEntry represents a single MCP server in the catalog.
// Fields mirror mcp.CatalogEntry so the store UI and install pipeline share
// one source of truth (internal/mcp/catalog.go).
type McpCatalogEntry struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Category    string       `json:"category"`
	Icon        string       `json:"icon"`
	Popular     bool         `json:"popular"`
	Type        string       `json:"type"`
	Command     []string     `json:"command,omitempty"`
	URL         string       `json:"url,omitempty"`
	InstallCmd  string       `json:"installCmd,omitempty"`
	RequiredEnv []McpEnvSpec `json:"requiredEnv,omitempty"`
	Tools       []string     `json:"tools"`
	Docs        string       `json:"docs"`
}

// McpCatalogItem combines a catalog entry with its installed status.
type McpCatalogItem struct {
	McpCatalogEntry
	Installed     bool   `json:"installed"`
	Enabled       bool   `json:"enabled"`
	Source        string `json:"source"`
	Status        string `json:"status"`
	StatusLabel   string `json:"statusLabel"`
	StatusMessage string `json:"statusMessage,omitempty"`
	FixAction     string `json:"fixAction,omitempty"`
}

// mcpConfigMu guards concurrent reads/writes to the config file.
var mcpConfigMu sync.Mutex

// getFullCatalog returns the canonical MCP catalog used by install + UI.
// ponytail: one catalog — never re-list servers here; edit internal/mcp/catalog.go.
func getFullCatalog() []McpCatalogEntry {
	src := mcp.Catalog()
	out := make([]McpCatalogEntry, 0, len(src))
	for _, e := range src {
		out = append(out, catalogEntryFromMCP(e))
	}
	return out
}

func catalogEntryFromMCP(e mcp.CatalogEntry) McpCatalogEntry {
	req := make([]McpEnvSpec, len(e.RequiredEnv))
	for i, s := range e.RequiredEnv {
		req[i] = McpEnvSpec{
			Name:        s.Name,
			Description: s.Description,
			Required:    s.Required,
			Secret:      s.Secret,
		}
	}
	return McpCatalogEntry{
		ID:          e.ID,
		Name:        e.Name,
		Description: e.Description,
		Category:    e.Category,
		Icon:        e.Icon,
		Popular:     e.Popular,
		Type:        e.Type,
		Command:     e.Command,
		URL:         e.URL,
		InstallCmd:  e.InstallCmd,
		RequiredEnv: req,
		Tools:       e.Tools,
		Docs:        e.Docs,
	}
}

// registerMcpStoreRoutes registers MCP store API routes.
func (s *Server) registerMcpStoreRoutes() {
	s.mux.HandleFunc("GET /api/mcp/catalog", s.handleMcpCatalog)
	s.mux.HandleFunc("GET /api/mcp/installed", s.handleMcpInstalled)
	s.mux.HandleFunc("POST /api/mcp/install", s.handleMcpInstall)
	s.mux.HandleFunc("GET /api/mcp/install/{id}", s.handleMcpInstallStatus)
	s.mux.HandleFunc("POST /api/mcp/uninstall", s.handleMcpUninstall)
	s.mux.HandleFunc("POST /api/mcp/toggle", s.handleMcpToggle)
	s.mux.HandleFunc("GET /api/mcp/status/{id}", s.handleMcpStatus)
	s.mux.HandleFunc("GET /api/mcp/health", s.handleMcpHealth)
}

// handleMcpCatalog returns the full catalog with installed/enabled status.
func (s *Server) handleMcpCatalog(w http.ResponseWriter, r *http.Request) {
	// Get catalog.
	fullCatalog := getFullCatalog()

	mcpConfig, err := readMcpConfig()
	if err != nil {
		log.Printf("mcp catalog: error reading config: %v", err)
		// Return catalog without installed status on error.
		items := make([]McpCatalogItem, len(fullCatalog))
		for i, entry := range fullCatalog {
			items[i] = McpCatalogItem{McpCatalogEntry: entry, Source: "catalog"}
		}
		writeJSON(w, http.StatusOK, items)
		return
	}

	items := make([]McpCatalogItem, len(fullCatalog))
	for i, entry := range fullCatalog {
		installed, enabled := false, true
		var cfg map[string]interface{}
		if rawCfg, ok := mcpConfig[entry.ID]; ok {
			if m, ok := rawCfg.(map[string]interface{}); ok {
				cfg = m
				installed = true
				// If enabled field is missing, default to true
				if enabledVal, hasEnabled := m["enabled"]; hasEnabled {
					if e, ok := enabledVal.(bool); ok {
						enabled = e
					}
				}
			}
		}
		status, label, message, action := mcpCatalogStatus(entry, installed, enabled, cfg)
		items[i] = McpCatalogItem{
			McpCatalogEntry: entry,
			Installed:       installed,
			Enabled:         enabled,
			Source:          "catalog",
			Status:          status,
			StatusLabel:     label,
			StatusMessage:   message,
			FixAction:       action,
		}
	}

	writeJSON(w, http.StatusOK, items)
}

func mcpCatalogStatus(entry McpCatalogEntry, installed, enabled bool, cfg map[string]interface{}) (string, string, string, string) {
	if !installed {
		return "available", "Available", "", "install"
	}
	if !enabled {
		return "disabled", "Disabled", "This MCP is configured but disabled in opencode.json.", "enable"
	}

	if entry.Type == "local" {
		command := entry.Command
		if rawCommand, ok := cfg["command"].([]string); ok && len(rawCommand) > 0 {
			command = rawCommand
		} else if rawCommand, ok := cfg["command"].([]interface{}); ok && len(rawCommand) > 0 {
			command = commandFromInterfaceSlice(rawCommand)
		}
		if entry.ID == "playwright" && commandContains(command, "@anthropic-ai/playwright-mcp") {
			return "connection_error", "Connection error", "This Playwright MCP entry uses the deprecated @anthropic-ai package. Reinstall it to use @playwright/mcp@latest.", "reinstall"
		}
		if len(command) == 0 {
			return "connection_error", "Configuration issue", "Local MCP has no command configured.", "reinstall"
		}
		if _, err := exec.LookPath(command[0]); err != nil {
			return "missing_executable", "Missing executable", fmt.Sprintf("%q is not available in $PATH.", command[0]), "install_dependency"
		}
	}

	return "connected", "Connected", "Configuration is enabled and prerequisites are available.", ""
}

func commandContains(command []string, needle string) bool {
	for _, part := range command {
		if strings.Contains(part, needle) {
			return true
		}
	}
	return false
}

func commandFromInterfaceSlice(values []interface{}) []string {
	command := make([]string, 0, len(values))
	for _, value := range values {
		if s, ok := value.(string); ok && s != "" {
			command = append(command, s)
		}
	}
	return command
}

// handleMcpInstalled returns currently installed MCPs.
// Supports ?project_dir=<path> for merged global+project-local config.
func (s *Server) handleMcpInstalled(w http.ResponseWriter, r *http.Request) {
	projectDir := r.URL.Query().Get("project_dir")
	if projectDir != "" {
		mcpConfig, err := readMergedMcpConfig(projectDir)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to read config: %v", err)})
			return
		}
		writeJSON(w, http.StatusOK, mcpConfig)
		return
	}

	mcpConfig, err := readMcpConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to read config: %v", err)})
		return
	}

	writeJSON(w, http.StatusOK, mcpConfig)
}

// mcpInstallRequest is the POST body for install.
type mcpInstallRequest struct {
	ID          string            `json:"id"`
	TargetAgent string            `json:"target_agent"`
	Credentials map[string]string `json:"credentials"`
	ProjectDir  string            `json:"project_dir,omitempty"`
}

// mcpInstallResponse is the 202 body returned by handleMcpInstall.
type mcpInstallResponse struct {
	InstallID   string `json:"install_id"`
	StatusURL   string `json:"status_url"`
	WSChannel   string `json:"ws_channel"`
	EntryID     string `json:"entry_id"`
	TargetAgent string `json:"target_agent"`
}

// mcpErrorResponse is the JSON shape used by the install / status
// handlers for non-2xx outcomes. Code is the machine-readable identifier
// ("install_in_progress", "missing_credentials", ...). Required lists
// the names of missing required env vars for the 422 path. ExistingID
// is the in-progress job's id for the 409 path.
type mcpErrorResponse struct {
	Error      string   `json:"error"`
	Code       string   `json:"code,omitempty"`
	Required   []string `json:"required,omitempty"`
	ExistingID string   `json:"existing_id,omitempty"`
}

// mcpToggleRequest is the POST body for toggle.
type mcpToggleRequest struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
}

// handleMcpInstall enqueues an MCP install job. The actual install runs
// in a background goroutine driven by the JobManager; the response is
// always 202 on success so the client can poll /api/mcp/install/{id} or
// subscribe to the WS channel for progress. Validation errors
// (unknown id, bad target, missing required creds) are still returned
// synchronously.
func (s *Server) handleMcpInstall(w http.ResponseWriter, r *http.Request) {
	var req mcpInstallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, mcpErrorResponse{Error: "invalid request body: " + err.Error()})
		return
	}

	if req.ID == "" {
		writeJSON(w, http.StatusBadRequest, mcpErrorResponse{Error: "id is required"})
		return
	}

	entry, ok := mcp.CatalogByID(req.ID)
	if !ok {
		writeJSON(w, http.StatusNotFound, mcpErrorResponse{Error: "unknown MCP id: " + req.ID})
		return
	}

	target := req.TargetAgent
	if target == "" {
		target = "opencode"
	}
	if target != "opencode" && target != "pi" && target != "claude-code" && target != "omp" {
		writeJSON(w, http.StatusBadRequest, mcpErrorResponse{Error: "invalid target_agent: " + target})
		return
	}

	missing := mcp.ValidateCreds(entry.RequiredEnv, req.Credentials)
	if len(missing) > 0 {
		names := make([]string, 0, len(missing))
		for _, s := range missing {
			names = append(names, s.Name)
		}
		writeJSON(w, http.StatusUnprocessableEntity, mcpErrorResponse{
			Error:    "missing_credentials",
			Code:     "missing_credentials",
			Required: names,
		})
		return
	}

	job, err := s.jobs.Start(r.Context(), entry, target, req.Credentials)
	if err != nil {
		if errors.Is(err, mcp.ErrJobInProgress) {
			existingID := ""
			var jpErr *mcp.JobInProgressError
			if errors.As(err, &jpErr) {
				existingID = jpErr.JobID
			}
			writeJSON(w, http.StatusConflict, mcpErrorResponse{
				Error:      "install_in_progress",
				Code:       "install_in_progress",
				ExistingID: existingID,
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, mcpErrorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusAccepted, mcpInstallResponse{
		InstallID:   job.ID,
		StatusURL:   "/api/mcp/install/" + job.ID,
		WSChannel:   "mcp-install",
		EntryID:     entry.ID,
		TargetAgent: target,
	})
}

// handleMcpInstallStatus is the GET /api/mcp/install/{id} polling
// endpoint. It returns the serialized *mcp.Job (which includes state,
// progress, result, and error). Unknown ids produce 404.
func (s *Server) handleMcpInstallStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	job, ok := s.jobs.Get(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, mcpErrorResponse{Error: "unknown install id: " + id})
		return
	}
	writeJSON(w, http.StatusOK, job)
}

// handleMcpUninstall removes an MCP server from opencode.json.
func (s *Server) handleMcpUninstall(w http.ResponseWriter, r *http.Request) {
	var req mcpInstallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.ID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	mcpConfig, err := readMcpConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to read config: %v", err)})
		return
	}

	if _, ok := mcpConfig[req.ID]; !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("MCP %q is not installed", req.ID)})
		return
	}

	delete(mcpConfig, req.ID)

	if err := writeMcpConfig(mcpConfig); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to write config: %v", err)})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("MCP %q uninstalled successfully", req.ID),
	})
}

// handleMcpToggle enables or disables an MCP server.
func (s *Server) handleMcpToggle(w http.ResponseWriter, r *http.Request) {
	var req mcpToggleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.ID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	mcpConfig, err := readMcpConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to read config: %v", err)})
		return
	}

	entry, ok := mcpConfig[req.ID]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("MCP %q is not installed", req.ID)})
		return
	}

	m, ok := entry.(map[string]interface{})
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "invalid MCP config entry"})
		return
	}

	m["enabled"] = req.Enabled
	mcpConfig[req.ID] = m

	if err := writeMcpConfig(mcpConfig); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to write config: %v", err)})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("MCP %q %s", req.ID, map[bool]string{true: "enabled", false: "disabled"}[req.Enabled]),
		"entry":   m,
	})
}

// handleMcpStatus returns the status of a specific MCP server.
func (s *Server) handleMcpStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	// Find in catalog.
	fullCatalog := getFullCatalog()
	var catalogEntry *McpCatalogEntry
	for i := range fullCatalog {
		if fullCatalog[i].ID == id {
			catalogEntry = &fullCatalog[i]
			break
		}
	}

	mcpConfig, err := readMcpConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to read config: %v", err)})
		return
	}

	configEntry, installed := mcpConfig[id]
	enabled := true
	var configMap map[string]interface{}
	if m, ok := configEntry.(map[string]interface{}); ok {
		configMap = m
		// If enabled field is missing, default to true
		if enabledVal, hasEnabled := m["enabled"]; hasEnabled {
			if e, ok := enabledVal.(bool); ok {
				enabled = e
			}
		}
	}

	result := map[string]interface{}{
		"id":        id,
		"installed": installed,
		"enabled":   enabled,
		"config":    configMap,
	}
	if catalogEntry != nil {
		result["catalog"] = catalogEntry
	}

	writeJSON(w, http.StatusOK, result)
}

// ─── Health check ─────────────────────────────────────────────────

// mcpHealthItem represents the health status of a single MCP server.
type mcpHealthItem struct {
	ID        string `json:"id"`
	Status    string `json:"status"` // "healthy", "unhealthy", "unknown"
	LatencyMs int64  `json:"latency_ms,omitempty"`
	Error     string `json:"error,omitempty"`
}

// mcpHealthResponse is the response for GET /api/mcp/health.
type mcpHealthResponse struct {
	Servers []mcpHealthItem `json:"servers"`
}

// handleMcpHealth checks the health of all installed MCP servers.
func (s *Server) handleMcpHealth(w http.ResponseWriter, r *http.Request) {
	mcpConfig, err := readMcpConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, mcpErrorResponse{Error: err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	type result struct {
		item mcpHealthItem
	}
	ch := make(chan result, len(mcpConfig))

	for id := range mcpConfig {
		go func(id string) {
			ch <- result{checkMcpHealth(ctx, id)}
		}(id)
	}

	items := make([]mcpHealthItem, 0, len(mcpConfig))
	for range mcpConfig {
		items = append(items, (<-ch).item)
	}

	writeJSON(w, http.StatusOK, mcpHealthResponse{Servers: items})
}

// checkMcpHealth checks health for a single MCP server by ID.
func checkMcpHealth(ctx context.Context, id string) mcpHealthItem {
	start := time.Now()

	// Find the catalog entry to determine type and check params.
	var entryType string
	var command []string
	var url string

	if ce, ok := mcp.CatalogByID(id); ok {
		entryType = ce.Type
		command = ce.Command
		url = ce.URL
	}

	item := mcpHealthItem{ID: id}

	switch entryType {
	case "local":
		item.Status = "healthy"
		if len(command) == 0 {
			item.Status = "unknown"
			break
		}
		if _, err := exec.LookPath(command[0]); err != nil {
			item.Status = "unhealthy"
			item.Error = fmt.Sprintf("binary %q not found: %v", command[0], err)
		}
		item.LatencyMs = time.Since(start).Milliseconds()

	case "remote":
		if url == "" {
			item.Status = "unknown"
			break
		}
		checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		req, reqErr := http.NewRequestWithContext(checkCtx, http.MethodHead, url, nil)
		if reqErr != nil {
			item.Status = "unhealthy"
			item.Error = reqErr.Error()
			item.LatencyMs = time.Since(start).Milliseconds()
			break
		}
		resp, doErr := http.DefaultClient.Do(req)
		item.LatencyMs = time.Since(start).Milliseconds()
		if doErr != nil {
			item.Status = "unhealthy"
			item.Error = doErr.Error()
			break
		}
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			item.Status = "healthy"
		} else {
			item.Status = "unhealthy"
			item.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}

	default:
		item.Status = "unknown"
		item.LatencyMs = time.Since(start).Milliseconds()
	}

	return item
}

// writeJSON is a local helper to write JSON responses with CORS headers.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("Error writing JSON response: %v", err)
	}
}

// configFilePath returns the path to opencode.json.
func configFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "opencode", "opencode.json"), nil
}

// readMcpConfig reads the mcp section from opencode.json.
// Returns a map of MCP ID -> config entry.
func readMcpConfig() (map[string]interface{}, error) {
	mcpConfigMu.Lock()
	defer mcpConfigMu.Unlock()

	path, err := configFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]interface{}{}, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var full map[string]interface{}
	if err := json.Unmarshal(data, &full); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	mcpSection, ok := full["mcp"].(map[string]interface{})
	if !ok {
		return map[string]interface{}{}, nil
	}

	return mcpSection, nil
}

// writeMcpConfig writes the mcp section back to opencode.json,
// preserving all other config keys.
func writeMcpConfig(mcp map[string]interface{}) error {
	mcpConfigMu.Lock()
	defer mcpConfigMu.Unlock()

	path, err := configFilePath()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	var full map[string]interface{}
	if err := json.Unmarshal(data, &full); err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	full["mcp"] = mcp

	out, err := json.MarshalIndent(full, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, out, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

// projectMcpConfigFilePath returns the path to the project-local MCP config file.
func projectMcpConfigFilePath(projectDir string) string {
	return filepath.Join(projectDir, ".opencode", "mcp.json")
}

// readProjectMcpConfig reads the mcpServers section from a project-local
// .opencode/mcp.json file. Returns an empty map if the file doesn't exist.
func readProjectMcpConfig(projectDir string) (map[string]interface{}, error) {
	path := projectMcpConfigFilePath(projectDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]interface{}{}, nil
		}
		return nil, fmt.Errorf("reading project mcp config %s: %w", path, err)
	}

	var mcpSection map[string]interface{}
	if err := json.Unmarshal(data, &mcpSection); err != nil {
		return nil, fmt.Errorf("parsing project mcp config %s: %w", path, err)
	}
	// Handle the case where the file has a top-level "mcpServers" key.
	if servers, ok := mcpSection["mcpServers"]; ok {
		if s, ok := servers.(map[string]interface{}); ok {
			return s, nil
		}
	}
	return mcpSection, nil
}

// readMergedMcpConfig merges global and project-local MCP config.
// Project-local servers override global servers with the same ID.
// Returns the merged map and a map of scope (id -> "global" | "project").
func readMergedMcpConfig(projectDir string) (map[string]interface{}, error) {
	global, err := readMcpConfig()
	if err != nil {
		return nil, err
	}

	result := make(map[string]interface{}, len(global))
	for id, entry := range global {
		result[id] = entry
	}

	if projectDir != "" {
		project, err := readProjectMcpConfig(projectDir)
		if err != nil {
			return nil, err
		}
		for id, entry := range project {
			result[id] = entry
		}
	}

	return result, nil
}
