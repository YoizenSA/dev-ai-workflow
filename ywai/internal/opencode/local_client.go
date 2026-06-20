package opencode

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// resolveOpencodeBin finds the opencode executable, trying Windows extensions
// so the CLI works when opencode is installed as opencode.cmd / opencode.exe
// (npm/bun shims) and not just a bare "opencode" on PATH.
func resolveOpencodeBin() string {
	candidates := []string{"opencode"}
	if runtime.GOOS == "windows" {
		// Prefer a real .exe (direct exec); fall back to npm/bun shims.
		candidates = []string{"opencode.exe", "opencode.cmd", "opencode.bat", "opencode.ps1", "opencode"}
	}
	for _, name := range candidates {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return "opencode"
}

// runOpencodeModels runs `opencode models` cross-platform and returns its
// stdout. On Windows a .cmd/.bat shim is run via `cmd /c` (and .ps1 via
// PowerShell) so the model list — including free models the HTTP API omits —
// is actually produced. The error is returned, not swallowed, so callers can
// log why they fell back to the HTTP/config source.
func runOpencodeModels(ctx context.Context) ([]byte, error) {
	bin := resolveOpencodeBin()
	lower := strings.ToLower(bin)
	switch {
	case runtime.GOOS == "windows" && (strings.HasSuffix(lower, ".cmd") || strings.HasSuffix(lower, ".bat")):
		return exec.CommandContext(ctx, "cmd", "/c", bin, "models").Output()
	case runtime.GOOS == "windows" && strings.HasSuffix(lower, ".ps1"):
		return exec.CommandContext(ctx, "powershell", "-NoProfile", "-File", bin, "models").Output()
	default:
		return exec.CommandContext(ctx, bin, "models").Output()
	}
}

// LocalClient implements Client by reading opencode config files directly.
type LocalClient struct {
	opencodeConfig string // path to opencode.json
	agentsDir      string // path to agents directory
	// useCLI controls whether ListModels tries 'opencode models' first.
	// True for the auto-detected client (production), false for the
	// path-parameterized client used in unit tests.
	useCLI bool
}

// NewLocalClient creates a LocalClient with auto-detected paths
// based on $HOME/.config/opencode/.
func NewLocalClient() *LocalClient {
	home, _ := os.UserHomeDir()
	return &LocalClient{
		opencodeConfig: filepath.Join(home, ".config", "opencode", "opencode.json"),
		agentsDir:      filepath.Join(home, ".config", "opencode", "agents"),
		useCLI:         true,
	}
}

// NewLocalClientWithPaths creates a LocalClient with explicit paths (for testing).
func NewLocalClientWithPaths(configPath, agentsDir string) *LocalClient {
	return &LocalClient{
		opencodeConfig: configPath,
		agentsDir:      agentsDir,
		useCLI:         false, // tests control the source via the config file
	}
}

// ListAgents reads agent profiles from the opencode.json config. Agents are
// defined under the top-level "agent" key as a map of name -> definition, not
// as files in a directory (the agentsDir is a legacy fallback that is usually
// empty in modern opencode setups).
func (c *LocalClient) ListAgents(_ context.Context) ([]AgentInfo, error) {
	// Primary source: the "agent" section of opencode.json.
	data, err := os.ReadFile(c.opencodeConfig)
	if err == nil {
		var config struct {
			Agent map[string]interface{} `json:"agent"`
		}
		if json.Unmarshal(data, &config) == nil && len(config.Agent) > 0 {
			agents := make([]AgentInfo, 0, len(config.Agent))
			for name := range config.Agent {
				agents = append(agents, AgentInfo{ID: name, Name: name})
			}
			// Stable order so the dropdown doesn't reshuffle between calls.
			sort.Slice(agents, func(i, j int) bool { return agents[i].ID < agents[j].ID })
			return agents, nil
		}
	}

	// Legacy fallback: agent files in a directory.
	entries, err := os.ReadDir(c.agentsDir)
	if err != nil {
		return nil, nil //nolint:nilerr // match current behavior: empty on error
	}

	agents := make([]AgentInfo, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".txt") && !strings.HasSuffix(name, ".json") {
			continue
		}
		id := strings.TrimSuffix(name, ".md")
		id = strings.TrimSuffix(id, ".txt")
		id = strings.TrimSuffix(id, ".json")
		agents = append(agents, AgentInfo{ID: id, Name: id})
	}
	return agents, nil
}

// ListModels returns available models. It prefers the opencode CLI
// ('opencode models'), which lists every model the runtime knows about
// (including connected providers and free models). If the CLI is not
// available, it falls back to reading the static opencode.json config.
func (c *LocalClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	// Primary source: the opencode CLI — lists all runtime models (37+ in
	// practice), not just the handful in the local config file. Skipped in
	// unit tests (useCLI=false) so they can control the source via the config.
	if c.useCLI {
		if out, err := runOpencodeModels(ctx); err == nil {
			if models := parseCLIModels(string(out)); len(models) > 0 {
				return models, nil
			}
		} else {
			log.Printf("opencode: 'opencode models' CLI failed (%v); falling back to config (free models may be missing)", err)
		}
	}

	// Fallback / test path: static config file.
	return c.modelsFromConfig(), nil
}

// parseCLIModels parses the newline-separated 'opencode models' output into
// ModelInfo entries. Each line is a "provider/model" ID.
func parseCLIModels(output string) []ModelInfo {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	seen := make(map[string]bool, len(lines))
	models := make([]ModelInfo, 0, len(lines))
	for _, line := range lines {
		id := strings.TrimSpace(line)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		provider := ""
		name := id
		if idx := strings.Index(id, "/"); idx >= 0 {
			provider = id[:idx]
			name = id[idx+1:]
		}
		models = append(models, ModelInfo{
			ID:       id,
			Name:     name,
			Provider: provider,
		})
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	return models
}

// modelsFromConfig reads model definitions from the static opencode.json.
func (c *LocalClient) modelsFromConfig() []ModelInfo {
	data, err := os.ReadFile(c.opencodeConfig)
	if err != nil {
		return nil
	}

	var config struct {
		Model    string                 `json:"model"`
		Provider map[string]interface{} `json:"provider"`
	}
	_ = json.Unmarshal(data, &config)

	modelSet := make(map[string]bool)
	models := make([]ModelInfo, 0)

	// Add the default model first.
	if config.Model != "" {
		models = append(models, ModelInfo{ID: config.Model, Name: config.Model})
		modelSet[config.Model] = true
	}

	// Add models from each provider config.
	for providerName, providerData := range config.Provider {
		providerMap, ok := providerData.(map[string]interface{})
		if !ok {
			continue
		}
		providerModels, ok := providerMap["models"].(map[string]interface{})
		if !ok {
			continue
		}
		for modelName := range providerModels {
			modelKey := modelName
			if !strings.Contains(modelName, "/") {
				modelKey = providerName + "/" + modelName
			}
			if !modelSet[modelKey] {
				models = append(models, ModelInfo{
					ID:       modelKey,
					Name:     modelName,
					Provider: providerName,
				})
				modelSet[modelKey] = true
			}
		}
	}

	return models
}

// Status returns connectivity based on whether opencode.json exists.
func (c *LocalClient) Status(_ context.Context) (ClientStatus, error) {
	if _, err := os.Stat(c.opencodeConfig); err == nil {
		return ClientStatus{Connected: true, Source: "local", Version: "file"}, nil
	}
	return ClientStatus{Connected: false}, nil
}

// Sessions returns a stub that always errors — local config does not support sessions.
func (c *LocalClient) Sessions() SessionAPI {
	return &localSessionAPI{}
}
