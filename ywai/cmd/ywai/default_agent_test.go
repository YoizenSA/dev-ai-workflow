package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pinOCEnv makes the OpenCode config resolvers hermetic: HOME/USERPROFILE
// point at a temp dir, and OPENCODE_CONFIG_DIR + XDG_CONFIG_HOME are cleared
// because the resolvers honor them over HOME — a live value would redirect
// writes outside the temp dir (and into the user's real config).
func pinOCEnv(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
}

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
			pinOCEnv(t, home)
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

// root `model` is the model a new session runs. ywai writes it only when the
// key is absent or empty; any value the user already set is their choice.
func TestSetDefaultModel(t *testing.T) {
	cases := []struct {
		name    string
		initial any // nil = no config file
		model   string
		want    string
	}{
		{"no config file writes", nil, "opencode-admin/deepseek-v4-pro", "opencode-admin/deepseek-v4-pro"},
		{"empty config writes", map[string]any{}, "opencode-admin/deepseek-v4-pro", "opencode-admin/deepseek-v4-pro"},
		{"empty model key writes", map[string]any{"model": ""}, "opencode-admin/deepseek-v4-pro", "opencode-admin/deepseek-v4-pro"},
		{"whitespace model key writes", map[string]any{"model": "   "}, "opencode-admin/deepseek-v4-pro", "opencode-admin/deepseek-v4-pro"},
		{"leaves user-chosen model", map[string]any{"model": "anthropic/claude-opus"}, "opencode-admin/deepseek-v4-pro", "anthropic/claude-opus"},
		{"leaves non-string model", map[string]any{"model": map[string]any{"id": "x"}}, "opencode-admin/deepseek-v4-pro", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			pinOCEnv(t, home)
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

			if err := setDefaultModel(tc.model, false); err != nil {
				t.Fatalf("setDefaultModel: %v", err)
			}

			data, err := os.ReadFile(cfgPath)
			if err != nil {
				t.Fatalf("reading config: %v", err)
			}
			var cfg map[string]any
			if err := json.Unmarshal(data, &cfg); err != nil {
				t.Fatal(err)
			}
			var got string
			if raw, ok := cfg["model"]; ok {
				got, _ = raw.(string)
			}
			if got != tc.want {
				t.Fatalf("model = %q, want %q", got, tc.want)
			}
		})
	}
}

// setDefaultAgent must resolve the config through XDG_CONFIG_HOME when HOME
// would point elsewhere.
func TestSetDefaultAgent_XDGConfigDir(t *testing.T) {
	home := t.TempDir()
	pinOCEnv(t, home)
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	if err := setDefaultAgent("orchestrator", false); err != nil {
		t.Fatalf("setDefaultAgent: %v", err)
	}

	path := filepath.Join(xdg, "opencode", "opencode.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("config not written under XDG_CONFIG_HOME: %v", err)
	}
	if !strings.Contains(string(data), `"default_agent": "orchestrator"`) {
		t.Fatalf("unexpected config: %s", data)
	}
}

// setDefaultAgent must resolve the config through OPENCODE_CONFIG_DIR, which
// isolated hosts (Orca) set when launching OpenCode with their own config.
func TestSetDefaultAgent_OpenCodeConfigDirEnv(t *testing.T) {
	home := t.TempDir()
	pinOCEnv(t, home)
	isolate := t.TempDir()
	t.Setenv("OPENCODE_CONFIG_DIR", isolate)

	if err := setDefaultAgent("orchestrator", false); err != nil {
		t.Fatalf("setDefaultAgent: %v", err)
	}

	path := filepath.Join(isolate, "opencode.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("config not written under OPENCODE_CONFIG_DIR: %v", err)
	}
	if !strings.Contains(string(data), `"default_agent": "orchestrator"`) {
		t.Fatalf("unexpected config: %s", data)
	}

	// The HOME-based file must not be created when an isolate is set.
	homePath := filepath.Join(home, ".config", "opencode", "opencode.json")
	if _, err := os.Stat(homePath); !os.IsNotExist(err) {
		t.Fatalf("HOME config should not be created when OPENCODE_CONFIG_DIR is set")
	}
}

// setDefaultModel must honor OPENCODE_CONFIG_DIR and a pre-existing .jsonc
// file (comments included), writing the model into that file and leaving an
// already-chosen model alone.
func TestSetDefaultModel_JSONC(t *testing.T) {
	home := t.TempDir()
	pinOCEnv(t, home)
	configDir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// .jsonc with a comment and a model the user already picked.
	unchanged := "{\n  // user's own choice\n  \"model\": \"anthropic/claude-opus\",\n  \"default_agent\": \"build\"\n}\n"
	if err := os.WriteFile(filepath.Join(configDir, "opencode.jsonc"), []byte(unchanged), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := setDefaultModel("opencode-admin/deepseek-v4-pro", false); err != nil {
		t.Fatalf("setDefaultModel (leave): %v", err)
	}
	path := filepath.Join(configDir, "opencode.jsonc")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "anthropic/claude-opus") || strings.Contains(string(data), "deepseek-v4-pro") {
		t.Fatalf("setDefaultModel must leave an existing model alone, got:\n%s", data)
	}

	// Empty model key in .jsonc → the key is claimed.
	empty := "{\n  // comment survives any read, not the rewrite\n  \"model\": \"\"\n}\n"
	if err := os.WriteFile(path, []byte(empty), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := setDefaultModel("opencode-admin/deepseek-v4-pro", false); err != nil {
		t.Fatalf("setDefaultModel (write): %v", err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "opencode-admin/deepseek-v4-pro") {
		t.Fatalf("setDefaultModel must write into the .jsonc file, got:\n%s", data)
	}
}

// setDefaultAgent must read and rewrite an existing .jsonc without breaking on
// the comments, and must keep using the .jsonc file rather than creating a
// sibling .json.
func TestSetDefaultAgent_JSONC(t *testing.T) {
	home := t.TempDir()
	pinOCEnv(t, home)
	configDir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	initial := "{\n  // managed default\n  \"default_agent\": \"build\"\n}\n"
	if err := os.WriteFile(filepath.Join(configDir, "opencode.jsonc"), []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := setDefaultAgent("orchestrator", false); err != nil {
		t.Fatalf("setDefaultAgent: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(configDir, "opencode.jsonc"))
	if err != nil {
		t.Fatalf("jsonc file must be updated in place: %v", err)
	}
	if !strings.Contains(string(data), `"default_agent": "orchestrator"`) {
		t.Fatalf("unexpected jsonc content: %s", data)
	}
	if _, err := os.Stat(filepath.Join(configDir, "opencode.json")); !os.IsNotExist(err) {
		t.Fatalf("a sibling opencode.json must not be created when .jsonc exists")
	}
}

// defaultRootModel must produce a plain provider/model (no #variant) from the
// seeded balanced profile in a clean environment.
func TestDefaultRootModel(t *testing.T) {
	home := t.TempDir()
	pinOCEnv(t, home)

	model := defaultRootModel()
	if model == "" {
		t.Fatal("defaultRootModel must fall back to the seeded balanced profile")
	}
	if !strings.Contains(model, "/") {
		t.Fatalf("defaultRootModel = %q, want a provider/model id", model)
	}
	if strings.Contains(model, "#") {
		t.Fatalf("defaultRootModel = %q, root model must not carry a variant", model)
	}
}

func TestStripModelVariant(t *testing.T) {
	cases := []struct{ in, want string }{
		{"opencode-admin/deepseek-v4-pro", "opencode-admin/deepseek-v4-pro"},
		{"opencode-admin/deepseek-v4-pro#high", "opencode-admin/deepseek-v4-pro"},
		{"  opencode-admin/x#max  ", "opencode-admin/x"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := stripModelVariant(tc.in); got != tc.want {
			t.Errorf("stripModelVariant(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
