package tokenbank

import "testing"

func TestIsVisionModel(t *testing.T) {
	if IsVisionModel(ModelInfo{ID: "text-only", Vision: false}) {
		t.Fatal("expected false for vision=false without modalities")
	}
	if !IsVisionModel(ModelInfo{ID: "flagged", Vision: true}) {
		t.Fatal("expected true when vision=true")
	}
	if !IsVisionModel(ModelInfo{
		ID:         "from-modalities",
		Modalities: &ModelModalities{Input: []string{"text", "image"}},
	}) {
		t.Fatal("expected true when modalities include image")
	}
}
