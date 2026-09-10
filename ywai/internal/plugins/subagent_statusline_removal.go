package plugins

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// RemoveSubagentStatusline uninstalls the vendored sub-agent statusline: both
// bundle halves and the TUI client config entry that registers the TUI one.
//
// It is retired rather than fixed. opencode2 shows subagents natively — a
// Subagents panel in the sidebar and a subagent count in the footer — and both
// read the parent edge the delegation plugin now writes, so the vendored
// statusline duplicates the host's own display in the same slots while
// tracking the same work less well.
//
// Every step is idempotent, so a config that never had it is left untouched.
func RemoveSubagentStatusline(configPath string) (bool, error) {
	dir := filepath.Dir(configPath)
	removed := false

	server := filepath.Join(dir, autoDiscoveredPluginsSubdir, config.SubagentStatuslineServerBundleName)
	if err := os.Remove(server); err == nil {
		removed = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("remove %s: %w", server, err)
	}

	tuiDir := filepath.Join(dir, autoDiscoveredPluginsSubdir, SubagentStatuslineTuiPluginDir)
	if _, err := os.Stat(tuiDir); err == nil {
		if err := os.RemoveAll(tuiDir); err != nil {
			return false, fmt.Errorf("remove %s: %w", tuiDir, err)
		}
		removed = true
	}

	// The legacy loose bundle, for a config that never went through the
	// directory layout.
	legacy := filepath.Join(dir, tuiPluginsSubdir, config.SubagentStatuslineTuiBundleName)
	if err := os.Remove(legacy); err == nil {
		removed = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("remove %s: %w", legacy, err)
	}

	dropped, err := dropSubagentStatuslineTuiEntry(filepath.Join(dir, tuiConfigName))
	if err != nil {
		return false, err
	}
	return removed || dropped, nil
}

// dropSubagentStatuslineTuiEntry removes the statusline entry from the TUI
// client config, in both the directory and legacy loose-file spellings, and
// leaves every unrelated entry and key alone.
func dropSubagentStatuslineTuiEntry(tuiConfigPath string) (bool, error) {
	root, err := config.ReadJSONC(tuiConfigPath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read %s: %w", tuiConfigPath, err)
	}

	entries := openCodePlugins(root)
	kept := make([]any, 0, len(entries))
	for _, raw := range entries {
		if s, ok := raw.(string); ok && isSubagentStatuslineTuiPath(s) {
			continue
		}
		kept = append(kept, raw)
	}
	if len(kept) == len(entries) {
		// openCodePlugins clears both spellings, so the array still has to be
		// written back even when nothing was dropped.
		writePlugins(root, kept)
		return false, config.WriteJSONC(tuiConfigPath, root)
	}

	writePlugins(root, kept)
	if err := config.WriteJSONC(tuiConfigPath, root); err != nil {
		return false, fmt.Errorf("write %s: %w", tuiConfigPath, err)
	}
	return true, nil
}

// isSubagentStatuslineTuiPath matches both layouts the entry has had: the
// plugin directory the v2 loader resolves, and the loose bundle file older
// installs registered.
func isSubagentStatuslineTuiPath(entry string) bool {
	base := filepath.Base(entry)
	return base == SubagentStatuslineTuiPluginDir || base == config.SubagentStatuslineTuiBundleName
}
