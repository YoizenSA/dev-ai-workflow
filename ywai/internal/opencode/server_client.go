package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"time"
)

const defaultTimeout = 3 * time.Second

// ServerClient implements Client by querying the opencode HTTP server API.
type ServerClient struct {
	baseURL    string
	httpClient *http.Client
	sessions   *serverSessionAPI
	// useCLI controls whether ListModels tries 'opencode models' first.
	// True for production, false for unit tests so they can deterministically
	// exercise the HTTP fallbacks.
	useCLI bool
}

// NewServerClient creates a ServerClient targeting the given base URL
// (e.g. "http://127.0.0.1:4096").
func NewServerClient(baseURL string) *ServerClient {
	return &ServerClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		useCLI: true,
	}
}

// ─── Agent API types (response from GET /agent) ────────────────────────────

type rawAgent struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Provider    *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"provider"`
	Model *struct {
		ModelID    string `json:"modelID"`
		ProviderID string `json:"providerID"`
	} `json:"model"`
}

// rawProvider is the shape of a provider inside GET /provider's "all" array.
type rawProvider struct {
	Name    string                 `json:"name"`
	ID      string                 `json:"id"`
	Type    string                 `json:"type"`
	Models  map[string]interface{} `json:"models"`
}

type providerResponse struct {
	All       []rawProvider       `json:"all"`
	Default   map[string]string   `json:"default"`
	Connected []string            `json:"connected"`
}

// rawProviderV2 is the shape from GET /api/provider.
type rawProviderV2 struct {
	ID         string `json:"id"`
	ProviderID string `json:"providerID"`
	Name       string `json:"name"`
	API        string `json:"api"`
}

type providerV2Response struct {
	Location json.RawMessage `json:"location"`
	Data     []rawProviderV2 `json:"data"`
}

// ─── Client interface implementation ───────────────────────────────────────

// ListAgents fetches agents from the opencode server.
func (c *ServerClient) ListAgents(ctx context.Context) ([]AgentInfo, error) {
	url := c.baseURL + "/agent"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("opencode server: create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("opencode server: get %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("opencode server: %s returned status %d", url, resp.StatusCode)
	}

	var raw []rawAgent
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("opencode server: decode agents: %w", err)
	}

	agents := make([]AgentInfo, 0, len(raw))
	for _, r := range raw {
		ai := AgentInfo{ID: r.ID, Name: r.Name, Description: r.Description}
		if r.Provider != nil {
			ai.Provider = r.Provider.ID
			if ai.Provider == "" {
				ai.Provider = r.Provider.Name
			}
		}
		if r.Model != nil {
			ai.Model = r.Model.ModelID
		}
		if ai.ID == "" {
			ai.ID = ai.Name
		}
		agents = append(agents, ai)
	}
	return agents, nil
}

// ListModels returns every model the opencode runtime knows about.
// It prefers the `opencode models` CLI, which enumerates all runtime models
// (37+ in practice — connected providers, free models, etc.). The HTTP
// /api/provider endpoint only returns the handful declared in opencode.json.
func (c *ServerClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	if c.useCLI {
		if out, err := exec.CommandContext(ctx, "opencode", "models").Output(); err == nil {
			if models := parseCLIModels(string(out)); len(models) > 0 {
				return models, nil
			}
		}
	}

	models, err := c.listModelsV2(ctx)
	if err == nil {
		return models, nil
	}

	models, err = c.listModelsV1(ctx)
	if err == nil {
		return models, nil
	}

	return nil, fmt.Errorf("opencode server: all endpoints failed: %v", err)
}

func (c *ServerClient) listModelsV2(ctx context.Context) ([]ModelInfo, error) {
	url := c.baseURL + "/api/provider"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var env providerV2Response
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	models := make([]ModelInfo, 0, len(env.Data))
	for _, p := range env.Data {
		id := p.ID
		if id == "" {
			id = p.ProviderID
		}
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		models = append(models, ModelInfo{
			ID:       id,
			Provider: p.ProviderID,
			Name:     p.Name,
		})
	}
	return models, nil
}

func (c *ServerClient) listModelsV1(ctx context.Context) ([]ModelInfo, error) {
	url := c.baseURL + "/provider"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var provResp providerResponse
	if err := json.NewDecoder(resp.Body).Decode(&provResp); err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	models := make([]ModelInfo, 0)
	for _, p := range provResp.All {
		// Iterate over models within each provider
		for modelID, modelData := range p.Models {
			modelMap, ok := modelData.(map[string]interface{})
			if !ok {
				continue
			}
			modelName := modelID
			if name, ok := modelMap["name"].(string); ok && name != "" {
				modelName = name
			}
			modelKey := p.ID + "/" + modelID
			if modelKey == "" || seen[modelKey] {
				continue
			}
			seen[modelKey] = true
			models = append(models, ModelInfo{
				ID:       modelKey,
				Provider: p.ID,
				Name:     modelName,
			})
		}
	}
	return models, nil
}

// Status checks if the opencode server is reachable.
func (c *ServerClient) Status(ctx context.Context) (ClientStatus, error) {
	// Try /status first, then /health as fallback.
	url := c.baseURL + "/status"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ClientStatus{Connected: false}, nil
	}

	resp, err := c.httpClient.Do(req)
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			// Try to get connected providers from /provider
			connectedProviders := c.getConnectedProviders(ctx)
			return ClientStatus{Connected: true, Source: "server", Version: "api", ConnectedProviders: connectedProviders}, nil
		}
	}

	// Fallback: try /health
	url = c.baseURL + "/health"
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ClientStatus{Connected: false}, nil
	}

	resp, err = c.httpClient.Do(req)
	if err != nil {
		return ClientStatus{Connected: false}, nil
	}
	resp.Body.Close()

	// Try to get connected providers from /provider
	connectedProviders := c.getConnectedProviders(ctx)
	return ClientStatus{Connected: true, Source: "server", Version: "api", ConnectedProviders: connectedProviders}, nil
}

// getConnectedProviders fetches the list of connected providers from /provider endpoint.
func (c *ServerClient) getConnectedProviders(ctx context.Context) []string {
	url := c.baseURL + "/provider"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var provResp providerResponse
	if err := json.NewDecoder(resp.Body).Decode(&provResp); err != nil {
		return nil
	}

	return provResp.Connected
}

// Sessions returns the session management API backed by the opencode HTTP server.
func (c *ServerClient) Sessions() SessionAPI {
	if c.sessions == nil {
		c.sessions = newServerSessionAPI(c.baseURL)
	}
	return c.sessions
}
