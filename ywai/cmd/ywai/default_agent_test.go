package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// setDefaultAgent must always land on ywai's own orchestrator: it overrides
// gentle-ai's auto-set "gentle-orchestrator" but respects any other explicit
// user choice.
func TestSetDefaultAgent(t *testing.T) {
	cases := []struct {
		name    string
		initial any // nil = no config file
		want    string
	}{
		{"no config file", nil, "orchestrator"},
		{"empty config", map[string]any{}, "orchestrator"},
		{"overrides gentle-orchestrator", map[string]any{"default_agent": "gentle-orchestrator"}, "orchestrator"},
		{"respects user choice", map[string]any{"default_agent": "dev"}, "dev"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			cfgPath := filepath.Join(home, ".config", "opencode", "opencode.json")

			if tc.initial != nil {
				if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
					t.Fatal(err)
				}
				data, _ := json.Marshal(tc.initial)
				if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
					t.Fatal(err)
				}
			}

			if err := setDefaultAgent("orchestrator", false); err != nil {
				t.Fatalf("setDefaultAgent: %v", err)
			}

			data, err := os.ReadFile(cfgPath)
			if err != nil {
				t.Fatalf("reading config: %v", err)
			}
			var cfg map[string]any
			if err := json.Unmarshal(data, &cfg); err != nil {
				t.Fatal(err)
			}
			if got := cfg["default_agent"]; got != tc.want {
				t.Fatalf("default_agent = %q, want %q", got, tc.want)
			}
		})
	}
}
