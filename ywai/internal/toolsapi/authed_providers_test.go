package toolsapi

import "testing"

func TestEnvKeyProvidersFrom(t *testing.T) {
	cfg := []byte(`{
		"$schema": "https://opencode.ai/config.json",
		"provider": {
			"tokenharbor": {
				"name": "Token Harbor",
				"npm": "@ai-sdk/openai-compatible",
				"options": {"apiKey": "{env:TOKENHARBOR_API_KEY}", "baseURL": "https://tokenharbor.ai/v1"}
			},
			"static-key": {
				"name": "Static",
				"options": {"apiKey": "sk-hardcoded"}
			},
			"missing-var": {
				"name": "Missing",
				"options": {"apiKey": "{env:DEFINITELY_NOT_SET_VAR_XYZ}"}
			}
		}
	}`)
	getenv := func(key string) string {
		if key == "TOKENHARBOR_API_KEY" {
			return "th-secret"
		}
		return ""
	}
	got := envKeyProvidersFrom(cfg, getenv)
	if len(got) != 1 || got[0] != "tokenharbor" {
		t.Fatalf("envKeyProvidersFrom() = %v, want [tokenharbor]", got)
	}
}

func TestEnvKeyProvidersFromInvalidJSON(t *testing.T) {
	if got := envKeyProvidersFrom([]byte("not json"), func(string) string { return "x" }); got != nil {
		t.Fatalf("envKeyProvidersFrom(invalid) = %v, want nil", got)
	}
}
