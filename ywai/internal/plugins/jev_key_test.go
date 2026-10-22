package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureJevKey(t *testing.T) {
	t.Run("writes the env key when there is none stored", func(t *testing.T) {
		isolateHome(t)
		path, wrote, err := EnsureJevKey("apikey_test")
		if err != nil {
			t.Fatalf("EnsureJevKey() error = %v", err)
		}
		if !wrote {
			t.Fatal("expected the key to be written")
		}
		var parsed map[string]string
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(body, &parsed); err != nil {
			t.Fatalf("key file is not JSON: %v", err)
		}
		if parsed["apiKey"] != "apikey_test" {
			t.Errorf("apiKey = %q, want the env key", parsed["apiKey"])
		}
	})

	t.Run("never overwrites a key the user edited", func(t *testing.T) {
		isolateHome(t)
		path := JevKeyPath()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(`{"apiKey":"mine"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		_, wrote, err := EnsureJevKey("apikey_from_env")
		if err != nil {
			t.Fatalf("EnsureJevKey() error = %v", err)
		}
		if wrote {
			t.Error("an existing key file must survive a re-run of install")
		}
		if got := readFile(t, path); !strings.Contains(got, "mine") {
			t.Errorf("key file was clobbered: %s", got)
		}
	})

	t.Run("no env key is not an error, just no file", func(t *testing.T) {
		isolateHome(t)
		path, wrote, err := EnsureJevKey("")
		if err != nil || wrote {
			t.Fatalf("EnsureJevKey(\"\") = %v, %v", wrote, err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Error("no key means no file")
		}
	})
}

func TestJevKeyStatus_NeverPrintsTheKey(t *testing.T) {
	isolateHome(t)
	path, _, err := EnsureJevKey("apikey_supersecret")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{JevKeyStatus(path, true), JevKeyStatus(path, false)} {
		if strings.Contains(line, "apikey_supersecret") {
			t.Errorf("status line leaks the key: %s", line)
		}
	}

	missing := filepath.Join(t.TempDir(), JevKeyFileName)
	if got := JevKeyStatus(missing, false); !strings.Contains(got, "NO API KEY") {
		t.Errorf("a missing key must be stated plainly, got %q", got)
	}
}
