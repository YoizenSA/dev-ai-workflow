package mcp

// agent_config.go — multi-format persistence of MCP server configs for the
// three agent targets ywai supports. Atomicity via write-to-tmp + os.Rename
// (POSIX-atomic on the same FS); concurrency via a per-target mutex held
// for the whole read-modify-write.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, "opencode", "opencode.json"), nil
		}
		return filepath.Join(home, ".config", "opencode", "opencode.json"), nil
	case "pi":
		return filepath.Join(home, ".pi", "agent", "mcp.json"), nil
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
	case "pi", "claude-code":
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
		shape["enabled"] = true
		return shape
	}
	if target == "opencode" {
		shape["type"] = "local"
		shape["command"] = stringSliceToAny(entry.Command)
	} else {
		shape["command"] = entry.Command[0]
		shape["args"] = stringSliceToAny(entry.Command[1:])
	}
	if len(creds) > 0 {
		shape["env"] = stringMapToAny(creds)
	}
	shape["enabled"] = true
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
		root[key] = section
	}
	section[entryID] = shape
	return nil
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
