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

func TestFilterVisionModels(t *testing.T) {
	in := []ModelInfo{
		{ID: "a", Vision: false},
		{ID: "b", Vision: true},
		{ID: "c", Modalities: &ModelModalities{Input: []string{"text", "image"}}},
	}
	got := FilterVisionModels(in)
	if len(got) != 2 {
		t.Fatalf("len=%d, want 2", len(got))
	}
	if got[0].ID != "b" || got[1].ID != "c" {
		t.Fatalf("got ids %q %q", got[0].ID, got[1].ID)
	}
}

func TestResolveVisionModelID(t *testing.T) {
	catalog := []ModelInfo{{ID: "first"}, {ID: "second"}}

	if got := ResolveVisionModelID("", catalog); got != "first" {
		t.Fatalf("empty preferred → first catalog id, got %q", got)
	}
	if got := ResolveVisionModelID("second", catalog); got != "second" {
		t.Fatalf("preferred in catalog, got %q", got)
	}
	if got := ResolveVisionModelID("opencode-admin/second", catalog); got != "second" {
		t.Fatalf("strip provider prefix, got %q", got)
	}
	// Preferred not in catalog → fall back to first
	if got := ResolveVisionModelID("unknown", catalog); got != "first" {
		t.Fatalf("unknown preferred falls back to first, got %q", got)
	}
	// Empty catalog + preferred → trust preferred
	if got := ResolveVisionModelID("custom", nil); got != "custom" {
		t.Fatalf("empty catalog trusts preferred, got %q", got)
	}
	if got := ResolveVisionModelID("", nil); got != "" {
		t.Fatalf("nothing available, got %q", got)
	}
}
