package plugins

import (
	"errors"
	"os"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// v1OnlyBundles lists plugin bundles that only work on OpenCode v1.
// Both vision-bridge and advisor now carry v2 dual exports and are installed
// into the auto-discovered plugins directory on v2, so the list is empty.
var v1OnlyBundles = []string{}

// orphanBundles are ywai plugin bundles no ywai version installs any more. They
// have no source in this repo and nothing updates them, but an older install
// left them in the config where OpenCode still loads them.
//
// background-agents-v2.js is the one that matters: it is a second build of the
// delegation plugin, so loading it alongside the maintained one puts two
// DelegationManagers on the same delegations tree, both re-adopting orphaned
// work at startup and both claiming the same tool names.
var orphanBundles = []string{
	"background-agents-v2.js",
}

// RemoveV1OnlyPlugins drops the v1-only ywai bundles from an OpenCode config
// when the active flavor is v2, and is a no-op on v1 where they still work.
//
// They are left behind by an earlier v1 install. Under v2 they sit inertly in
// the "plugin" key that v2 never reads, which looks harmless — until anything
// rewrites the array, because that write moves the entries to "plugins" and v2
// then tries to load a v1 plugin. Removing them is what makes the flavor switch
// safe in both directions. Reinstalling under v1 puts them back.
func RemoveV1OnlyPlugins(configPath string) (int, error) {
	if !agent.OpenCodeIsV2() {
		return 0, nil
	}
	root, err := config.ReadJSONC(configPath)
	// ReadJSONC wraps the OS error, so os.IsNotExist would miss it and turn a
	// machine without an opencode config into a hard failure.
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	existing := openCodePlugins(root)
	kept := make([]any, 0, len(existing))
	removed := 0
	for _, raw := range existing {
		if s, ok := raw.(string); ok && (anyContains(s, v1OnlyBundles) || anyContains(s, orphanBundles)) {
			removed++
			continue
		}
		kept = append(kept, raw)
	}
	// Always write back: openCodePlugins clears both spellings, so returning
	// early here would drop the plugin list entirely.
	writePlugins(root, kept)
	if err := config.WriteJSONC(configPath, root); err != nil {
		return 0, err
	}
	return removed, nil
}
