package opencode

import "testing"

func TestIsProviderCatalog(t *testing.T) {
	if IsProviderCatalog(nil) || IsProviderCatalog([]ModelInfo{}) {
		t.Fatal("empty list is not a provider catalog")
	}
	if !IsProviderCatalog([]ModelInfo{
		{ID: "opencode-admin", Name: "Token Bank Proxy"},
		{ID: "openai", Name: "OpenAI"},
	}) {
		t.Fatal("ids without a slash are a provider catalog")
	}
	if IsProviderCatalog([]ModelInfo{
		{ID: "opencode-admin/deepseek-v4-flash", Provider: "opencode-admin"},
	}) {
		t.Fatal("provider/model ids are a real catalog")
	}
}
