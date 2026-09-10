package tokenbank

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
)

// pinOCFlavor forces the OpenCode flavor so these tests stay deterministic on
// hosts with either binary installed.
func pinOCFlavor(t *testing.T, flavor string) {
	t.Helper()
	t.Setenv(agent.OpenCodeOverrideEnv, flavor)
	t.Setenv("XDG_CONFIG_HOME", "")
}

// isolateOpenCodeConfig points every path ConfigureOpenCode writes into temp
// dirs and returns the fake HOME. Hosts may export OPENCODE_CONFIG_DIR at a real
// shared config, which ConfigureOpenCode writes too, so tests must never inherit it.
func isolateOpenCodeConfig(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("OPENCODE_CONFIG_DIR", filepath.Join(t.TempDir(), "orca-isolate"))
	return home
}

func TestApplyVisionCapabilities_TextOnly(t *testing.T) {
	entry := map[string]interface{}{}
	applyVisionCapabilities(entry, ModelInfo{
		ID:     "deepseek-v4-flash",
		Vision: false,
		Modalities: &ModelModalities{
			Input:  []string{"text"},
			Output: []string{"text"},
		},
	})

	if entry["attachment"] != false {
		t.Fatalf("attachment = %v, want false for text-only model", entry["attachment"])
	}
	mods := entry["modalities"].(map[string]interface{})
	input := mods["input"].([]string)
	if len(input) != 1 || input[0] != "text" {
		t.Fatalf("input modalities = %v, want [text]", input)
	}
}

func TestApplyVisionCapabilities_VisionFromFlag(t *testing.T) {
	entry := map[string]interface{}{}
	applyVisionCapabilities(entry, ModelInfo{
		ID:     "mimo-v2.5",
		Vision: true,
	})

	if entry["attachment"] != true {
		t.Fatalf("attachment = %v, want true when vision=true", entry["attachment"])
	}
	mods := entry["modalities"].(map[string]interface{})
	input := mods["input"].([]string)
	foundImage := false
	for _, m := range input {
		if m == "image" {
			foundImage = true
		}
	}
	if !foundImage {
		t.Fatalf("expected image in default vision modalities, got %v", input)
	}
}

func TestApplyVisionCapabilities_ModalitiesFromAPI(t *testing.T) {
	entry := map[string]interface{}{}
	applyVisionCapabilities(entry, ModelInfo{
		ID:     "mimo-v2.5-pro",
		Vision: true,
		Modalities: &ModelModalities{
			Input:  []string{"text", "image"},
			Output: []string{"text"},
		},
	})

	if entry["attachment"] != true {
		t.Fatalf("attachment = %v, want true", entry["attachment"])
	}
	mods := entry["modalities"].(map[string]interface{})
	input := mods["input"].([]string)
	if len(input) != 2 || input[0] != "text" || input[1] != "image" {
		t.Fatalf("input modalities = %v, want [text image]", input)
	}
}

func TestInjectModelLimits_RespectsVision(t *testing.T) {
	config := map[string]interface{}{
		"provider": map[string]interface{}{
			"opencode-admin": map[string]interface{}{
				"models": map[string]interface{}{
					"deepseek-v4-flash": map[string]interface{}{
						"name": "DeepSeek V4 Flash",
					},
					"mimo-v2.5": map[string]interface{}{
						"name": "MiMo V2.5",
					},
				},
			},
		},
	}

	injectModelLimits(config, []ModelInfo{
		{
			ID:             "deepseek-v4-flash",
			Vision:         false,
			MaxInputTokens: 1000,
			MaxOutputToken: 100,
			Modalities:     &ModelModalities{Input: []string{"text"}, Output: []string{"text"}},
		},
		{
			ID:             "mimo-v2.5",
			Vision:         true,
			MaxInputTokens: 2000,
			MaxOutputToken: 200,
			Modalities:     &ModelModalities{Input: []string{"text", "image"}, Output: []string{"text"}},
		},
	}, "provider")

	models := config["provider"].(map[string]interface{})["opencode-admin"].(map[string]interface{})["models"].(map[string]interface{})

	flash := models["deepseek-v4-flash"].(map[string]interface{})
	if flash["attachment"] != false {
		t.Fatalf("deepseek attachment = %v, want false", flash["attachment"])
	}

	mimo := models["mimo-v2.5"].(map[string]interface{})
	if mimo["attachment"] != true {
		t.Fatalf("mimo attachment = %v, want true", mimo["attachment"])
	}
}

func TestInjectModelLimits_DropsModelsAbsentFromGET(t *testing.T) {
	config := map[string]interface{}{
		"provider": map[string]interface{}{
			"opencode-admin": map[string]interface{}{
				"models": map[string]interface{}{
					"kept":    map[string]interface{}{"name": "Kept"},
					"retired": map[string]interface{}{"name": "Retired upstream"},
				},
			},
		},
	}

	injectModelLimits(config, []ModelInfo{{ID: "kept", Name: "Kept", MaxInputTokens: 1000}}, "provider")

	models := config["provider"].(map[string]interface{})["opencode-admin"].(map[string]interface{})["models"].(map[string]interface{})
	if _, stale := models["retired"]; stale {
		t.Fatalf("a model absent from GET /v1/models must be removed, got %v", models)
	}
	kept, ok := models["kept"].(map[string]interface{})
	if !ok {
		t.Fatalf("a model the GET still returns must survive, got %v", models)
	}
	limit, _ := kept["limit"].(map[string]interface{})
	if limit["context"] != 1000 {
		t.Fatalf("kept model should still receive injected limits, got %v", kept)
	}
}

func TestInjectModelLimits_EmptyCatalogDoesNotWipe(t *testing.T) {
	config := map[string]interface{}{
		"provider": map[string]interface{}{
			"opencode-admin": map[string]interface{}{
				"models": map[string]interface{}{
					"kept": map[string]interface{}{"name": "Kept"},
				},
			},
		},
	}

	injectModelLimits(config, nil, "provider")

	models := config["provider"].(map[string]interface{})["opencode-admin"].(map[string]interface{})["models"].(map[string]interface{})
	if _, ok := models["kept"]; !ok {
		t.Fatalf("an empty GET catalog must not wipe local models, got %v", models)
	}
}

func TestPrunePiModels_DropsModelsAbsentFromGET(t *testing.T) {
	config := map[string]interface{}{
		"providers": map[string]interface{}{
			OmpProviderID: map[string]interface{}{
				"models": []interface{}{
					map[string]interface{}{"id": "kept", "name": "Kept"},
					map[string]interface{}{"id": "ghost", "name": "Not in /v1/models"},
				},
			},
		},
	}

	prunePiModels(config, []ModelInfo{{ID: "kept"}})

	models := config["providers"].(map[string]interface{})[OmpProviderID].(map[string]interface{})["models"].([]interface{})
	if len(models) != 1 {
		t.Fatalf("pi models = %v, want only kept", models)
	}
	if models[0].(map[string]interface{})["id"] != "kept" {
		t.Fatalf("kept id missing, got %v", models)
	}
}

func TestBuildOmpTokenBankProvider(t *testing.T) {
	p := BuildOmpTokenBankProvider("https://tb.example/v1", "tb-key", []ModelInfo{
		{ID: "flash", Name: "Flash", MaxInputTokens: 1000, MaxOutputToken: 200},
		{ID: "vision", Name: "", Vision: true},
	})
	if p["baseUrl"] != "https://tb.example/v1" {
		t.Fatalf("baseUrl = %v", p["baseUrl"])
	}
	if p["api"] != "openai-completions" {
		t.Fatalf("api = %v", p["api"])
	}
	if p["authHeader"] != true {
		t.Fatalf("authHeader = %v", p["authHeader"])
	}
	if p["apiKey"] != "tb-key" {
		t.Fatalf("apiKey = %v", p["apiKey"])
	}
	models := p["models"].([]interface{})
	if len(models) != 2 {
		t.Fatalf("models len = %d", len(models))
	}
	m0 := models[0].(map[string]interface{})
	if m0["contextWindow"] != 1000 || m0["maxTokens"] != 200 {
		t.Fatalf("flash limits = %+v", m0)
	}
	m1 := models[1].(map[string]interface{})
	if m1["name"] != "vision" {
		t.Fatalf("empty name should fall back to id, got %v", m1["name"])
	}
	inputs := m1["input"].([]interface{})
	if len(inputs) != 2 || inputs[1] != "image" {
		t.Fatalf("vision input = %v, want [text image]", inputs)
	}
}

func TestWriteAndReadOmpYAMLRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "models.yml")

	// Seed with an unrelated provider so merge must keep it.
	seed := map[string]interface{}{
		"providers": map[string]interface{}{
			"ollama": map[string]interface{}{
				"baseUrl": "http://127.0.0.1:11434",
				"auth":    "none",
			},
		},
	}
	if err := WriteYAMLFile(path, seed); err != nil {
		t.Fatal(err)
	}

	existing, err := ReadYAMLFile(path)
	if err != nil {
		t.Fatal(err)
	}
	providers := existing["providers"].(map[string]interface{})
	providers[OmpProviderID] = BuildOmpTokenBankProvider(
		"https://tb.example/v1", "k", []ModelInfo{{ID: "m1", Name: "M1"}},
	)
	existing["providers"] = providers
	if err := WriteYAMLFile(path, existing); err != nil {
		t.Fatal(err)
	}

	// Backup should exist after second write
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Fatalf("expected .bak after overwrite: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := yaml.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	provs := got["providers"].(map[string]interface{})
	if _, ok := provs["ollama"]; !ok {
		t.Fatal("existing ollama provider was wiped")
	}
	tb, ok := provs[OmpProviderID].(map[string]interface{})
	if !ok {
		t.Fatal("tokenbank-proxy missing")
	}
	if tb["baseUrl"] != "https://tb.example/v1" {
		t.Fatalf("baseUrl = %v", tb["baseUrl"])
	}
}

func TestReadYAMLFileMissing(t *testing.T) {
	got, err := ReadYAMLFile(filepath.Join(t.TempDir(), "nope.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("want empty map, got %v", got)
	}
}

// TestReplaceOwnedProvider_PrunesStaleModels pins the reason this exists:
// DeepMerge alone only adds, so a model TokenBank stopped returning stayed in
// the local config forever and kept being offered by the agent.
func TestReplaceOwnedProvider_PrunesStaleModels(t *testing.T) {
	existing := map[string]interface{}{
		"model": "opencode-admin/kept",
		"provider": map[string]interface{}{
			"opencode-admin": map[string]interface{}{
				"models": map[string]interface{}{
					"kept":    map[string]interface{}{"name": "Kept"},
					"retired": map[string]interface{}{"name": "Retired upstream"},
				},
			},
			"someone-else": map[string]interface{}{
				"models": map[string]interface{}{"mine": map[string]interface{}{}},
			},
		},
	}
	fresh := map[string]interface{}{
		"provider": map[string]interface{}{
			"opencode-admin": map[string]interface{}{
				"models": map[string]interface{}{
					"kept": map[string]interface{}{"name": "Kept"},
				},
			},
		},
	}

	merged := DeepMerge(existing, fresh)
	replaceOwnedProvider(merged, fresh, "provider", "opencode-admin")

	provider := merged["provider"].(map[string]interface{})
	admin := provider["opencode-admin"].(map[string]interface{})
	models := admin["models"].(map[string]interface{})
	if _, stale := models["retired"]; stale {
		t.Errorf("a model absent from the API response must be removed, got %v", models)
	}
	if _, ok := models["kept"]; !ok {
		t.Errorf("a model the API still returns must survive, got %v", models)
	}
	if _, ok := provider["someone-else"]; !ok {
		t.Error("providers ywai does not own must be preserved")
	}
	if merged["model"] != "opencode-admin/kept" {
		t.Errorf("unrelated top-level keys must survive, got %v", merged["model"])
	}
}

// TestReplaceOwnedProvider_NoProviderInResponse guards the fail-safe: an API
// response without the owned provider must not wipe the local one.
func TestReplaceOwnedProvider_NoProviderInResponse(t *testing.T) {
	merged := map[string]interface{}{
		"provider": map[string]interface{}{
			"opencode-admin": map[string]interface{}{"models": map[string]interface{}{"a": map[string]interface{}{}}},
		},
	}
	replaceOwnedProvider(merged, map[string]interface{}{}, "provider", "opencode-admin")

	provider := merged["provider"].(map[string]interface{})
	if _, ok := provider["opencode-admin"]; !ok {
		t.Error("a response with no provider section must leave the local config alone")
	}
}

// TestEntryVendors_OnlyManagedVendors pins which Copilot entries ywai may
// prune: only those under a vendor the API response itself manages.
func TestEntryVendors_OnlyManagedVendors(t *testing.T) {
	got := entryVendors([]interface{}{
		map[string]interface{}{"vendor": "Token Bank", "name": "a"},
		map[string]interface{}{"vendor": "Token Bank", "name": "b"},
		map[string]interface{}{"name": "no-vendor"},
		"not-a-map",
	})
	if len(got) != 1 || !got["Token Bank"] {
		t.Errorf("entryVendors = %v, want just the Token Bank vendor", got)
	}
}

// TestConfigureOpenCode_DropsModelsAbsentFromGET is the user-facing contract:
// `ywai tokenbank configure` must write ~/.config/opencode/opencode.json (not
// an OPENCODE_CONFIG_DIR isolate) and the models there must match GET
// /v1/models, even when GET /api/setup/config still lists extras. The flavor
// is pinned to v1 because the v2 shape is covered by
// TestConfigureOpenCode_WritesV2Providers.
func TestConfigureOpenCode_DropsModelsAbsentFromGET(t *testing.T) {
	pinOCFlavor(t, "opencode")
	home := isolateOpenCodeConfig(t)

	userPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(userPath), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := map[string]interface{}{
		"model": "opencode-admin/kept",
		"provider": map[string]interface{}{
			"opencode-admin": map[string]interface{}{
				"models": map[string]interface{}{
					"kept":    map[string]interface{}{"name": "Kept"},
					"retired": map[string]interface{}{"name": "Stale local"},
				},
			},
		},
	}
	raw, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userPath, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/models":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"object": "list",
				"data": []map[string]interface{}{
					{"id": "kept", "name": "Kept", "limit": map[string]int{"context": 1000}},
				},
			})
		case "/api/setup/config":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"origin": "http://tokenbank.test",
				"config": map[string]interface{}{
					"provider": map[string]interface{}{
						"opencode-admin": map[string]interface{}{
							"npm":  "@ai-sdk/openai-compatible",
							"name": "Token Bank Proxy",
							"models": map[string]interface{}{
								"kept":  map[string]interface{}{"name": "Kept"},
								"ghost": map[string]interface{}{"name": "In config GET, not models GET"},
							},
						},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	if err := ConfigureOpenCode(srv.URL, "pk-test"); err != nil {
		t.Fatalf("ConfigureOpenCode: %v", err)
	}

	got, err := ReadJSONFile(userPath)
	if err != nil {
		t.Fatal(err)
	}
	models := got["provider"].(map[string]interface{})["opencode-admin"].(map[string]interface{})["models"].(map[string]interface{})
	if _, stale := models["retired"]; stale {
		t.Errorf("stale local model must be removed from %s, got %v", userPath, models)
	}
	if _, ghost := models["ghost"]; ghost {
		t.Errorf("a model from /config but not GET /models must be removed, got %v", models)
	}
	if _, ok := models["kept"]; !ok {
		t.Errorf("GET /models catalog entry must survive, got %v", models)
	}
}

// TestConfigureOpenCode_WritesV2Providers pins the v2 shape. TokenBank only
// serves the v1 `provider` payload (npm/options/variants map); v2 ignores npm
// and fails with "Unsupported package", and drops models whose variants are a
// map or whose fields are null. So ywai must translate it into `providers`.
func TestConfigureOpenCode_WritesV2Providers(t *testing.T) {
	pinOCFlavor(t, "opencode2")
	home := isolateOpenCodeConfig(t)
	userPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(userPath), 0o755); err != nil {
		t.Fatal(err)
	}
	// A v1 copy left by an older ywai run must go; other v2 providers stay.
	seed := `{"provider":{"opencode-admin":{"npm":"@ai-sdk/openai-compatible","models":{"stale":{}}}},` +
		`"providers":{"someone-else":{"package":"@opencode/ai/providers/openai","models":{}}}}`
	if err := os.WriteFile(userPath, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/models":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"object": "list",
				"data": []map[string]interface{}{
					{"id": "kept", "name": "Kept", "limit": map[string]int{"context": 1000}},
					{"id": "bare", "name": "Bare"},
				},
			})
		case "/api/setup/config":
			// The real TokenBank payload: v1 `provider` shape.
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"origin": "http://tokenbank.test",
				"config": map[string]interface{}{
					"provider": map[string]interface{}{
						"opencode-admin": map[string]interface{}{
							"npm":     "@ai-sdk/openai-compatible",
							"name":    "Token Bank Proxy",
							"options": map[string]interface{}{"baseURL": "http://tokenbank.test/v1", "apiKey": "pk-test"},
							"models": map[string]interface{}{
								"kept": map[string]interface{}{
									"name":     "Kept",
									"variants": map[string]interface{}{"high": map[string]interface{}{"reasoningEffort": "high"}},
								},
								"bare":  map[string]interface{}{"name": "Bare"},
								"ghost": map[string]interface{}{"name": "In config GET, not models GET"},
							},
						},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	if err := ConfigureOpenCode(srv.URL, "pk-test"); err != nil {
		t.Fatalf("ConfigureOpenCode: %v", err)
	}

	got, err := ReadJSONFile(userPath)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["provider"]; ok {
		t.Fatalf("v2 config must not keep a v1 provider key, got:\n%s", raw)
	}
	providers, ok := got["providers"].(map[string]interface{})
	if !ok {
		t.Fatalf("v2 config must hold a providers map, got:\n%s", raw)
	}
	if _, ok := providers["someone-else"]; !ok {
		t.Errorf("providers ywai does not own must be preserved, got:\n%s", raw)
	}
	admin, ok := providers["opencode-admin"].(map[string]interface{})
	if !ok {
		t.Fatalf("opencode-admin provider missing under providers, got:\n%s", raw)
	}
	if admin["package"] != "@opencode/ai/providers/openai-compatible" {
		t.Errorf("v2 provider must select its SDK via package, got:\n%s", raw)
	}
	for _, v1 := range []string{"npm", "options"} {
		if _, ok := admin[v1]; ok {
			t.Errorf("v2 provider must not keep v1 %q, got:\n%s", v1, raw)
		}
	}
	settings, _ := admin["settings"].(map[string]interface{})
	if settings["baseURL"] != "http://tokenbank.test/v1" || settings["apiKey"] != "pk-test" {
		t.Errorf("v1 options must move to settings, got:\n%s", raw)
	}

	models := admin["models"].(map[string]interface{})
	if _, ghost := models["ghost"]; ghost {
		t.Errorf("a model from /config but not GET /models must be removed, got %v", models)
	}
	kept, ok := models["kept"].(map[string]interface{})
	if !ok {
		t.Fatalf("GET /models catalog entry must survive, got %v", models)
	}
	if limit, _ := kept["limit"].(map[string]interface{}); limit["context"] != float64(1000) { // read back from disk: JSON numbers are float64
		t.Errorf("kept model must receive injected limits, got %v", kept)
	}
	caps, _ := kept["capabilities"].(map[string]interface{})
	if caps["tools"] != true {
		t.Errorf("v2 model must declare capabilities.tools, got %v", kept)
	}
	if in, _ := caps["input"].([]interface{}); len(in) != 1 || in[0] != "text" {
		t.Errorf("v2 capabilities.input must follow the model modalities, got %v", kept)
	}
	variants, _ := kept["variants"].([]interface{})
	if len(variants) != 1 {
		t.Fatalf("v2 variants must be an array, got %v", kept["variants"])
	}
	high, _ := variants[0].(map[string]interface{})
	if hs, _ := high["settings"].(map[string]interface{}); high["id"] != "high" || hs["reasoningEffort"] != "high" {
		t.Errorf("v2 variant must be {id, settings}, got %v", high)
	}
	bare, _ := models["bare"].(map[string]interface{})
	for k, v := range bare {
		if v == nil {
			t.Errorf("v2 model fields must never be null (v2 drops the model), %q is null in %v", k, bare)
		}
	}
}
