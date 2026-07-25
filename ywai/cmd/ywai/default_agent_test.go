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
		// OpenCode's own built-ins: a fresh install sits on "build", which is
		// nobody's deliberate choice, so ywai may claim it.
		{"claims opencode build default", map[string]any{"default_agent": "build"}, "orchestrator"},
		{"claims opencode plan default", map[string]any{"default_agent": "plan"}, "orchestrator"},
		{"keeps orchestrator", map[string]any{"default_agent": "orchestrator"}, "orchestrator"},
		{"respects user choice", map[string]any{"default_agent": "dev"}, "dev"},
		{"respects a custom agent", map[string]any{"default_agent": "my-agent"}, "my-agent"},
		{"respects another ywai agent", map[string]any{"default_agent": "designer"}, "designer"},
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

// default_agent decides what runs in every new OpenCode session. Install may
// claim it only when it still holds a value nobody chose on purpose; taking
// over a user's own pick would silently redirect all their work.
func TestIsManagedDefaultAgent(t *testing.T) {
	claimable := []string{
		"",                    // never set
		"build",               // OpenCode's out-of-the-box default
		"plan",                // its sibling built-in
		"orchestrator",        // ywai's own
		"gentle-orchestrator", // auto-set by gentle-ai
		"  build  ",           // whitespace must not defeat the check
	}
	for _, name := range claimable {
		if !isManagedDefaultAgent(name) {
			t.Errorf("isManagedDefaultAgent(%q) = false; install would leave a default nobody picked", name)
		}
	}

	userChosen := []string{"dev", "ask", "architect", "designer", "my-agent", "qa-orchestrator", "Build"}
	for _, name := range userChosen {
		if isManagedDefaultAgent(name) {
			t.Errorf("isManagedDefaultAgent(%q) = true; install would overwrite the user's choice", name)
		}
	}
}
