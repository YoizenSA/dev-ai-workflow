package control

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

// McpCatalogEntry represents a single MCP server in the catalog.
type McpCatalogEntry struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Icon        string   `json:"icon"`
	Popular     bool     `json:"popular"`
	Type        string   `json:"type"`
	Command     []string `json:"command,omitempty"`
	URL         string   `json:"url,omitempty"`
	InstallCmd  string   `json:"installCmd,omitempty"`
	Tools       []string `json:"tools"`
	Docs        string   `json:"docs"`
}

// McpCatalogItem combines a catalog entry with its installed status.
type McpCatalogItem struct {
	McpCatalogEntry
	Installed bool   `json:"installed"`
	Enabled   bool   `json:"enabled"`
	Source    string `json:"source"`
}

// mcpConfigMu guards concurrent reads/writes to the config file.
var mcpConfigMu sync.Mutex

// getFullCatalog returns the custom MCP catalog.
func getFullCatalog() []McpCatalogEntry {
	return customMcpCatalog
}

// customMcpCatalog is our local catalog for MCPs not in the official registry.
var customMcpCatalog = []McpCatalogEntry{
	{
		ID:          "context7",
		Name:        "Context7",
		Description: "Up-to-date library documentation and code examples. Query any framework or library for current API docs.",
		Category:    "docs",
		Icon:        "DOC",
		Popular:     true,
		Type:        "remote",
		URL:         "https://mcp.context7.com/mcp",
		Tools:       []string{"query-docs", "resolve-library-id"},
		Docs:        "https://context7.com",
	},
	{
		ID:          "chrome-devtools",
		Name:        "Chrome DevTools",
		Description: "Inspect and debug web pages directly from your agent. Access DOM, console, network, and performance data.",
		Category:    "browser",
		Icon:        "BRW",
		Popular:     true,
		Type:        "local",
		Command:     []string{"npx", "-y", "@anthropic-ai/chrome-devtools-mcp"},
		InstallCmd:  "npx -y @anthropic-ai/chrome-devtools-mcp",
		Tools:       []string{"navigate", "screenshot", "evaluate-js", "get-dom", "get-console", "get-network"},
		Docs:        "https://github.com/anthropics/chrome-devtools-mcp",
	},
	{
		ID:          "playwright",
		Name:        "Playwright",
		Description: "Browser automation and E2E testing. Control Chromium, Firefox, and WebKit programmatically.",
		Category:    "testing",
		Icon:        "BRW",
		Popular:     true,
		Type:        "local",
		Command:     []string{"npx", "-y", "@anthropic-ai/playwright-mcp"},
		InstallCmd:  "npx -y @anthropic-ai/playwright-mcp",
		Tools:       []string{"browser-navigate", "browser-click", "browser-type", "browser-screenshot", "browser-evaluate", "browser-select"},
		Docs:        "https://github.com/anthropics/playwright-mcp",
	},
	{
		ID:          "engram",
		Name:        "Engram",
		Description: "Persistent memory system for AI agents. Store and recall observations across sessions with full-text search.",
		Category:    "memory",
		Icon:        "MEM",
		Popular:     true,
		Type:        "local",
		Command:     []string{"engram", "mcp"},
		InstallCmd:  "go install github.com/nahuelyoizen/engram/cmd/engram@latest",
		Tools:       []string{"mem-save", "mem-search", "mem-context", "mem-get-observation", "mem-session-summary", "mem-review"},
		Docs:        "https://github.com/nahuelyoizen/engram",
	},
	{
		ID:          "codegraph",
		Name:        "CodeGraph",
		Description: "Tree-sitter-parsed knowledge graph of your codebase. Sub-millisecond structural queries for symbols, callers, and call paths.",
		Category:    "code-analysis",
		Icon:        "COD",
		Popular:     true,
		Type:        "local",
		Command:     []string{"codegraph", "mcp"},
		InstallCmd:  "go install github.com/nahuelyoizen/codegraph/cmd/codegraph@latest",
		Tools:       []string{"codegraph-search", "codegraph-node", "codegraph-callers", "codegraph-callees", "codegraph-trace", "codegraph-impact", "codegraph-context", "codegraph-explore", "codegraph-files", "codegraph-status"},
		Docs:        "https://github.com/nahuelyoizen/codegraph",
	},
	{
		ID:          "git",
		Name:        "Git",
		Description: "Git repository operations. Read logs, diffs, branches, and commit history without leaving your agent.",
		Category:    "core",
		Icon:        "COR",
		Popular:     false,
		Type:        "local",
		Command:     []string{"npx", "-y", "@modelcontextprotocol/server-git"},
		InstallCmd:  "npx -y @modelcontextprotocol/server-git",
		Tools:       []string{"git-status", "git-log", "git-diff", "git-show", "git-blame", "git-branch-list", "git-commit-history"},
		Docs:        "https://github.com/modelcontextprotocol/servers/tree/main/src/git",
	},
	{
		ID:          "github",
		Name:        "GitHub",
		Description: "GitHub API integration. Manage repos, issues, PRs, and workflows directly from your agent.",
		Category:    "integration",
		Icon:        "OPS",
		Popular:     true,
		Type:        "local",
		Command:     []string{"npx", "-y", "@modelcontextprotocol/server-github"},
		InstallCmd:  "npx -y @modelcontextprotocol/server-github",
		Tools:       []string{"create-repo", "list-repos", "get-file-contents", "create-issue", "list-issues", "create-pull-request", "list-pull-requests"},
		Docs:        "https://github.com/modelcontextprotocol/servers/tree/main/src/github",
	},
	{
		ID:          "postgres",
		Name:        "PostgreSQL",
		Description: "Query and manage PostgreSQL databases. Execute SQL, inspect schemas, and analyze query performance.",
		Category:    "database",
		Icon:        "DB",
		Popular:     false,
		Type:        "local",
		Command:     []string{"npx", "-y", "@modelcontextprotocol/server-postgres"},
		InstallCmd:  "npx -y @modelcontextprotocol/server-postgres",
		Tools:       []string{"query", "list-tables", "describe-table", "explain-query", "list-databases"},
		Docs:        "https://github.com/modelcontextprotocol/servers/tree/main/src/postgres",
	},
	{
		ID:          "docker",
		Name:        "Docker",
		Description: "Manage Docker containers, images, volumes, and networks. Build, run, and inspect containers.",
		Category:    "devops",
		Icon:        "OPS",
		Popular:     false,
		Type:        "local",
		Command:     []string{"npx", "-y", "@modelcontextprotocol/server-docker"},
		InstallCmd:  "npx -y @modelcontextprotocol/server-docker",
		Tools:       []string{"list-containers", "create-container", "start-container", "stop-container", "remove-container", "list-images", "build-image"},
		Docs:        "https://github.com/modelcontextprotocol/servers/tree/main/src/docker",
	},
	{
		ID:          "kubernetes",
		Name:        "Kubernetes",
		Description: "Manage Kubernetes clusters, pods, services, and deployments. Inspect resources and troubleshoot issues.",
		Category:    "devops",
		Icon:        "OPS",
		Popular:     false,
		Type:        "local",
		Command:     []string{"npx", "-y", "@anthropic-ai/kubernetes-mcp"},
		InstallCmd:  "npx -y @anthropic-ai/kubernetes-mcp",
		Tools:       []string{"list-pods", "get-pod", "list-services", "list-deployments", "kubectl-exec", "describe-resource"},
		Docs:        "https://github.com/anthropics/kubernetes-mcp",
	},
	{
		ID:          "microsoft-learn",
		Name:        "Microsoft Learn",
		Description: "Search Microsoft documentation, Azure docs, and learn.microsoft.com content for up-to-date technical references.",
		Category:    "docs",
		Icon:        "DOC",
		Popular:     false,
		Type:        "remote",
		URL:         "https://learn.microsoft.com/api/mcp",
		Tools:       []string{"search-docs", "get-article", "list-articles", "get-api-reference"},
		Docs:        "https://learn.microsoft.com",
	},
	{
		ID:          "jam",
		Name:        "Jam",
		Description: "Browser extension for capturing bugs, screenshots, and console logs directly into your workflow.",
		Category:    "testing",
		Icon:        "TST",
		Popular:     false,
		Type:        "local",
		Command:     []string{"jam-mcp"},
		Tools:       []string{"capture-screenshot", "get-console-logs", "report-bug"},
		Docs:        "https://jam.dev",
	},
	{
		ID:          "sharptools",
		Name:        "SharpTools",
		Description: "C# code analysis and .NET development tools. Roslyn-based syntax analysis, ILSpy decompilation, and Git integration.",
		Category:    "dotnet",
		Icon:        "NET",
		Popular:     false,
		Type:        "local",
		Command:     []string{"sharptools"},
		Tools:       []string{"analyze-code", "decompile", "syntax-tree", "git-blame"},
		Docs:        "https://learn.microsoft.com/en-us/dotnet/csharp/roslyn-sdk/",
	},
	{
		ID:          "ywai-kanban",
		Name:        "ywai Kanban",
		Description: "Kanban board for tracking AI agent delegations and task progress in ywai workflows.",
		Category:    "project-management",
		Icon:        "PM",
		Popular:     false,
		Type:        "local",
		Command:     []string{"ywai", "serve", "--mcp-only"},
		Tools:       []string{"kanban_create_session", "kanban_create_delegation", "kanban_update_delegation", "kanban_get_board"},
		Docs:        "https://github.com/Yoizen/dev-ai-workflow",
	},
}

// registerMcpStoreRoutes registers MCP store API routes.
func (s *Server) registerMcpStoreRoutes() {
	s.mux.HandleFunc("GET /api/mcp/catalog", s.handleMcpCatalog)
	s.mux.HandleFunc("GET /api/mcp/installed", s.handleMcpInstalled)
	s.mux.HandleFunc("POST /api/mcp/install", s.handleMcpInstall)
	s.mux.HandleFunc("POST /api/mcp/uninstall", s.handleMcpUninstall)
	s.mux.HandleFunc("POST /api/mcp/toggle", s.handleMcpToggle)
	s.mux.HandleFunc("GET /api/mcp/status/{id}", s.handleMcpStatus)
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
			source := "custom"
			items[i] = McpCatalogItem{McpCatalogEntry: entry, Source: source}
		}
		writeJSON(w, http.StatusOK, items)
		return
	}

	items := make([]McpCatalogItem, len(fullCatalog))
	for i, entry := range fullCatalog {
		installed, enabled := false, true
		if cfg, ok := mcpConfig[entry.ID]; ok {
			if m, ok := cfg.(map[string]interface{}); ok {
				installed = true
				// If enabled field is missing, default to true
				if enabledVal, hasEnabled := m["enabled"]; hasEnabled {
					if e, ok := enabledVal.(bool); ok {
						enabled = e
					}
				}
			}
		}
		source := "custom"
		items[i] = McpCatalogItem{
			McpCatalogEntry: entry,
			Installed:       installed,
			Enabled:         enabled,
			Source:          source,
		}
	}

	writeJSON(w, http.StatusOK, items)
}

// handleMcpInstalled returns currently installed MCPs from opencode.json.
func (s *Server) handleMcpInstalled(w http.ResponseWriter, r *http.Request) {
	mcpConfig, err := readMcpConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to read config: %v", err)})
		return
	}

	writeJSON(w, http.StatusOK, mcpConfig)
}

// mcpInstallRequest is the POST body for install.
type mcpInstallRequest struct {
	ID string `json:"id"`
}

// mcpToggleRequest is the POST body for toggle.
type mcpToggleRequest struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
}

// handleMcpInstall installs an MCP server by adding it to opencode.json.
func (s *Server) handleMcpInstall(w http.ResponseWriter, r *http.Request) {
	var req mcpInstallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.ID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	// Find the catalog entry.
	fullCatalog := getFullCatalog()
	var entry *McpCatalogEntry
	for i := range fullCatalog {
		if fullCatalog[i].ID == req.ID {
			entry = &fullCatalog[i]
			break
		}
	}
	if entry == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("MCP %q not found in catalog", req.ID)})
		return
	}

	mcpConfig, err := readMcpConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to read config: %v", err)})
		return
	}

	// Build the new entry.
	newEntry := map[string]interface{}{
		"enabled": true,
		"type":    entry.Type,
	}
	if entry.Type == "local" && len(entry.Command) > 0 {
		newEntry["command"] = entry.Command
	}
	if entry.Type == "remote" && entry.URL != "" {
		newEntry["url"] = entry.URL
	}

	mcpConfig[entry.ID] = newEntry

	if err := writeMcpConfig(mcpConfig); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to write config: %v", err)})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("MCP %q installed successfully", entry.ID),
		"entry":   newEntry,
	})
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

// GetInstalledMcpIDs returns the IDs of currently installed MCPs.
// Exported for use by other packages.
func GetInstalledMcpIDs() ([]string, error) {
	mcpConfig, err := readMcpConfig()
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(mcpConfig))
	for id := range mcpConfig {
		ids = append(ids, id)
	}
	return ids, nil
}
