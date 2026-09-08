package plugins

import (
	"reflect"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
)

// v1 reads "plugin", v2 reads "plugins". Writing the wrong key leaves the
// plugin registered where the client never looks, so it silently does nothing.
func TestWritePlugins_KeyFollowsFlavor(t *testing.T) {
	for _, tc := range []struct {
		flavor, want, stale string
	}{
		{"v1", "plugin", "plugins"},
		{"v2", "plugins", "plugin"},
	} {
		t.Run(tc.flavor, func(t *testing.T) {
			t.Setenv(agent.OpenCodeOverrideEnv, tc.flavor)

			// Seed the other spelling: a flavor switch must not leave it behind.
			root := map[string]any{tc.stale: []any{"old"}}
			writePlugins(root, []any{"a.js"})

			if _, lingering := root[tc.stale]; lingering {
				t.Errorf("stale %q key survived the write: %v", tc.stale, root)
			}
			got, ok := root[tc.want].([]any)
			if !ok {
				t.Fatalf("root[%q] = %v, want a slice", tc.want, root[tc.want])
			}
			if !reflect.DeepEqual(got, []any{"a.js"}) {
				t.Errorf("root[%q] = %v, want [a.js]", tc.want, got)
			}
		})
	}
}

// Switching flavors must carry the existing entries over, not drop them.
func TestOpenCodePlugins_ReadsEitherSpelling(t *testing.T) {
	for _, key := range []string{"plugin", "plugins"} {
		root := map[string]any{key: []any{"kept.js"}}
		got := openCodePlugins(root)
		if !reflect.DeepEqual(got, []any{"kept.js"}) {
			t.Errorf("openCodePlugins(%q) = %v, want [kept.js]", key, got)
		}
		// Both spellings are cleared so writePlugins owns what goes back.
		for _, k := range []string{"plugin", "plugins"} {
			if _, still := root[k]; still {
				t.Errorf("reading via %q left %q behind: %v", key, k, root)
			}
		}
	}
}

// The round trip is what an install actually does: read, append, write.
func TestPluginRoundTrip_V1ToV2Migrates(t *testing.T) {
	root := map[string]any{"plugin": []any{"existing.js"}}

	t.Setenv(agent.OpenCodeOverrideEnv, "v2")
	plugins := openCodePlugins(root)
	writePlugins(root, append(plugins, "new.js"))

	if _, old := root["plugin"]; old {
		t.Errorf("v1 key survived the migration: %v", root)
	}
	got, _ := root["plugins"].([]any)
	if !reflect.DeepEqual(got, []any{"existing.js", "new.js"}) {
		t.Errorf("plugins = %v, want [existing.js new.js]", got)
	}
}
