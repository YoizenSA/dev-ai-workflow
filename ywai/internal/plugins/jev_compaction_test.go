package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSetCompactionThreshold_KeepsKeyAndMode(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path := JevKeyPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"apiKey":"k"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := SetCompactionThreshold(1000000); err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	body, _ := os.ReadFile(path)
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got["apiKey"] != "k" || got["compactionThresholdTokens"] != float64(1000000) {
		t.Fatalf("unexpected file: %s", body)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
	if n, _ := CompactionThreshold(); n != 1000000 {
		t.Fatalf("CompactionThreshold = %d", n)
	}
}

func TestCompactionThreshold_DefaultWithoutFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if n, set := CompactionThreshold(); n != DefaultCompactionThreshold || set {
		t.Fatalf("got %d, %v", n, set)
	}
	if err := SetCompactionThreshold(0); err == nil {
		t.Fatal("want error for 0")
	}
}
