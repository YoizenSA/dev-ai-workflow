package kanban

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/mcp"
)

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
