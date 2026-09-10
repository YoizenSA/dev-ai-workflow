package plugins

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

func writeConfigWithPlugins(t *testing.T, entries []any) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "opencode.json")
	if err := config.WriteJSONC(path, map[string]any{"plugin": entries}); err != nil {
		t.Fatal(err)
	}
	return path
}

// On v2, orphan bundles (like legacy background-agents-v2.js) must go, while
// supported dual-export bundles (background-agents, vision-bridge, advisor)
// survive because they now carry v2 setup hooks.
func TestRemoveV1OnlyPlugins_V2StripsOrphansOnly(t *testing.T) {
	t.Setenv(agent.OpenCodeOverrideEnv, "v2")

	path := writeConfigWithPlugins(t, []any{
		"/home/u/.config/opencode/ywai-plugins/vision-bridge.js",
		"/home/u/.config/opencode/ywai-plugins/background-agents.js",
		"/home/u/.config/opencode/ywai-plugins/advisor.js",
		"/home/u/.config/opencode/ywai-plugins/background-agents-v2.js",
		"keep-me.js",
	})

	removed, err := RemoveV1OnlyPlugins(path)
	if err != nil {
		t.Fatalf("RemoveV1OnlyPlugins: %v", err)
	}
	if removed != 1 {
		t.Errorf("removed = %d, want 1 (only orphan background-agents-v2.js)", removed)
	}

	root, err := config.ReadJSONC(path)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := root["plugins"].([]any)
	if !ok {
		t.Fatalf("v2 must write the plugins key, got %v", root)
	}
	want := []any{
		"/home/u/.config/opencode/ywai-plugins/vision-bridge.js",
		"/home/u/.config/opencode/ywai-plugins/background-agents.js",
		"/home/u/.config/opencode/ywai-plugins/advisor.js",
		"keep-me.js",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("plugins = %v, want %v", got, want)
	}
}

// On v1 the same bundles are the working plugins, so touching them would
// uninstall delegation, vision routing and the advisor.
func TestRemoveV1OnlyPlugins_V1KeepsBundles(t *testing.T) {
	t.Setenv(agent.OpenCodeOverrideEnv, "v1")

	entries := []any{"/x/ywai-plugins/advisor.js", "keep-me.js"}
	path := writeConfigWithPlugins(t, entries)

	removed, err := RemoveV1OnlyPlugins(path)
	if err != nil {
		t.Fatalf("RemoveV1OnlyPlugins: %v", err)
	}
	if removed != 0 {
		t.Errorf("removed = %d, want 0 on v1", removed)
	}

	root, _ := config.ReadJSONC(path)
	got, _ := root["plugin"].([]any)
	if !reflect.DeepEqual(got, entries) {
		t.Errorf("plugin = %v, want %v untouched", got, entries)
	}
}

// A config with nothing to strip must keep its plugin list: openCodePlugins
// clears both spellings, so an early return would blank the array.
func TestRemoveV1OnlyPlugins_V2PreservesUnrelatedList(t *testing.T) {
	t.Setenv(agent.OpenCodeOverrideEnv, "v2")

	path := writeConfigWithPlugins(t, []any{"a.js", "b.js"})
	if _, err := RemoveV1OnlyPlugins(path); err != nil {
		t.Fatalf("RemoveV1OnlyPlugins: %v", err)
	}

	root, _ := config.ReadJSONC(path)
	got, _ := root["plugins"].([]any)
	if !reflect.DeepEqual(got, []any{"a.js", "b.js"}) {
		t.Errorf("plugins = %v, want [a.js b.js]", got)
	}
}

func TestRemoveV1OnlyPlugins_MissingConfigIsNoOp(t *testing.T) {
	t.Setenv(agent.OpenCodeOverrideEnv, "v2")

	removed, err := RemoveV1OnlyPlugins(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil {
		t.Fatalf("missing config must not error: %v", err)
	}
	if removed != 0 {
		t.Errorf("removed = %d, want 0", removed)
	}
}

// A second build of the delegation plugin means two DelegationManagers over the
// same tree, both re-adopting orphaned work and claiming the same tool names.
func TestRemoveV1OnlyPlugins_V2StripsOrphanDelegationBuild(t *testing.T) {
	t.Setenv(agent.OpenCodeOverrideEnv, "v2")

	path := writeConfigWithPlugins(t, []any{
		"/x/ywai-plugins/background-agents-v2.js",
		"/x/ywai-plugins/background-agents.js",
	})

	removed, err := RemoveV1OnlyPlugins(path)
	if err != nil {
		t.Fatalf("RemoveV1OnlyPlugins: %v", err)
	}
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}

	root, _ := config.ReadJSONC(path)
	got, _ := root["plugins"].([]any)
	want := []any{"/x/ywai-plugins/background-agents.js"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("plugins = %v, want %v (the maintained build must survive)", got, want)
	}
}
