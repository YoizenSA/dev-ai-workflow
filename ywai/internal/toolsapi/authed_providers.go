package toolsapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// authedProviderSources answers "which providers can actually run a model?".
// The auth store (opencode2 auth list) covers console/plan logins; opencode.json
// covers direct-API providers whose key comes from an environment variable; and
// opencode2 ships a built-in gateway provider that needs no credential at all.
// A provider in none of those sets cannot authenticate, so model pickers can
// hide its models instead of offering runs that always fail.

// credentialFreeProviders lists opencode2 built-in providers that run without
// any stored credential — the bundled gateway routes them with the install's
// own identity ("opencode2 run --model opencode/big-pickle" verified OK on an
// install whose auth store only holds opencode-go). Contrast: "openai/*" is
// listed by `opencode2 models` but fails without a real OpenAI key, and
// "opencode-admin/*" answers "Model unavailable" — both stay out.
var credentialFreeProviders = []string{"opencode"}

// envKeyProviders returns provider ids from the host opencode.json whose
// apiKey comes from an environment variable ("{env:VAR}") that is set in this
// process. The CLI child inherits this process env, so those providers are
// usable even though they never appear in the auth store. Best-effort: an
// unreadable or unparsable config yields an empty list.
func envKeyProviders() []string {
	dir := config.OpenCodeUserConfigDir()
	for _, name := range []string{"opencode.json", "opencode.jsonc"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		if ids := envKeyProvidersFrom(data, os.Getenv); len(ids) > 0 {
			return ids
		}
	}
	return nil
}

// envKeyProvidersFrom extracts "{env:VAR}"-keyed provider ids from opencode.json
// bytes, keeping only providers whose VAR resolves non-empty. Pure so tests can
// inject the environment.
func envKeyProvidersFrom(data []byte, getenv func(string) string) []string {
	var cfg struct {
		Provider map[string]struct {
			Options struct {
				APIKey string `json:"apiKey"`
			} `json:"options"`
		} `json:"provider"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	var ids []string
	for id, p := range cfg.Provider {
		key := p.Options.APIKey
		if !strings.HasPrefix(key, "{env:") || !strings.HasSuffix(key, "}") {
			continue
		}
		envVar := strings.TrimSuffix(strings.TrimPrefix(key, "{env:"), "}")
		if envVar != "" && getenv(envVar) != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}
