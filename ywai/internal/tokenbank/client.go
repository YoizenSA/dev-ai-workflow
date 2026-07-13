// Package tokenbank provides a client for the TokenBank setup API
// and config writers for opencode, copilot, and pi agents.
package tokenbank

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// ---------------------------------------------------------------------------
// API client
// ---------------------------------------------------------------------------

// ModelModalities is the input/output modality set returned by TokenBank.
type ModelModalities struct {
	Input  []string `json:"input,omitempty"`
	Output []string `json:"output,omitempty"`
}

// ModelInfo represents a model returned by the TokenBank API.
type ModelInfo struct {
	ID             string           `json:"id"`
	Name           string           `json:"name"`
	ToolCalling    bool             `json:"toolCalling,omitempty"`
	Vision         bool             `json:"vision,omitempty"`
	MaxInputTokens int              `json:"maxInputTokens,omitempty"`
	MaxOutputToken int              `json:"maxOutputTokens,omitempty"`
	Modalities     *ModelModalities `json:"modalities,omitempty"`
}

// ModelsResponse is the response from GET /api/setup/models.
type ModelsResponse struct {
	OK           bool        `json:"ok"`
	Origin       string      `json:"origin"`
	Models       []ModelInfo `json:"models"`
	DefaultModel string      `json:"defaultModel"`
}

// ConfigResponse is the response from GET /api/setup/config.
type ConfigResponse struct {
	OK     bool            `json:"ok"`
	Origin string          `json:"origin"`
	Target string          `json:"target"`
	Config json.RawMessage `json:"config"`
}

// IsVisionModel reports whether a model can accept image (or other media) input.
// Prefer the explicit vision flag; fall back to modalities.input containing "image".
func IsVisionModel(m ModelInfo) bool {
	if m.Vision {
		return true
	}
	if m.Modalities != nil {
		for _, mod := range m.Modalities.Input {
			if mod == "image" {
				return true
			}
		}
	}
	return false
}

// FilterVisionModels returns only vision-capable models from the catalog.
func FilterVisionModels(models []ModelInfo) []ModelInfo {
	out := make([]ModelInfo, 0, len(models))
	for _, m := range models {
		if IsVisionModel(m) {
			out = append(out, m)
		}
	}
	return out
}

// ResolveVisionModelID picks the effective vision model id.
// preferred (from user config) wins when non-empty and present in visionModels
// (or when visionModels is empty — trust the override). Otherwise the first
// vision model from the live catalog is used. Empty when nothing is available.
func ResolveVisionModelID(preferred string, visionModels []ModelInfo) string {
	preferred = strings.TrimSpace(preferred)
	if i := strings.LastIndex(preferred, "/"); i >= 0 && i+1 < len(preferred) {
		preferred = preferred[i+1:]
	}
	if preferred != "" {
		if len(visionModels) == 0 {
			return preferred
		}
		for _, m := range visionModels {
			if m.ID == preferred {
				return preferred
			}
		}
	}
	if len(visionModels) > 0 {
		return visionModels[0].ID
	}
	return ""
}

// FetchModels fetches available models from the TokenBank API.
func FetchModels(baseURL, apiKey string) (*ModelsResponse, error) {
	url := strings.TrimRight(baseURL, "/") + "/api/setup/models"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching models: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	var result ModelsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("API error: %s", string(body))
	}
	return &result, nil
}

// FetchConfig fetches the provider config for a given target agent.
func FetchConfig(baseURL, apiKey, target string) (*ConfigResponse, error) {
	url := fmt.Sprintf("%s/api/setup/config?target=%s", strings.TrimRight(baseURL, "/"), target)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching config: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	var result ConfigResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("API error: %s", string(body))
	}
	return &result, nil
}

// ---------------------------------------------------------------------------
// Config file helpers
// ---------------------------------------------------------------------------

// ReadJSONFile reads and parses a JSON (or JSONC) file.
func ReadJSONFile(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]interface{}), nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	// Strip BOM
	data = []byte(strings.TrimLeft(string(data), "\ufeff"))

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if result == nil {
		result = make(map[string]interface{})
	}
	return result, nil
}

// ReadJSONArrayFile reads and parses a JSON array file.
func ReadJSONArrayFile(path string) ([]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []interface{}{}, nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	data = []byte(strings.TrimLeft(string(data), "\ufeff"))

	var result []interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if result == nil {
		result = []interface{}{}
	}
	return result, nil
}

// WriteJSONFile writes a pretty-printed JSON file with backup.
func WriteJSONFile(path string, data interface{}) error {
	// Backup existing file
	if _, err := os.Stat(path); err == nil {
		backup := path + ".bak"
		if err := os.Rename(path, backup); err != nil {
			return fmt.Errorf("backing up %s: %w", path, err)
		}
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}
	content = append(content, '\n')

	if err := os.WriteFile(path, content, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Deep merge
// ---------------------------------------------------------------------------

// DeepMerge recursively merges src into dst. src values take precedence.
func DeepMerge(dst, src map[string]interface{}) map[string]interface{} {
	for key, srcVal := range src {
		dstVal, exists := dst[key]
		if exists {
			dstMap, dstIsMap := dstVal.(map[string]interface{})
			srcMap, srcIsMap := srcVal.(map[string]interface{})
			if dstIsMap && srcIsMap {
				dst[key] = DeepMerge(dstMap, srcMap)
				continue
			}
		}
		dst[key] = srcVal
	}
	return dst
}
