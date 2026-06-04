package tokenbank

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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

// Copilot config path depends on OS.
func CopilotConfigPath() string {
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "Code", "User", "chatLanguageModels.json")
	case "windows":
		appdata := os.Getenv("APPDATA")
		if appdata == "" {
			home, _ := os.UserHomeDir()
			appdata = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appdata, "Code", "User", "chatLanguageModels.json")
	default: // linux
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", "Code", "User", "chatLanguageModels.json")
	}
}

// ConfigureCopilot merges the tokenbank provider into VS Code's chatLanguageModels.json.
func ConfigureCopilot(baseURL, apiKey string) error {
	configPath := CopilotConfigPath()

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

	// Read existing config (array)
	existing, err := ReadJSONArrayFile(configPath)
	if err != nil {
		return err
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
		return err
	}

	fmt.Printf("  ✓ Copilot configured: %s\n", configPath)
	fmt.Printf("    Provider: Token Bank → %s/v1/chat/completions\n", resp.Origin)
	fmt.Printf("    NOTE: Set TOKENBANK_API_KEY environment variable for Copilot to authenticate\n")
	return nil
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
