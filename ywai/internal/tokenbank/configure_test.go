package tokenbank

import (
	"testing"
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
