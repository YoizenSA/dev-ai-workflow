package tokenbank

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ---------------------------------------------------------------------------
// OpenCode
// ---------------------------------------------------------------------------

// OpenCode config path.
func OpenCodeConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "opencode", "opencode.json")
}

// ConfigureOpenCode merges the tokenbank provider into opencode.json.
func ConfigureOpenCode(baseURL, apiKey string) error {
	configPath := OpenCodeConfigPath()

	// Fetch config from API
	resp, err := FetchConfig(baseURL, apiKey, "opencode")
	if err != nil {
		return fmt.Errorf("fetching opencode config: %w", err)
	}

	// Parse the config from API
	var newConfig map[string]interface{}
	if err := json.Unmarshal(resp.Config, &newConfig); err != nil {
		return fmt.Errorf("parsing opencode config: %w", err)
	}

	// Fetch models and inject context limits
	if modelsResp, err := FetchModels(baseURL, apiKey); err == nil {
		injectModelLimits(newConfig, modelsResp.Models)
	} else {
		fmt.Printf("  ⚠ Warning: could not fetch model limits: %v\n", err)
	}

	// Read existing config
	existing, err := ReadJSONFile(configPath)
	if err != nil {
		return err
	}

	// Deep merge
	merged := DeepMerge(existing, newConfig)

	// Write
	if err := WriteJSONFile(configPath, merged); err != nil {
		return err
	}

	fmt.Printf("  ✓ OpenCode configured: %s\n", configPath)
	fmt.Printf("    Provider: opencode-admin → %s/v1\n", resp.Origin)
	return nil
}

// injectModelLimits inyecta limit.context y limit.output en cada modelo
// del provider opencode-admin dentro del config map.
func injectModelLimits(config map[string]interface{}, models []ModelInfo) {
	provider, _ := config["provider"].(map[string]interface{})
	if provider == nil {
		return
	}
	admin, _ := provider["opencode-admin"].(map[string]interface{})
	if admin == nil {
		return
	}
	modelsSection, _ := admin["models"].(map[string]interface{})
	if modelsSection == nil {
		return
	}

	for _, m := range models {
		entry, ok := modelsSection[m.ID].(map[string]interface{})
		if !ok {
			continue
		}

		hasCtx := m.MaxInputTokens > 0
		hasOut := m.MaxOutputToken > 0
		if hasCtx || hasOut {
			limit := make(map[string]interface{})
			if hasCtx {
				limit["context"] = m.MaxInputTokens
			}
			if hasOut {
				limit["output"] = m.MaxOutputToken
			}
			entry["limit"] = limit
		}

		// Inject additional capability flags.
		// Kimi models (Moonshot upstream) reject the `temperature` parameter
		// and respond 502 Bad Gateway, so disable it for them while keeping
		// reasoning/variants intact. All other models accept temperature.
		entry["reasoning"] = true
		entry["temperature"] = !isKimiModel(m.ID)
		entry["tool_call"] = true

		// Respect TokenBank vision/modalities metadata. Forcing every model to
		// accept images makes OpenCode send media natively to text-only models
		// (e.g. deepseek-v4-flash) and TokenBank returns 502 Upstream error.
		// Text-only models keep attachment=false so agents can use mcp-vision
		// tools instead of a broken native image path.
		applyVisionCapabilities(entry, m)
	}
}

// applyVisionCapabilities sets attachment + modalities from TokenBank metadata.
// Prefer explicit modalities from the API; fall back to the vision flag.
func applyVisionCapabilities(entry map[string]interface{}, m ModelInfo) {
	input := []string{"text"}
	output := []string{"text"}
	if m.Modalities != nil {
		if len(m.Modalities.Input) > 0 {
			input = m.Modalities.Input
		}
		if len(m.Modalities.Output) > 0 {
			output = m.Modalities.Output
		}
	} else if m.Vision {
		input = []string{"text", "audio", "image", "video", "pdf"}
	}

	supportsMedia := m.Vision
	for _, mod := range input {
		switch mod {
		case "image", "audio", "video", "pdf":
			supportsMedia = true
		}
	}

	entry["attachment"] = supportsMedia
	entry["modalities"] = map[string]interface{}{
		"input":  input,
		"output": output,
	}
}

// isKimiModel reports whether a model ID belongs to the Kimi (Moonshot)
// family, whose upstream gateway returns 502 when `temperature` is sent.
func isKimiModel(id string) bool {
	return strings.HasPrefix(strings.ToLower(id), "kimi")
}

// ---------------------------------------------------------------------------
// Pi
// ---------------------------------------------------------------------------

// Pi config path.
func PiConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".pi", "agent", "models.json")
}

// ConfigurePi merges the tokenbank provider into Pi's models.json.
func ConfigurePi(baseURL, apiKey string) error {
	configPath := PiConfigPath()

	// Fetch config from API
	resp, err := FetchConfig(baseURL, apiKey, "pi")
	if err != nil {
		return fmt.Errorf("fetching pi config: %w", err)
	}

	// Parse the config from API
	var newConfig map[string]interface{}
	if err := json.Unmarshal(resp.Config, &newConfig); err != nil {
		return fmt.Errorf("parsing pi config: %w", err)
	}

	// Read existing config
	existing, err := ReadJSONFile(configPath)
	if err != nil {
		return err
	}

	// Deep merge (merges providers.tokenbank-proxy)
	merged := DeepMerge(existing, newConfig)

	// Write
	if err := WriteJSONFile(configPath, merged); err != nil {
		return err
	}

	fmt.Printf("  ✓ Pi configured: %s\n", configPath)
	fmt.Printf("    Provider: tokenbank-proxy → %s/v1\n", resp.Origin)
	return nil
}

// ---------------------------------------------------------------------------
// Copilot
// ---------------------------------------------------------------------------

// CopilotConfigPaths returns all VS Code chatLanguageModels.json paths:
// - Default user config (Code)
// - Default user config (Code - Insiders)
// - All profile-specific configs found under both Code and Code - Insiders.
func CopilotConfigPaths() []string {
	home, _ := os.UserHomeDir()
	var paths []string
	var baseDirs []string

	switch runtime.GOOS {
	case "darwin":
		baseDirs = append(baseDirs,
			filepath.Join(home, "Library", "Application Support", "Code"),
			filepath.Join(home, "Library", "Application Support", "Code - Insiders"),
		)
	case "windows":
		appdata := os.Getenv("APPDATA")
		if appdata == "" {
			appdata = filepath.Join(home, "AppData", "Roaming")
		}
		baseDirs = append(baseDirs,
			filepath.Join(appdata, "Code"),
			filepath.Join(appdata, "Code - Insiders"),
		)
	default: // linux
		baseDirs = append(baseDirs,
			filepath.Join(home, ".config", "Code"),
			filepath.Join(home, ".config", "Code - Insiders"),
		)
	}

	seen := make(map[string]bool)
	for _, base := range baseDirs {
		// Default user config
		defaultPath := filepath.Join(base, "User", "chatLanguageModels.json")
		if !seen[defaultPath] {
			seen[defaultPath] = true
			if _, err := os.Stat(filepath.Dir(defaultPath)); err == nil {
				// Only add if the parent dir (User/) exists
				paths = append(paths, defaultPath)
			}
		}

		// Profile-specific configs
		profilesDir := filepath.Join(base, "User", "profiles")
		entries, err := os.ReadDir(profilesDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			profilePath := filepath.Join(profilesDir, entry.Name(), "chatLanguageModels.json")
			if !seen[profilePath] {
				seen[profilePath] = true
				paths = append(paths, profilePath)
			}
		}
	}

	return paths
}

// ConfigureCopilot merges the tokenbank provider into VS Code's chatLanguageModels.json
// for the default user config and all discovered profiles.
func ConfigureCopilot(baseURL, apiKey string) error {
	// Fetch config from API
	resp, err := FetchConfig(baseURL, apiKey, "copilot")
	if err != nil {
		return fmt.Errorf("fetching copilot config: %w", err)
	}

	// Parse the config from API (it's an array)
	var newEntries []interface{}
	if err := json.Unmarshal(resp.Config, &newEntries); err != nil {
		return fmt.Errorf("parsing copilot config: %w", err)
	}

	// Inject thinking/reasoning support into each model entry
	newEntries = injectThinkingFields(newEntries)

	// Discover all config paths (default + profiles)
	configPaths := CopilotConfigPaths()
	if len(configPaths) == 0 {
		return fmt.Errorf("no VS Code installation found (checked Code and Code - Insiders)")
	}

	for _, configPath := range configPaths {
		// Read existing config (array)
		existing, err := ReadJSONArrayFile(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠ Skipping %s: %v\n", configPath, err)
			continue
		}

		// Find and replace existing Token Bank entry, or append
		newEntriesMap := makeEntryMap(newEntries)
		merged := make([]interface{}, 0, len(existing)+len(newEntries))

		// Track which new entries we've added
		added := make(map[string]bool)

		for _, entry := range existing {
			entryMap, ok := entry.(map[string]interface{})
			if ok {
				name, _ := entryMap["name"].(string)
				vendor, _ := entryMap["vendor"].(string)
				key := vendor + "/" + name

				// If this is a Token Bank entry, replace with new config
				if replacement, exists := newEntriesMap[key]; exists {
					merged = append(merged, replacement)
					added[key] = true
					continue
				}
			}
			merged = append(merged, entry)
		}

		// Append any new entries not yet added
		for key, entry := range newEntriesMap {
			if !added[key] {
				merged = append(merged, entry)
			}
		}

		// Write
		if err := WriteJSONFile(configPath, merged); err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠ Failed to write %s: %v\n", configPath, err)
			continue
		}

		fmt.Printf("  ✓ Copilot configured: %s\n", configPath)
	}

	fmt.Printf("    Provider: Token Bank → %s/v1/chat/completions\n", resp.Origin)
	fmt.Printf("    NOTE: Set TOKENBANK_API_KEY environment variable for Copilot to authenticate\n")
	return nil
}

// injectThinkingFields adds thinking and supportsReasoningEffort to every model
// in each provider entry, matching the format VS Code expects for reasoning models.
func injectThinkingFields(entries []interface{}) []interface{} {
	reasoningEffort := []interface{}{"low", "medium", "high"}

	for _, entry := range entries {
		entryMap, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		modelsRaw, ok := entryMap["models"].([]interface{})
		if !ok {
			continue
		}
		for _, model := range modelsRaw {
			modelMap, ok := model.(map[string]interface{})
			if !ok {
				continue
			}
			modelMap["thinking"] = true
			modelMap["supportsReasoningEffort"] = reasoningEffort
		}
	}
	return entries
}

// makeEntryMap converts a slice of entries to a map keyed by "vendor/name".
func makeEntryMap(entries []interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for _, entry := range entries {
		entryMap, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := entryMap["name"].(string)
		vendor, _ := entryMap["vendor"].(string)
		result[vendor+"/"+name] = entry
	}
	return result
}

// ---------------------------------------------------------------------------
// ConfigureAll configures all detected agents.
// ---------------------------------------------------------------------------

// ConfigureAll configures all agents with the given tokenbank credentials.
// Returns a list of errors for agents that failed.
func ConfigureAll(baseURL, apiKey string) []error {
	var errors []error

	agents := []struct {
		name string
		fn   func(string, string) error
	}{
		{"opencode", ConfigureOpenCode},
		{"pi", ConfigurePi},
		{"copilot", ConfigureCopilot},
	}

	for _, agent := range agents {
		if err := agent.fn(baseURL, apiKey); err != nil {
			errors = append(errors, fmt.Errorf("%s: %w", agent.name, err))
		}
	}

	return errors
}
