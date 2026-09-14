package plugins

import (
	"reflect"
	"testing"
)

// v2 always writes "plugins"; a stale "plugin" key from a v1-era install must
// not survive the write, or the plugin ends up registered where nothing looks.
func TestWritePlugins_AlwaysPluginsKey(t *testing.T) {
	// Seed the retired spelling: the write must not leave it behind.
	root := map[string]any{"plugin": []any{"old"}}
	writePlugins(root, []any{"a.js"})

	if _, lingering := root["plugin"]; lingering {
		t.Errorf("stale %q key survived the write: %v", "plugin", root)
	}
	got, ok := root["plugins"].([]any)
	if !ok {
		t.Fatalf("root[%q] = %v, want a slice", "plugins", root["plugins"])
	}
	if !reflect.DeepEqual(got, []any{"a.js"}) {
		t.Errorf("root[%q] = %v, want [a.js]", "plugins", got)
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

// The round trip is what an install actually does: read, append, write. A
// config still carrying the v1 "plugin" spelling is migrated: both spellings
// are read, and the write lands everything under "plugins".
func TestPluginRoundTrip_V1ToV2Migrates(t *testing.T) {
	root := map[string]any{"plugin": []any{"existing.js"}}

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
