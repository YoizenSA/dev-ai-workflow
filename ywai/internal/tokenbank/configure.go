package tokenbank

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// OpenCode
// ---------------------------------------------------------------------------

// OpenCodeConfigPath is the user-level opencode.json TokenBank configure
// writes. Isolated hosts (Orca) set OPENCODE_CONFIG_DIR for their own
// OpenCode process; that must not steal the user's catalog.
func OpenCodeConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", "opencode.json")
	}
	return filepath.Join(home, ".config", "opencode", "opencode.json")
}

func openCodeConfigPaths() []string {
	primary := OpenCodeConfigPath()
	paths := []string{primary}
	if dir := strings.TrimSpace(os.Getenv("OPENCODE_CONFIG_DIR")); dir != "" {
		extra := filepath.Join(dir, "opencode.json")
		if extra != primary {
			paths = append(paths, extra)
		}
	}
	return paths
}

// ConfigureOpenCode merges the tokenbank provider into opencode.json.
func ConfigureOpenCode(baseURL, apiKey string) error {
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

	// Fetch models and inject context limits. GET /v1/models is the
	// catalog: models the GET does not return are dropped from the provider.
	if modelsResp, err := FetchModels(baseURL, apiKey); err == nil {
		injectModelLimits(newConfig, modelsResp.Models)
	} else {
		fmt.Printf("  ⚠ Warning: could not fetch model limits: %v\n", err)
	}

	var lastPath string
	for _, configPath := range openCodeConfigPaths() {
		existing, err := ReadJSONFile(configPath)
		if err != nil {
			return err
		}

		// Deep merge, then let the (GET-pruned) API provider own the slot so
		// models dropped upstream are removed instead of lingering.
		merged := DeepMerge(existing, newConfig)
		replaceOwnedProvider(merged, newConfig, "provider", "opencode-admin")

		if err := WriteJSONFile(configPath, merged); err != nil {
			return err
		}
		lastPath = configPath
		fmt.Printf("  ✓ OpenCode configured: %s\n", configPath)
	}

	if lastPath != "" {
		fmt.Printf("    Provider: opencode-admin → %s/v1\n", resp.Origin)
	}
	return nil
}

// replaceOwnedProvider makes the API response authoritative for the one
// provider ywai owns. DeepMerge only ever adds and overwrites keys, so a model
// the API stopped returning would survive in the local config forever and keep
// being offered by the agent long after TokenBank dropped it. Everything
// outside providerKey — other providers, and the user's own settings — is left
// exactly as the merge produced it.
func replaceOwnedProvider(merged, fresh map[string]interface{}, sectionKey, providerKey string) {
	freshSection, ok := fresh[sectionKey].(map[string]interface{})
	if !ok {
		return
	}
	freshProvider, ok := freshSection[providerKey].(map[string]interface{})
	if !ok {
		return
	}
	mergedSection, ok := merged[sectionKey].(map[string]interface{})
	if !ok {
		mergedSection = map[string]interface{}{}
		merged[sectionKey] = mergedSection
	}
	mergedSection[providerKey] = freshProvider
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

	// GET /v1/models is the catalog. An empty fetch is a fail-safe:
	// never wipe the local list because the API returned nothing.
	if len(models) > 0 {
		allowed := make(map[string]struct{}, len(models))
		for _, m := range models {
			if m.ID != "" {
				allowed[m.ID] = struct{}{}
			}
		}
		for id := range modelsSection {
			if _, ok := allowed[id]; !ok {
				delete(modelsSection, id)
			}
		}
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
		// Text-only models keep attachment=false so vision-bridge can analyze
		// attached images via a vision model instead of a broken native path.
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

	if modelsResp, err := FetchModels(baseURL, apiKey); err == nil {
		prunePiModels(newConfig, modelsResp.Models)
	}

	// Read existing config
	existing, err := ReadJSONFile(configPath)
	if err != nil {
		return err
	}

	// Deep merge, then let the API response own providers.tokenbank-proxy
	// outright so models dropped upstream are removed instead of lingering.
	merged := DeepMerge(existing, newConfig)
	replaceOwnedProvider(merged, newConfig, "providers", OmpProviderID)

	// Write
	if err := WriteJSONFile(configPath, merged); err != nil {
		return err
	}

	fmt.Printf("  ✓ Pi configured: %s\n", configPath)
	fmt.Printf("    Provider: tokenbank-proxy → %s/v1\n", resp.Origin)
	return nil
}

func prunePiModels(config map[string]interface{}, catalog []ModelInfo) {
	if len(catalog) == 0 {
		return
	}
	allowed := make(map[string]struct{}, len(catalog))
	for _, m := range catalog {
		if m.ID != "" {
			allowed[m.ID] = struct{}{}
		}
	}
	providers, _ := config["providers"].(map[string]interface{})
	proxy, _ := providers[OmpProviderID].(map[string]interface{})
	if proxy == nil {
		return
	}
	raw, _ := proxy["models"].([]interface{})
	kept := make([]interface{}, 0, len(catalog))
	for _, item := range raw {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := m["id"].(string)
		if _, ok := allowed[id]; ok {
			kept = append(kept, item)
		}
	}
	proxy["models"] = kept
}

// ---------------------------------------------------------------------------
// OMP (oh-my-pi)
// ---------------------------------------------------------------------------

// OmpProviderID is the provider key written into ~/.omp/agent/models.yml.
const OmpProviderID = "tokenbank-proxy"

// OmpConfigPath returns ~/.omp/agent/models.yml (OMP's preferred models file).
func OmpConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".omp", "agent", "models.yml")
}

// ConfigureOmp merges the TokenBank OpenAI-compatible proxy into OMP's models.yml.
//
// TokenBank does not need a dedicated "omp" setup target: we build a valid
// openai-completions provider from GET /v1/models + credentials.
// Existing providers in models.yml are preserved; tokenbank-proxy is replaced.
func ConfigureOmp(baseURL, apiKey string) error {
	configPath := OmpConfigPath()

	modelsResp, err := FetchModels(baseURL, apiKey)
	if err != nil {
		return fmt.Errorf("fetching models for omp: %w", err)
	}

	origin := strings.TrimRight(modelsResp.Origin, "/")
	if origin == "" {
		origin = strings.TrimRight(baseURL, "/")
	}
	v1Base := origin + "/v1"

	provider := BuildOmpTokenBankProvider(v1Base, apiKey, modelsResp.Models)

	existing, err := ReadYAMLFile(configPath)
	if err != nil {
		return err
	}
	if existing == nil {
		existing = map[string]interface{}{}
	}

	providers, _ := existing["providers"].(map[string]interface{})
	if providers == nil {
		providers = map[string]interface{}{}
	}
	providers[OmpProviderID] = provider
	existing["providers"] = providers

	if err := WriteYAMLFile(configPath, existing); err != nil {
		return err
	}

	fmt.Printf("  ✓ OMP configured: %s\n", configPath)
	fmt.Printf("    Provider: %s → %s\n", OmpProviderID, v1Base)
	fmt.Printf("    Models: %d (use: omp models %s  or  /model %s/<id>)\n",
		len(modelsResp.Models), OmpProviderID, OmpProviderID)
	return nil
}

// BuildOmpTokenBankProvider builds the OMP models.yml provider entry for TokenBank.
func BuildOmpTokenBankProvider(v1Base, apiKey string, models []ModelInfo) map[string]interface{} {
	modelEntries := make([]interface{}, 0, len(models))
	for _, m := range models {
		name := m.Name
		if name == "" {
			name = m.ID
		}
		entry := map[string]interface{}{
			"id":   m.ID,
			"name": name,
			"api":  "openai-completions",
		}
		if m.MaxInputTokens > 0 {
			entry["contextWindow"] = m.MaxInputTokens
		} else {
			entry["contextWindow"] = 128000
		}
		if m.MaxOutputToken > 0 {
			entry["maxTokens"] = m.MaxOutputToken
		} else {
			entry["maxTokens"] = 16384
		}
		// input modalities
		inputs := []interface{}{"text"}
		if IsVisionModel(m) {
			inputs = append(inputs, "image")
		}
		entry["input"] = inputs
		entry["cost"] = map[string]interface{}{
			"input":      0,
			"output":     0,
			"cacheRead":  0,
			"cacheWrite": 0,
		}
		modelEntries = append(modelEntries, entry)
	}

	return map[string]interface{}{
		"baseUrl":    v1Base,
		"apiKey":     apiKey, // OMP: env name if set, else literal token
		"api":        "openai-completions",
		"authHeader": true,
		"models":     modelEntries,
	}
}

// ReadYAMLFile reads a YAML object file. Missing file → empty map.
func ReadYAMLFile(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]interface{}{}, nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	data = []byte(strings.TrimLeft(string(data), "\ufeff"))
	var result map[string]interface{}
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if result == nil {
		result = map[string]interface{}{}
	}
	return result, nil
}

// WriteYAMLFile writes a YAML file with backup of the previous contents.
func WriteYAMLFile(path string, data interface{}) error {
	if _, err := os.Stat(path); err == nil {
		backup := path + ".bak"
		if err := os.Rename(path, backup); err != nil {
			return fmt.Errorf("backing up %s: %w", path, err)
		}
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}
	content, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshaling YAML: %w", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
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
		ownedVendors := entryVendors(newEntries)
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
				// A model under a vendor the API manages that the API no longer
				// returns was retired upstream: drop it instead of leaving a
				// dead model in the picker. Entries under any other vendor are
				// the user's own and are never touched.
				if ownedVendors[vendor] {
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

// entryVendors returns the set of vendors the API response manages. Only
// entries under these vendors are ywai's to prune.
func entryVendors(entries []interface{}) map[string]bool {
	out := make(map[string]bool)
	for _, entry := range entries {
		entryMap, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		if vendor, _ := entryMap["vendor"].(string); vendor != "" {
			out[vendor] = true
		}
	}
	return out
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
		{"omp", ConfigureOmp},
		{"copilot", ConfigureCopilot},
	}

	for _, agent := range agents {
		if err := agent.fn(baseURL, apiKey); err != nil {
			errors = append(errors, fmt.Errorf("%s: %w", agent.name, err))
		}
	}

	return errors
}
