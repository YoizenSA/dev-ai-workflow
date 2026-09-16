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
// manifest declares it as data: which id installs, HOW it installs (kind +
// params), for which agents, and behind which CLI flag. Adding a plugin that
// reuses an existing kind is pure JSON plus its bundle or catalog entry — no
// Go changes. Only a genuinely new install mechanism needs a new kind and its
// few lines in the dispatcher.
//
// Layering mirrors role-defaults: the embedded manifest is the seed, and an
// external override file fully replaces it when present. Deleting the
// override returns the install to the shipped defaults.

//go:embed manifest.json
var embeddedManifest []byte

// Install kinds. Each kind is one shared installer driven by entry params.
const (
	// KindVendorJS vendors a .js plugin bundle where OpenCode v2
	// auto-discovers it. Params: bundle, sweep, command.
	KindVendorJS = "vendor-js"
	// KindVendorTUI vendors a TUI plugin directory and registers it in
	// cli.json. Params: bundle, dir, mouse.
	KindVendorTUI = "vendor-tui"
	// KindMCP merges a catalog MCP server into the agent config. Params:
	// catalog (defaults to id), claudeStdio.
	KindMCP = "mcp"
	// KindExec removes a legacy plugin entry on opencode and installs via
	// the host CLI on claude-code. Params: remove, marketplace, plugin.
	KindExec = "exec"
)

// ManifestPath returns the optional external override file. When it exists it
// fully replaces the embedded manifest.
func ManifestPath() string {
	return filepath.Join(config.DataDir(), "plugins.json")
}

// ManifestEntry is one installable in the manifest.
type ManifestEntry struct {
	ID     string   `json:"id"`
	Kind   string   `json:"kind,omitempty"`
	Agents []string `json:"agents,omitempty"` // empty = every agent the plugin pass handles
	Flag   string   `json:"flag,omitempty"`   // install only with this CLI flag: mcp, meta-mcp, ponytail
	Label  string   `json:"label,omitempty"`  // install UI row name (flagged entries)
	Desc   string   `json:"description,omitempty"`
	Bundle string   `json:"bundle,omitempty"` // vendor-js/vendor-tui: bundle key (see vendorBundles)
	Dir    string   `json:"dir,omitempty"`    // vendor-tui: plugin directory name under plugins/
	Mouse  bool     `json:"mouse,omitempty"`  // vendor-tui: enable mouse capture in cli.json
	// Command names a slash command file to install alongside the bundle
	// (opencode only).
	Command string `json:"command,omitempty"`
	Catalog string `json:"catalog,omitempty"` // mcp: catalog id (defaults to the entry id)
	// ClaudeStdio overrides the entry for claude-code/pi configs with a stdio
	// server argv instead of the catalog transport.
	ClaudeStdio []string `json:"claudeStdio,omitempty"`
	Remove      string   `json:"remove,omitempty"`      // exec: opencode plugin entry to remove
	Marketplace string   `json:"marketplace,omitempty"` // exec: claude marketplace to add
	Plugin      string   `json:"plugin,omitempty"`      // exec: claude plugin id to install
}

// Manifest is the install policy document.
type Manifest struct {
	Install []ManifestEntry `json:"install"`
}

// vendorBundle resolves a bundle key to its source path and file name.
type vendorBundle struct {
	path func() (string, error)
	name string
}

// vendorBundles maps a manifest bundle key to the config resolution that
// finds the source (source checkout, seeded copy, embedded FS). Adding a new
// vendored plugin registers its bundle here; the per-plugin installer choice
// stays in the manifest JSON.
var vendorBundles = map[string]vendorBundle{
	"background-agents":        {config.BackgroundAgentsBundlePath, config.BackgroundAgentsBundleName},
	"vision-bridge":            {config.VisionBridgeBundlePath, config.VisionBridgeBundleName},
	"advisor":                  {config.AdvisorBundlePath, config.AdvisorBundleName},
	"tui-logo":                 {config.TuiLogoBundlePath, config.TuiLogoBundleName},
	"background-agents-notify": {config.BackgroundAgentsNotifyBundlePath, config.BackgroundAgentsNotifyBundleName},
}

// slashCommands maps an installed command file to the source it is copied
// from. The command is a router over the plugin's tools, so it only installs
// alongside its vendor-js entry (opencode only).
var slashCommands = map[string]func() (string, error){
	AdvisorCommandName: config.AdvisorCommandPath,
}

// installVendorJS vendors the entry's bundle where OpenCode
// auto-discovers it, and installs its slash command on opencode when the
// entry names one.
func installVendorJS(agentName, configPath string, e ManifestEntry) error {
	b, ok := vendorBundles[e.Bundle]
	if !ok {
		return fmt.Errorf("unknown vendor bundle %q", e.Bundle)
	}
	src, err := b.path()
	if err != nil {
		return err
	}
	if err := installVendorPluginV2(configPath, src, b.name); err != nil {
		return err
	}
	if e.Command != "" {
		if agentName != "opencode" {
			return nil
		}
		// The command source is validated by validateEntryKind; the
		// installer resolves it the same way InstallAdvisorCommand does.
		return InstallAdvisorCommand(config.OpenCodeCommandsDir())
	}
	return nil
}

// installVendorTUI vendors the entry's bundle as a TUI plugin directory and
// registers it in cli.json, enabling mouse capture only for entries that need
// it and only when the user has not explicitly opted out.
func installVendorTUI(configPath string, e ManifestEntry) error {
	b, ok := vendorBundles[e.Bundle]
	if !ok {
		return fmt.Errorf("unknown vendor bundle %q", e.Bundle)
	}
	src, err := b.path()
	if err != nil {
		return err
	}
	destDir, err := installTuiPluginDir(configPath, src, e.Dir)
	if err != nil {
		return fmt.Errorf("install %s: %w", e.ID, err)
	}
	tuiConfig := filepath.Join(filepath.Dir(configPath), tuiConfigName)
	return patchTuiPlugin(tuiConfig, destDir, e.Mouse)
}

// installExecEntry removes a legacy plugin entry on opencode and installs via
// the host CLI on claude-code. Other agents are not wired.
func installExecEntry(agentName, configPath string, e ManifestEntry) error {
	switch agentName {
	case "opencode":
		return removeOpenCodePluginName(configPath, e.Remove)
	case "claude-code":
		return installClaudeMarketplacePlugin(e.Marketplace, e.Plugin)
	default:
		return fmt.Errorf("%s install not supported for agent %q", e.ID, agentName)
	}
}

// installMCPManifestEntry merges the entry's catalog server into the agent
// config, honoring its claude-code stdio override when set.
func installMCPManifestEntry(agentName, configPath string, e ManifestEntry) error {
	catalogID := e.Catalog
	if catalogID == "" {
		catalogID = e.ID
	}
	return installCatalogMCP(configPath, agentName, catalogID, e.ClaudeStdio)
}

// runManifestEntry dispatches one entry to its kind installer. An entry
// without a kind resolves against the embedded manifest, so overrides written
// against the old id-only schema keep working.
func runManifestEntry(e ManifestEntry, agentName, configPath string) error {
	if e.Kind == "" {
		resolved, err := resolveEntry(e)
		if err != nil {
			return err
		}
		e = resolved
	}
	switch e.Kind {
	case KindVendorJS:
		return installVendorJS(agentName, configPath, e)
	case KindVendorTUI:
		return installVendorTUI(configPath, e)
	case KindMCP:
		return installMCPManifestEntry(agentName, configPath, e)
	case KindExec:
		return installExecEntry(agentName, configPath, e)
	default:
		return fmt.Errorf("unknown install kind %q", e.Kind)
	}
}

// embeddedDefaults indexes the embedded manifest by id so kindless entries
// (old id-only overrides, or Manifest values built in code) resolve their
// kind and params from the shipped policy.
func embeddedDefaults() map[string]ManifestEntry {
	var mf Manifest
	if err := json.Unmarshal(embeddedManifest, &mf); err != nil {
		return nil
	}
	out := make(map[string]ManifestEntry, len(mf.Install))
	for _, e := range mf.Install {
		out[e.ID] = e
	}
	return out
}

// resolveEntry fills a kindless entry from the embedded manifest: the shipped
// kind and params apply, while an explicitly set agents/flag list on the
// entry still wins (that retuning is the point of an override).
func resolveEntry(e ManifestEntry) (ManifestEntry, error) {
	d, ok := embeddedDefaults()[e.ID]
	if !ok {
		return ManifestEntry{}, fmt.Errorf("unknown install id %q", e.ID)
	}
	resolved := d
	if e.Agents != nil {
		resolved.Agents = e.Agents
	}
	if e.Flag != "" {
		resolved.Flag = e.Flag
	}
	return resolved, nil
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
// install list is valid policy: it means "install nothing". A kindless entry
// resolves against the embedded manifest by id; anything else unknown is an
// error.
func parseManifest(data []byte) (Manifest, error) {
	var mf Manifest
	if err := json.Unmarshal(data, &mf); err != nil {
		return Manifest{}, err
	}
	seen := map[string]bool{}
	for i, e := range mf.Install {
		switch {
		case e.ID == "":
			return Manifest{}, fmt.Errorf("manifest entry missing id")
		case seen[e.ID]:
			return Manifest{}, fmt.Errorf("duplicate install id %q", e.ID)
		}
		if e.Kind == "" {
			resolved, err := resolveEntry(e)
			if err != nil {
				return Manifest{}, err
			}
			mf.Install[i] = resolved
		} else if err := validateEntryKind(e); err != nil {
			return Manifest{}, err
		}
		seen[e.ID] = true
	}
	return mf, nil
}

// validateEntryKind checks the kind name and the params it needs.
func validateEntryKind(e ManifestEntry) error {
	switch e.Kind {
	case KindVendorJS:
		if vendorBundles[e.Bundle].path == nil {
			return fmt.Errorf("%s: unknown vendor bundle %q", e.ID, e.Bundle)
		}
		if e.Command != "" && slashCommands[e.Command] == nil {
			return fmt.Errorf("%s: unknown slash command %q", e.ID, e.Command)
		}
	case KindVendorTUI:
		if vendorBundles[e.Bundle].path == nil {
			return fmt.Errorf("%s: unknown vendor bundle %q", e.ID, e.Bundle)
		}
		if e.Dir == "" {
			return fmt.Errorf("%s: vendor-tui entry missing dir", e.ID)
		}
	case KindMCP:
		catalogID := e.Catalog
		if catalogID == "" {
			catalogID = e.ID
		}
		if err := checkCatalogID(catalogID); err != nil {
			return fmt.Errorf("%s: %w", e.ID, err)
		}
		if len(e.ClaudeStdio) == 1 {
			return fmt.Errorf("%s: claudeStdio needs at least a command", e.ID)
		}
	case KindExec:
		if e.Remove == "" && (e.Marketplace == "" || e.Plugin == "") {
			return fmt.Errorf("%s: exec entry needs remove or marketplace+plugin", e.ID)
		}
	default:
		return fmt.Errorf("%s: unknown install kind %q", e.ID, e.Kind)
	}
	return nil
}

// ManifestResult reports what one manifest entry did for one agent.
type ManifestResult struct {
	ID      string
	Skipped string // non-empty: deliberately not installed, with the reason
	Err     error  // non-nil: install attempted and failed
}

// RunManifest applies every entry that applies to the agent, in manifest
// order. Entries filtered out by their agent list or flag stay silent — that
// is policy working, not a problem; failures are reported so the output
// explains what did not install and why.
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
		r.Err = runManifestEntry(e, agentName, configPath)
		results = append(results, r)
	}
	return results
}
