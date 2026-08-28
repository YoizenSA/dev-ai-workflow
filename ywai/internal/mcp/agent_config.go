package mcp

// agent_config.go — multi-format persistence of MCP server configs for the
// agent targets ywai supports (opencode, pi, omp, claude-code). Atomicity via
// write-to-tmp + os.Rename (POSIX-atomic on the same FS); concurrency via a
// per-target mutex held for the whole read-modify-write.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// configLocks holds one *sync.Mutex per target, fetched via lockFor.
var configLocks sync.Map

func EntryTargetPath(target string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch target {
	case "opencode":
		if dir := os.Getenv("OPENCODE_CONFIG_DIR"); dir != "" {
			return config.FindJSONCPath(dir, "opencode"), nil
		}
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, "opencode", "opencode.json"), nil
		}
		return filepath.Join(home, ".config", "opencode", "opencode.json"), nil
	case "pi":
		return filepath.Join(home, ".pi", "agent", "mcp.json"), nil
	case "omp":
		// oh-my-pi reads ~/.omp/agent/mcp.json (user level) or .omp/mcp.json
		// (per project); the user-level file is the ywai-managed one.
		return filepath.Join(home, ".omp", "agent", "mcp.json"), nil
	case "claude-code":
		return filepath.Join(home, ".claude.json"), nil
	default:
		return "", fmt.Errorf("unknown mcp target %q", target)
	}
}

func topLevelKey(target string) (string, error) {
	switch target {
	case "opencode":
		return "mcp", nil
	case "pi", "claude-code", "omp":
		return "mcpServers", nil
	default:
		return "", fmt.Errorf("unknown mcp target %q", target)
	}
}

// BuildEntryShape normalizes a catalog entry into the per-target shape the
// runtime expects. creds may be nil or empty; in that case the env key is
// omitted entirely (some runtimes treat {} as a contract violation).
//
// Slices/maps are stored as []any / map[string]any so the shape survives
// a JSON round-trip and remains reflect.DeepEqual to the parsed file
// content — the atomic-writes test pins this directly.
func BuildEntryShape(target string, entry CatalogEntry, creds map[string]string) map[string]any {
	shape := map[string]any{}
	if entry.Type == "remote" {
		if target == "opencode" {
			shape["type"] = "remote"
		}
		shape["url"] = entry.URL
		if target != "opencode" {
			// v2 has no "enabled" (absent = enabled); pi/omp still read it.
			shape["enabled"] = true
		}
		return shape
	}
	if target == "opencode" {
		shape["type"] = "local"
		shape["command"] = stringSliceToAny(entry.Command)
		if len(creds) > 0 {
			// v2 renamed env → environment; servers are enabled by default.
			shape["environment"] = stringMapToAny(creds)
		}
		if entry.DefaultDisabled {
			// v2: absent = on. `enabled` is ignored; use `disabled`.
			shape["disabled"] = true
		}
		return shape
	}
	shape["command"] = entry.Command[0]
	shape["args"] = stringSliceToAny(entry.Command[1:])
	if len(creds) > 0 {
		shape["env"] = stringMapToAny(creds)
	}
	shape["enabled"] = !entry.DefaultDisabled
	return shape
}

func stringSliceToAny(s []string) []any {
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = v
	}
	return out
}

func stringMapToAny(m map[string]string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// WriteAgentConfig merges shape into the target's config under entryID.
// The write is atomic (write-to-tmp + os.Rename) and the file is created
// with mode 0o600 to protect any credentials the env map contains.
func WriteAgentConfig(target string, entryID string, shape map[string]any) (string, error) {
	path, err := EntryTargetPath(target)
	if err != nil {
		return "", err
	}
	mu := lockFor(target)
	mu.Lock()
	defer mu.Unlock()

	root, err := readRoot(path)
	if err != nil {
		return "", err
	}
	if err := putEntry(root, target, entryID, shape); err != nil {
		return "", err
	}
	if err := writeRootAtomic(path, root); err != nil {
		return "", err
	}
	return path, nil
}

// RemoveAgentConfig deletes entryID from the target's config. Removing an
// entry that does not exist (or a file that does not exist) is a no-op.
func RemoveAgentConfig(target string, entryID string) error {
	path, err := EntryTargetPath(target)
	if err != nil {
		return err
	}
	mu := lockFor(target)
	mu.Lock()
	defer mu.Unlock()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	root, err := readRoot(path)
	if err != nil {
		return err
	}
	key, err := topLevelKey(target)
	if err != nil {
		return err
	}
	section, ok := root[key].(map[string]any)
	if !ok {
		return nil
	}
	if target == "opencode" {
		servers := collectOpenCodeServers(section)
		if _, exists := servers[entryID]; !exists {
			return nil
		}
		delete(servers, entryID)
		root[key] = flattenOpenCodeMCP(section, servers)
		return writeRootAtomic(path, root)
	}
	if _, exists := section[entryID]; !exists {
		return nil
	}
	delete(section, entryID)
	return writeRootAtomic(path, root)
}

// ReadAgentConfig returns the target's mcp/mcpServers section. A missing
// file yields an empty (non-nil) map and no error; malformed JSON yields
// an error so the UI can surface corruption.
func ReadAgentConfig(target string) (map[string]any, error) {
	path, err := EntryTargetPath(target)
	if err != nil {
		return nil, err
	}
	// Serialize against WriteAgentConfig's atomic rename. On POSIX the
	// rename is atomic and readers never tear, but Windows uses mandatory
	// locking: opening the target while a concurrent write is mid-rename
	// fails with a sharing violation. Sharing the per-target lock makes
	// reads and writes mutually exclusive on every platform.
	mu := lockFor(target)
	mu.Lock()
	data, err := os.ReadFile(path)
	mu.Unlock()
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if root == nil {
		return map[string]any{}, nil
	}
	key, err := topLevelKey(target)
	if err != nil {
		return nil, err
	}
	section, ok := root[key].(map[string]any)
	if !ok {
		return map[string]any{}, nil
	}
	if target == "opencode" {
		return collectOpenCodeServers(section), nil
	}
	return section, nil
}

// ─── internal helpers ──────────────────────────────────────────────────────

func lockFor(target string) *sync.Mutex {
	v, _ := configLocks.LoadOrStore(target, &sync.Mutex{})
	return v.(*sync.Mutex)
}

func readRoot(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if root == nil {
		return map[string]any{}, nil
	}
	return root, nil
}

func putEntry(root map[string]any, target, entryID string, shape map[string]any) error {
	key, err := topLevelKey(target)
	if err != nil {
		return err
	}
	section, ok := root[key].(map[string]any)
	if !ok {
		section = map[string]any{}
	}
	if target == "opencode" {
		servers := collectOpenCodeServers(section)
		servers[entryID] = shape
		root[key] = flattenOpenCodeMCP(section, servers)
		return nil
	}
	section[entryID] = shape
	root[key] = section
	return nil
}

func openCodeReservedMCPKey(k string) bool {
	return k == "servers" || k == "timeout"
}

func collectOpenCodeServers(mcp map[string]any) map[string]any {
	out := map[string]any{}
	if mcp == nil {
		return out
	}
	if nested, ok := mcp["servers"].(map[string]any); ok {
		for k, v := range nested {
			out[k] = v
		}
	}
	for k, v := range mcp {
		if openCodeReservedMCPKey(k) {
			continue
		}
		if _, ok := v.(map[string]any); ok {
			if _, exists := out[k]; !exists {
				out[k] = v
			}
		}
	}
	return out
}

// flattenOpenCodeMCP writes the opencode v1 MCP layout: each server sits
// directly under `mcp`, and every entry carries an explicit `enabled` bool.
// v1 validates that key, so a v2 entry (nested under `servers`, using
// `disabled`) is converted rather than passed through.
func flattenOpenCodeMCP(mcp map[string]any, servers map[string]any) map[string]any {
	out := map[string]any{}
	if mcp != nil {
		for k, v := range mcp {
			if k == "servers" {
				continue
			}
			if _, isObj := v.(map[string]any); isObj && !openCodeReservedMCPKey(k) {
				continue
			}
			out[k] = v
		}
	}
	for id, raw := range servers {
		entry, ok := raw.(map[string]any)
		if !ok {
			out[id] = raw
			continue
		}
		next := make(map[string]any, len(entry)+1)
		for k, v := range entry {
			next[k] = v
		}
		enabled := true
		if disabled, ok := next["disabled"].(bool); ok {
			enabled = !disabled
			delete(next, "disabled")
		}
		if e, ok := next["enabled"].(bool); ok {
			enabled = e
		}
		next["enabled"] = enabled
		out[id] = next
	}
	return out
}

// writeRootAtomic serializes root as indented JSON, writes to a sibling
// .tmp file, forces mode 0o600 (umask-independent), and renames into place.
func writeRootAtomic(path string, root map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write tmp %s: %w", tmp, err)
	}
	// Belt-and-braces: a restrictive umask can strip bits despite 0o600.
	if err := os.Chmod(tmp, 0o600); err != nil {
		return fmt.Errorf("chmod tmp %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmp, path, err)
	}
	return nil
}
