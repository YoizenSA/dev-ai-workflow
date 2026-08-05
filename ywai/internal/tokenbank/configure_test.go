package tokenbank

import (
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
