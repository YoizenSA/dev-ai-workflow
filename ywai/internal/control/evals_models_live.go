package control

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/opencode"
)

// Live models: the AgentBenchmarks picker used to read the static
// opencode.json provider section, which only knows hand-configured providers.
// The running OpenCode server merges built-ins, env logins, auth.json and
// every config layer — so the picker now asks the server itself. Anything
// the server cannot reach degrades back to the static file (the UI falls
// back on any non-2xx).

type liveModel struct {
	Provider string `json:"provider"`
	ID       string `json:"id"`
	Name     string `json:"name"`
}

// fetchLiveModels queries {baseURL}/api/model (opencode2 dialect) with the
// stored server credentials and returns the slimmed provider/id/name list.
func fetchLiveModels(baseURL string, client *http.Client) ([]liveModel, error) {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/model", nil)
	if err != nil {
		return nil, err
	}
	opencode.ApplyServerAuth(req)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("opencode server: %s", resp.Status)
	}
	var payload struct {
		Data []struct {
			ID         string `json:"id"`
			ModelID    string `json:"modelID"`
			ProviderID string `json:"providerID"`
			Name       string `json:"name"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	out := make([]liveModel, 0, len(payload.Data))
	for _, m := range payload.Data {
		id := m.ModelID
		if id == "" {
			id = m.ID
		}
		name := m.Name
		if name == "" {
			name = id
		}
		out = append(out, liveModel{Provider: m.ProviderID, ID: id, Name: name})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Provider != out[j].Provider {
			return out[i].Provider < out[j].Provider
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// handleEvalModelsLive proxies the live model list of the OpenCode server
// the bench would run against.
func (s *Server) handleEvalModelsLive(w http.ResponseWriter, r *http.Request) {
	base := opencodeURLForBench()
	models, err := fetchLiveModels(base, nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": models})
}
