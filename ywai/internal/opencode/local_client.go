package opencode

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// LocalClient implements Client by reading opencode config files directly.
type LocalClient struct {
	opencodeConfig string // path to opencode.json
	agentsDir      string // path to agents directory
}

// NewLocalClient creates a LocalClient with auto-detected paths
// based on $HOME/.config/opencode/.
func NewLocalClient() *LocalClient {
	home, _ := os.UserHomeDir()
	return &LocalClient{
		opencodeConfig: filepath.Join(home, ".config", "opencode", "opencode.json"),
		agentsDir:      filepath.Join(home, ".config", "opencode", "agents"),
	}
}

// NewLocalClientWithPaths creates a LocalClient with explicit paths (for testing).
func NewLocalClientWithPaths(configPath, agentsDir string) *LocalClient {
	return &LocalClient{
		opencodeConfig: configPath,
		agentsDir:      agentsDir,
	}
}

// ListAgents reads agent files from the agents directory.
// This replicates the current behavior in missions/web/handlers.go:ListAgents.
func (c *LocalClient) ListAgents(_ context.Context) ([]AgentInfo, error) {
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

// ListModels reads model definitions from opencode.json.
// This replicates the current behavior in missions/web/handlers.go:ListModels.
func (c *LocalClient) ListModels(_ context.Context) ([]ModelInfo, error) {
	data, err := os.ReadFile(c.opencodeConfig)
	if err != nil {
		return nil, nil //nolint:nilerr // match current behavior: empty on error
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

	return models, nil
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
