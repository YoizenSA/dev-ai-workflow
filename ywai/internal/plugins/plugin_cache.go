package plugins

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// opencode resolves npm plugins once and caches the install under
// ~/.cache/opencode/packages/<spec>/ (Kilo: ~/.cache/kilo/packages/). The
// cached copy is reused verbatim on later starts — Npm.add returns the cached
// node_modules/<name> without re-resolving @latest — so an old resolved
// version sticks even after ywai re-writes the config to "latest". These
// helpers clear the cached entries for the npm plugins ywai manages so the
// next agent start re-resolves them at their latest published version.

// managedPackageNames returns the npm package names whose cache entries ywai
// may clear: plugins ywai installs today (statusline, ponytail) plus packages
// it installed in the past and has since retired (ADO, quota). Clearing a
// retired entry is harmless cleanup — it is only re-fetched if still
// referenced in an agent config.
func managedPackageNames() []string {
	return append([]string{
		subAgentStatuslinePlugin,
		PonytailNPMPackage,
		"@slkiser/opencode-quota",
	}, adoPluginPackageNames...)
}

// pluginCacheRoots are the opencode-format plugin cache roots to sweep.
// Indirected so tests can point them at temp dirs (mirrors adoLatestFn).
var pluginCacheRoots = func() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{
		filepath.Join(home, ".cache", "opencode", "packages"),
		filepath.Join(home, ".cache", "opencode", "node_modules"),
		filepath.Join(home, ".cache", "kilo", "packages"),
		filepath.Join(home, ".cache", "kilo", "node_modules"),
	}
}

// pluginCacheEntryMatches reports whether a cache entry (dir name) belongs to
// pkg: the bare package name, or the name with any @spec suffix
// (e.g. "opencode-subagent-statusline@latest", "@dietrichgebert/ponytail@latest").
func pluginCacheEntryMatches(entryBase, pkg string) bool {
	return entryBase == pkg || strings.HasPrefix(entryBase, pkg+"@")
}

// ClearPluginCache removes the cached installs of the npm plugins ywai manages
// from the opencode/kilo plugin caches, so the next agent start re-resolves
// them at their latest published version. When dryRun is true nothing is
// deleted; the would-be entries are still returned. The returned names are
// deduplicated and relative to each cache root (e.g. "opencode-subagent-statusline@latest").
func ClearPluginCache(dryRun bool) ([]string, error) {
	seen := map[string]bool{}
	var cleared []string
	var errs []error
	for _, root := range pluginCacheRoots() {
		names, err := clearCacheRoot(root, managedPackageNames(), dryRun)
		for _, name := range names {
			if !seen[name] {
				seen[name] = true
				cleared = append(cleared, name)
			}
		}
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return cleared, fmt.Errorf("clearing plugin cache: %w", errors.Join(errs...))
	}
	return cleared, nil
}

// clearCacheRoot removes matching cache entries under one cache root and
// returns their display names. A missing root is a no-op.
func clearCacheRoot(root string, pkgs []string, dryRun bool) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", root, err)
	}

	var cleared []string
	var errs []error
	remove := func(path, name string) {
		cleared = append(cleared, name)
		if dryRun {
			return
		}
		if err := os.RemoveAll(path); err != nil {
			errs = append(errs, fmt.Errorf("remove %s: %w", path, err))
		}
	}

	for _, pkg := range pkgs {
		if strings.HasPrefix(pkg, "@") {
			// Scoped package: root/@scope/<name>[@spec].
			scope, name, ok := strings.Cut(strings.TrimPrefix(pkg, "@"), "/")
			if !ok {
				continue
			}
			scopeDir := filepath.Join(root, "@"+scope)
			sub, err := os.ReadDir(scopeDir)
			if err != nil {
				if !os.IsNotExist(err) {
					errs = append(errs, fmt.Errorf("read %s: %w", scopeDir, err))
				}
				continue
			}
			for _, e := range sub {
				if !pluginCacheEntryMatches(e.Name(), name) {
					continue
				}
				remove(filepath.Join(scopeDir, e.Name()), "@"+scope+"/"+e.Name())
			}
			continue
		}
		// Unscoped package: root/<name>[@spec].
		for _, e := range entries {
			if !pluginCacheEntryMatches(e.Name(), pkg) {
				continue
			}
			remove(filepath.Join(root, e.Name()), e.Name())
		}
	}

	if len(errs) > 0 {
		return cleared, errors.Join(errs...)
	}
	return cleared, nil
}
