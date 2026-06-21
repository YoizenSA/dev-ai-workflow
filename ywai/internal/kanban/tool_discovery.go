package kanban

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func discoverMCPTools(urlStr string) ([]string, error) {
	// JSON-RPC request for tools/list
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}
	payload, _ := json.Marshal(reqBody)

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Post(urlStr, "application/json", strings.NewReader(string(payload)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var rpcResp struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, err
	}

	names := []string{}
	for _, t := range rpcResp.Result.Tools {
		if t.Name != "" {
			names = append(names, t.Name)
		}
	}
	return names, nil
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

// discoverStdioMCPTools starts a stdio MCP server process, sends initialize
// + tools/list JSON-RPC requests, and returns the discovered tool names.
// The process is killed after discovery.
func discoverStdioMCPTools(command []string, env map[string]string) ([]string, error) {
	if len(command) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}
	defer func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	reader := bufio.NewReader(stdout)

	// Send initialize request
	initReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "ywai-kanban",
				"version": "1.0.0",
			},
		},
	}
	if err := sendJSONRPC(stdin, initReq); err != nil {
		return nil, fmt.Errorf("send initialize: %w", err)
	}

	// Read initialize response (skip any notifications)
	if _, err := readJSONRPCResponse(reader); err != nil {
		return nil, fmt.Errorf("read initialize: %w", err)
	}

	// Send initialized notification
	initialized := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	_ = sendJSONRPC(stdin, initialized)

	// Send tools/list request
	toolsReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}
	if err := sendJSONRPC(stdin, toolsReq); err != nil {
		return nil, fmt.Errorf("send tools/list: %w", err)
	}

	// Read tools/list response
	resp, err := readJSONRPCResponse(reader)
	if err != nil {
		return nil, fmt.Errorf("read tools/list: %w", err)
	}

	// Parse tool names from result
	var names []string
	if result, ok := resp["result"].(map[string]interface{}); ok {
		if tools, ok := result["tools"].([]interface{}); ok {
			for _, t := range tools {
				if tool, ok := t.(map[string]interface{}); ok {
					if name, ok := tool["name"].(string); ok && name != "" {
						names = append(names, name)
					}
				}
			}
		}
	}
	return names, nil
}

func sendJSONRPC(w io.Writer, msg map[string]interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}

func readJSONRPCResponse(reader *bufio.Reader) (map[string]interface{}, error) {
	for i := 0; i < 50; i++ {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var resp map[string]interface{}
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			continue
		}
		// Skip notifications (no id field)
		if _, ok := resp["id"]; !ok {
			continue
		}
		return resp, nil
	}
	return nil, fmt.Errorf("no response after 50 lines")
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
