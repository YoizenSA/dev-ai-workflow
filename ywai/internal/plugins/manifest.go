package plugins

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// What ywai installs into each agent's config is policy, not code. The
// manifest declares it as data: which id installs, for which agents, behind
// which CLI flag, and under which OpenCode flavor restriction. Editing the
// manifest changes the install policy; wiring a NEW id still needs a Go
// executor below, and an entry without one fails validation.
//
// Layering mirrors role-defaults: the embedded manifest is the seed, and an
// external override file fully replaces it when present. Deleting the
// override returns the install to the shipped defaults.

//go:embed manifest.json
var embeddedManifest []byte

// ManifestPath returns the optional external override file. When it exists it
// fully replaces the embedded manifest.
func ManifestPath() string {
	return filepath.Join(config.DataDir(), "plugins.json")
}

// ManifestEntry is one installable in the manifest.
type ManifestEntry struct {
	ID     string   `json:"id"`
	Agents []string `json:"agents,omitempty"` // empty = every agent the plugin pass handles
	Flag   string   `json:"flag,omitempty"`   // install only with this CLI flag: mcp, meta-mcp, ponytail
	Flavor string   `json:"flavor,omitempty"` // "v2" = OpenCode 2 only; skipped with a notice on v1
}

// Manifest is the install policy document.
type Manifest struct {
	Install []ManifestEntry `json:"install"`
}

// manifestExecutors maps a manifest id to its installer: the manifest decides
// WHETHER something installs, these functions carry the HOW. An id defined
// here but absent from the manifest simply never installs.
var manifestExecutors = map[string]func(agentName, configPath string) error{
	"background-agents": func(_ string, configPath string) error { return InstallBackgroundAgents(configPath) },
	"vision-bridge":     func(_ string, configPath string) error { return InstallVisionBridge(configPath) },
	"advisor":           installAdvisorEntry,
	"tui-logo":          func(_ string, configPath string) error { return InstallTuiLogo(configPath) },
	"chrome-devtools":   func(agentName, configPath string) error { return InstallChromeDevToolsMCP(configPath, agentName) },
	"grafana":           func(agentName, configPath string) error { return InstallGrafanaMCP(configPath, agentName) },
	"microsoft-learn":   func(agentName, configPath string) error { return InstallMicrosoftLearnMCP(configPath, agentName) },
	"meta-devtools":     func(agentName, configPath string) error { return InstallMetaDevToolsMCP(configPath, agentName) },
	"ponytail":          InstallPonytail,
}

// installAdvisorEntry vendors the advisor plugin plus the /advisor command,
// which only works where the plugin's tools are registered (opencode).
func installAdvisorEntry(agentName, configPath string) error {
	if err := InstallAdvisor(configPath); err != nil {
		return err
	}
	if agentName == "opencode" {
		return InstallAdvisorCommand(config.OpenCodeCommandsDir())
	}
	return nil
}

// LoadManifest resolves the install policy: the external override when valid,
// the embedded seed otherwise. A malformed override is ignored with a warning
// rather than half-applied, so a bad hand edit can never brick an install.
func LoadManifest() (Manifest, []string) {
	var warnings []string
	if data, err := os.ReadFile(ManifestPath()); err == nil {
		mf, perr := parseManifest(data)
		if perr == nil {
			return mf, nil
		}
		warnings = append(warnings, fmt.Sprintf("ignoring %s: %v — using the embedded install policy", ManifestPath(), perr))
	}
	mf, err := parseManifest(embeddedManifest)
	if err != nil {
		return Manifest{}, append(warnings, "embedded install manifest is invalid: "+err.Error())
	}
	return mf, warnings
}

// parseManifest unmarshals and validates a manifest document. An empty
// install list is valid policy: it means "install nothing".
func parseManifest(data []byte) (Manifest, error) {
	var mf Manifest
	if err := json.Unmarshal(data, &mf); err != nil {
		return Manifest{}, err
	}
	seen := map[string]bool{}
	for _, e := range mf.Install {
		switch {
		case e.ID == "":
			return Manifest{}, fmt.Errorf("manifest entry missing id")
		case manifestExecutors[e.ID] == nil:
			return Manifest{}, fmt.Errorf("unknown install id %q", e.ID)
		case seen[e.ID]:
			return Manifest{}, fmt.Errorf("duplicate install id %q", e.ID)
		case e.Flavor != "" && e.Flavor != "v2":
			return Manifest{}, fmt.Errorf("%s: unknown flavor %q (want \"v2\" or empty)", e.ID, e.Flavor)
		}
		seen[e.ID] = true
	}
	return mf, nil
}

// ManifestResult reports what one manifest entry did for one agent.
type ManifestResult struct {
	ID      string
	Skipped string // non-empty: deliberately not installed, with the reason
	Err     error  // non-nil: install attempted and failed
}

// RunManifest applies every entry that applies to the agent, in manifest
// order. Entries filtered out by their agent list or flag stay silent — that
// is policy working, not a problem; flavor skips and failures are reported so
// the output explains what did not install and why.
func RunManifest(mf Manifest, agentName, configPath string, flags map[string]bool) []ManifestResult {
	var results []ManifestResult
	for _, e := range mf.Install {
		if len(e.Agents) > 0 && !slices.Contains(e.Agents, agentName) {
			continue
		}
		if e.Flag != "" && !flags[e.Flag] {
			continue
		}
		r := ManifestResult{ID: e.ID}
		r.Err = manifestExecutors[e.ID](agentName, configPath)
		results = append(results, r)
	}
	return results
}
