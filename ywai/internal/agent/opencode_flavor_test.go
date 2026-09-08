package agent

import (
	"os"
	"path/filepath"
	"testing"
)

// The env override is the escape hatch over the persisted setting, and both must
// beat autodetect. Without this, a machine with opencode2 on PATH silently wins
// every time and the v1 target becomes unreachable.
func TestOpenCodeBinaryNameHonorsOverride(t *testing.T) {
	// Isolate the stored config so the developer's real one is not read.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	for _, tc := range []struct {
		env  string
		want string
	}{
		{"v1", "opencode"},
		{"opencode", "opencode"},
		{"v2", "opencode2"},
		{"opencode2", "opencode2"},
		{"  V2  ", "opencode2"},
	} {
		t.Setenv(OpenCodeOverrideEnv, tc.env)
		if got := OpenCodeBinaryName(); got != tc.want {
			t.Errorf("YWAI_OPENCODE=%q: got %q, want %q", tc.env, got, tc.want)
		}
	}

	// Garbage falls through to autodetect rather than pinning a bogus binary.
	t.Setenv(OpenCodeOverrideEnv, "v9")
	if got := OpenCodeBinaryName(); got != "opencode" && got != "opencode2" {
		t.Errorf("unrecognised override: got %q, want an autodetected name", got)
	}
}

func TestOpenCodeIsV2TracksBinaryName(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(OpenCodeOverrideEnv, "v1")
	if OpenCodeIsV2() {
		t.Error("pinned to v1 but OpenCodeIsV2 reported true")
	}
	t.Setenv(OpenCodeOverrideEnv, "v2")
	if !OpenCodeIsV2() {
		t.Error("pinned to v2 but OpenCodeIsV2 reported false")
	}
}

func TestOpenCodeBinaryNameNeverEmpty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(OpenCodeOverrideEnv, "")
	t.Setenv("PATH", t.TempDir()) // neither binary installed
	if got := OpenCodeBinaryName(); got == "" {
		t.Error("callers that only need a name must never get an empty string")
	}
	_ = os.Getenv("PATH")
}

// `ywai config set opencode_version v2` and the web UI both land in the same
// config.yaml field, so the resolver must read it — and the env must still win
// over it, or a one-off YWAI_OPENCODE run would be silently ignored.
func TestOpenCodeBinaryNameReadsStoredSetting(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(OpenCodeOverrideEnv, "")

	if err := os.MkdirAll(filepath.Join(home, ".ywai"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(v string) {
		body := []byte("opencode_version: " + v + "\n")
		if err := os.WriteFile(filepath.Join(home, ".ywai", "config.yaml"), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("v1")
	if got := OpenCodeBinaryName(); got != "opencode" {
		t.Errorf("stored v1: got %q, want opencode", got)
	}
	write("v2")
	if got := OpenCodeBinaryName(); got != "opencode2" {
		t.Errorf("stored v2: got %q, want opencode2", got)
	}

	// Env beats the stored value.
	t.Setenv(OpenCodeOverrideEnv, "v1")
	if got := OpenCodeBinaryName(); got != "opencode" {
		t.Errorf("env must override the stored setting: got %q, want opencode", got)
	}
}
