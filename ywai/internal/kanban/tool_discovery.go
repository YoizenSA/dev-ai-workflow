package kanban

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/mcp"
)

// opencodeToolIDsURL is opencode's authoritative tool catalog: it returns
// every tool ID, including built-ins and tools registered dynamically by
// plugins. It requires a running opencode server, so callers must treat a
// failure as "server not up" and fall back to static discovery.
const opencodeToolIDsURL = "http://127.0.0.1:4096/experimental/tool/ids"

// discoverOpencodeToolIDs asks the running opencode server for the complete
// list of tool IDs. Best-effort: returns nil if opencode is not reachable so
// the caller can fall back to scanning config + bundles.
func discoverOpencodeToolIDs() []string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, opencodeToolIDsURL, nil)
	if err != nil {
		return nil
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var ids []string
	if err := json.NewDecoder(resp.Body).Decode(&ids); err != nil {
		return nil
	}
	return ids
}

// discoverMCPTools probes a remote MCP endpoint over HTTP and returns the
// tool names it advertises. Thin wrapper around mcp.DiscoverHTTP — the probe
// implementation lives in the mcp package now so the install flow can share
// it. ctx is background: the kanban handler invokes this from a request
// handler whose lifetime we don't propagate here (matches the prior behavior
// of the old unexported implementation, which used a 3s client timeout).
func discoverMCPTools(urlStr string) ([]string, error) {
	return mcp.DiscoverHTTP(context.Background(), urlStr)
}

// discoverStdioMCPTools spawns a stdio MCP server subprocess, runs the
// initialize + tools/list handshake, and returns the discovered tool names.
// Thin wrapper around mcp.DiscoverStdio — see internal/mcp/discovery.go for
// the transport. ctx is background: the old unexported implementation
// layered an 8s timeout via context.WithTimeout, and that cap is preserved
// inside mcp.DiscoverStdio.
func discoverStdioMCPTools(command []string, env map[string]string) ([]string, error) {
	return mcp.DiscoverStdio(context.Background(), command, env)
}

func sortStrings(a []string) {
	for i := 0; i < len(a); i++ {
		for j := i + 1; j < len(a); j++ {
			if a[j] < a[i] {
				a[i], a[j] = a[j], a[i]
			}
		}
	}
}

// discoverPluginTools parses plugin source code to find tool registrations.
// Looks for patterns like: toolName: tool({ ... }) or "toolName": tool({ ... })
func discoverPluginTools(pluginDir string) []string {
	toolSet := map[string]bool{}

	// Possible plugin entry files. Pi-style plugins expose tools in pi-entry.js.
	candidates := []string{
		filepath.Join(pluginDir, "dist", "index.js"),
		filepath.Join(pluginDir, "dist", "pi-entry.js"),
		filepath.Join(pluginDir, "index.js"),
		filepath.Join(pluginDir, "pi-entry.js"),
	}

	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(data)

		// Pattern 1: toolName: tool({ ... }) — look for tool keys before `: tool(`
		toolPattern := regexp.MustCompile(`(?:"|')?([a-z][a-z0-9_]*)(?:"|')?:\s*tool\s*\(`)
		for _, match := range toolPattern.FindAllStringSubmatch(content, -1) {
			if len(match) > 1 && match[1] != "" {
				toolSet[match[1]] = true
			}
		}

		// Pattern 2: name: "toolname" in tool definitions
		namePattern := regexp.MustCompile(`name\s*:\s*["']([a-z][a-z0-9_]*)["']`)
		for _, match := range namePattern.FindAllStringSubmatch(content, -1) {
			if len(match) > 1 && match[1] != "" {
				toolSet[match[1]] = true
			}
		}
	}

	var tools []string
	for t := range toolSet {
		tools = append(tools, t)
	}
	sortStrings(tools)
	return tools
}

// knownBundleTools maps a seeded plugin bundle filename to the tools it
// exposes. ywai ships its own plugins as minified bundles (e.g.
// background-agents.js, ~500KB); regex-scanning a minified file yields false
// positives, so we declare their stable tool sets explicitly. These bundles
// are seeded next to the opencode config and referenced from its "plugin"
// array, so discoverAllPluginTools (which only scans the npm packages dir)
// never sees them.
var knownBundleTools = map[string][]string{
	"background-agents.js": {
		"delegate",
		"delegation_read",
		"delegation_list",
		"delegation_status",
		"delegation_peek",
		"delegation_steer",
		"delegation_stop",
	},
}

// scanPluginFile parses a single plugin source/bundle file for tool
// registrations. Best-effort: used for non-ywai local plugins where we have
// no declared tool set.
func scanPluginFile(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	content := string(data)
	toolSet := map[string]bool{}
	toolPattern := regexp.MustCompile(`(?:"|')?([a-z][a-z0-9_]*)(?:"|')?:\s*tool\s*\(`)
	for _, m := range toolPattern.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 && m[1] != "" {
			toolSet[m[1]] = true
		}
	}
	namePattern := regexp.MustCompile(`name\s*:\s*["']([a-z][a-z0-9_]*)["']`)
	for _, m := range namePattern.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 && m[1] != "" {
			toolSet[m[1]] = true
		}
	}
	var tools []string
	for t := range toolSet {
		tools = append(tools, t)
	}
	sortStrings(tools)
	return tools
}

// discoverConfigPluginTools resolves the tools exposed by every plugin listed
// in opencode's "plugin" array. Entries are local file paths (ywai seeds
// bundles next to the opencode config) optionally prefixed with "file:".
// ywai bundles use knownBundleTools; other local JS files fall back to a
// best-effort source scan. npm-package plugins are covered by
// discoverAllPluginTools instead. Keyed by a friendly plugin name.
func discoverConfigPluginTools(entries []string) map[string][]string {
	result := map[string][]string{}
	for _, raw := range entries {
		path := strings.TrimPrefix(raw, "file://")
		path = strings.TrimPrefix(path, "file:")
		base := filepath.Base(path)
		name := strings.TrimSuffix(base, filepath.Ext(base))
		if name == "" || name == "." {
			continue
		}

		if tools, ok := knownBundleTools[base]; ok {
			result[name] = append([]string(nil), tools...)
			continue
		}
		if tools := scanPluginFile(path); len(tools) > 0 {
			result[name] = tools
		}
	}
	return result
}

// discoverAllPluginTools scans the opencode packages directory for plugin tools.
func discoverAllPluginTools() map[string][]string {
	result := map[string][]string{}

	home, err := os.UserHomeDir()
	if err != nil {
		return result
	}

	// Check .cache/opencode/packages/ for npm plugins
	packagesDir := filepath.Join(home, ".cache", "opencode", "packages")
	if _, err := os.Stat(packagesDir); err != nil {
		return result
	}

	entries, err := os.ReadDir(packagesDir)
	if err != nil {
		return result
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Check for @scope/package pattern
		if strings.HasPrefix(entry.Name(), "@") {
			scopeEntries, err := os.ReadDir(filepath.Join(packagesDir, entry.Name()))
			if err != nil {
				continue
			}
			for _, scopeEntry := range scopeEntries {
				if !scopeEntry.IsDir() {
					continue
				}
				pluginPath := filepath.Join(packagesDir, entry.Name(), scopeEntry.Name())
				// Package dir may include a version suffix (e.g. opencode-ado@0.4.4).
				// The actual installed package lives under node_modules/@scope/package
				// without the version suffix, so try both forms.
				scopeName := entry.Name()
				pkgName := scopeEntry.Name()
				pkgBase := pkgName
				if idx := strings.LastIndex(pkgName, "@"); idx > 0 {
					pkgBase = pkgName[:idx]
				}
				// Try node_modules/@scope/package (no version suffix)
				nmPathBase := filepath.Join(pluginPath, "node_modules", scopeName, pkgBase)
				tools := discoverPluginTools(nmPathBase)
				// Fallback: node_modules/@scope/package@version
				if len(tools) == 0 {
					nmPathVersioned := filepath.Join(pluginPath, "node_modules", scopeName, pkgName)
					tools = discoverPluginTools(nmPathVersioned)
				}
				// Fallback: plugin root itself
				if len(tools) == 0 {
					tools = discoverPluginTools(pluginPath)
				}
				if len(tools) > 0 {
					pluginName := scopeName + "/" + pkgBase
					result[pluginName] = tools
				}
			}
		} else {
			pluginPath := filepath.Join(packagesDir, entry.Name())
			tools := discoverPluginTools(pluginPath)
			if len(tools) > 0 {
				result[entry.Name()] = tools
			}
		}
	}

	return result
}
