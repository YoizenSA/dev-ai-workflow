package tokenbank

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

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
	})

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

	injectModelLimits(config, []ModelInfo{{ID: "kept", Name: "Kept", MaxInputTokens: 1000}})

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

	injectModelLimits(config, nil)

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
// /v1/models, even when GET /api/setup/config still lists extras.
func TestConfigureOpenCode_DropsModelsAbsentFromGET(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OPENCODE_CONFIG_DIR", filepath.Join(t.TempDir(), "orca-isolate"))

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
