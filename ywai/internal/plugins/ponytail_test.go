package plugins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallPonytail_OpenCode(t *testing.T) {
	t.Run("creates_plugin_array_when_missing", func(t *testing.T) {
		path := writeAgentConfig(t, "opencode.json", map[string]any{})

		if err := InstallPonytail("opencode", path); err != nil {
			t.Fatalf("InstallPonytail() error = %v", err)
		}

		if arr := pluginArray(t, path); !containsString(arr, PonytailNPMPackage) {
			t.Errorf("plugin array %v does not contain %q", arr, PonytailNPMPackage)
		}
	})

	t.Run("preserves_existing_entries", func(t *testing.T) {
		path := writeAgentConfig(t, "opencode.json", map[string]any{
			"plugin": []any{"some-other-plugin"},
		})

		if err := InstallPonytail("kilocode", path); err != nil {
			t.Fatalf("InstallPonytail() error = %v", err)
		}

		arr := pluginArray(t, path)
		if !containsString(arr, "some-other-plugin") {
			t.Errorf("plugin array %v dropped pre-existing entry", arr)
		}
		if !containsString(arr, PonytailNPMPackage) {
			t.Errorf("plugin array %v does not contain %q", arr, PonytailNPMPackage)
		}
	})

	t.Run("idempotent_no_duplicate", func(t *testing.T) {
		path := writeAgentConfig(t, "opencode.json", map[string]any{})

		for i := 0; i < 2; i++ {
			if err := InstallPonytail("opencode", path); err != nil {
				t.Fatalf("InstallPonytail() call %d error = %v", i, err)
			}
		}

		arr := pluginArray(t, path)
		count := 0
		for _, v := range arr {
			if s, _ := v.(string); s == PonytailNPMPackage {
				count++
			}
		}
		if count != 1 {
			t.Errorf("plugin array %v contains %q %d times, want exactly 1", arr, PonytailNPMPackage, count)
		}
	})

	t.Run("already_present_left_alone", func(t *testing.T) {
		path := writeAgentConfig(t, "opencode.json", map[string]any{
			"plugin": []any{PonytailNPMPackage, "other"},
		})

		if err := InstallPonytail("opencode", path); err != nil {
			t.Fatalf("InstallPonytail() error = %v", err)
		}

		arr := pluginArray(t, path)
		if len(arr) != 2 {
			t.Errorf("plugin array length = %d, want 2 (no reshuffle/dup): %v", len(arr), arr)
		}
	})
}

func TestInstallPonytail_UnsupportedAgent(t *testing.T) {
	err := InstallPonytail("cursor", "/tmp/x.json")
	if err == nil {
		t.Fatal("expected error for unsupported agent")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Errorf("error = %v, want not supported", err)
	}
}

func TestSupportsPonytail(t *testing.T) {
	cases := map[string]bool{
		"opencode":    true,
		"kilocode":    true,
		"claude-code": true,
		"cursor":      false,
		"pi":          false,
	}
	for name, want := range cases {
		if got := SupportsPonytail(name); got != want {
			t.Errorf("SupportsPonytail(%q)=%v, want %v", name, got, want)
		}
	}
}

func TestInstallPonytail_Claude(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	// Fake claude: record full argv after the binary name.
	script := filepath.Join(dir, "claude")
	content := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"" + logPath + "\"\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write fake claude: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	prev := claudeCLI
	claudeCLI = "claude"
	t.Cleanup(func() { claudeCLI = prev })

	if err := InstallPonytail("claude-code", ""); err != nil {
		t.Fatalf("InstallPonytail(claude-code) error = %v", err)
	}

	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read call log: %v", err)
	}
	log := string(raw)
	wantLines := []string{
		"plugin marketplace add " + PonytailClaudeMarketplaceSource,
		"plugin install " + PonytailClaudePluginID + " -s user",
	}
	for _, want := range wantLines {
		if !strings.Contains(log, want) {
			t.Errorf("call log missing %q\nlog:\n%s", want, log)
		}
	}
}

func TestInstallPonytail_ClaudeMissingBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // empty of claude
	prev := claudeCLI
	claudeCLI = "claude-not-installed-xyz"
	t.Cleanup(func() { claudeCLI = prev })

	err := InstallPonytail("claude-code", "")
	if err == nil {
		t.Fatal("expected error when claude binary missing")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %v, want not found", err)
	}
}
