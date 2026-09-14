package envprofile

import (
	"encoding/json"
	"os"
	"testing"
)

func readServiceCfg(t *testing.T, p Profile) map[string]any {
	t.Helper()
	data, err := os.ReadFile(ServiceConfigFile(p))
	if err != nil {
		t.Fatal(err)
	}
	cfg := map[string]any{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestCreatePinsServicePort(t *testing.T) {
	testRoot(t)
	p, err := Create("dev", "dev")
	if err != nil {
		t.Fatal(err)
	}
	if got := readServiceCfg(t, p)["port"]; got != float64(p.Port+servicePortOffset) {
		t.Errorf("service port = %v, want %d", got, p.Port+servicePortOffset)
	}
}

func TestEnsureServicePortKeepsKeysAndCustomPort(t *testing.T) {
	testRoot(t)
	p, err := Create("dev", "dev")
	if err != nil {
		t.Fatal(err)
	}
	// opencode's own password key must survive; the shared default port is
	// rewritten because it collides with the global service.
	if err := os.WriteFile(ServiceConfigFile(p), []byte(`{"password":"x","port":49374}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EnsureServicePort(p); err != nil {
		t.Fatal(err)
	}
	cfg := readServiceCfg(t, p)
	if cfg["password"] != "x" || cfg["port"] != float64(ServicePort(p)) {
		t.Errorf("cfg = %v", cfg)
	}
	// A hand-picked port stays.
	if err := os.WriteFile(ServiceConfigFile(p), []byte(`{"port":7777}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EnsureServicePort(p); err != nil {
		t.Fatal(err)
	}
	if got := readServiceCfg(t, p)["port"]; got != float64(7777) {
		t.Errorf("custom port overwritten: %v", got)
	}
}
