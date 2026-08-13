package plugins

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/mcp"
)

func TestWireEngramMCP_WritesDetectedHosts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	putFakeEngramOnPATH(t)

	var setups []string
	orig := runEngramSetup
	runEngramSetup = func(agent string) error {
		setups = append(setups, agent)
		return nil
	}
	t.Cleanup(func() { runEngramSetup = orig })

	if err := WireEngramMCP([]string{"opencode", "pi", "omp", "claude-code"}); err != nil {
		t.Fatalf("WireEngramMCP: %v", err)
	}

	entry, ok := mcp.CatalogByID("engram")
	if !ok {
		t.Fatal("catalog missing engram")
	}

	for _, host := range []string{"opencode", "pi", "omp", "claude-code"} {
		section, err := mcp.ReadAgentConfig(host)
		if err != nil {
			t.Fatalf("ReadAgentConfig(%s): %v", host, err)
		}
		got, ok := section["engram"].(map[string]any)
		if !ok {
			t.Fatalf("%s missing engram entry: %#v", host, section)
		}
		want := mcp.BuildEntryShape(host, entry, nil)
		if !entryShapeEqual(got, want) {
			t.Errorf("%s engram shape = %#v, want %#v", host, got, want)
		}
	}

	if !slices.Equal(setups, []string{"opencode", "pi"}) {
		t.Fatalf("engram setup calls = %v, want [opencode pi]", setups)
	}
}

func TestWireEngramMCP_SkipsSetupWhenPresent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	putFakeEngramOnPATH(t)

	origSetup := runEngramSetup
	origPresent := engramSetupPresent
	runEngramSetup = func(string) error { t.Fatal("setup must not run"); return nil }
	engramSetupPresent = func(string) bool { return true }
	t.Cleanup(func() {
		runEngramSetup = origSetup
		engramSetupPresent = origPresent
	})

	if err := WireEngramMCP([]string{"opencode", "pi"}); err != nil {
		t.Fatalf("WireEngramMCP: %v", err)
	}
}

func TestWireEngramMCP_SkipsUnsupportedHosts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	putFakeEngramOnPATH(t)

	orig := runEngramSetup
	runEngramSetup = func(string) error { t.Fatal("setup must not run"); return nil }
	t.Cleanup(func() { runEngramSetup = orig })

	if err := WireEngramMCP([]string{"cursor", "kilocode"}); err == nil {
		t.Fatal("WireEngramMCP() err = nil, want error when no supported host")
	}
}

func TestWireEngramMCP_RequiresBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	err := WireEngramMCP([]string{"opencode"})
	if err == nil {
		t.Fatal("WireEngramMCP() err = nil, want missing-binary error")
	}
}

func putFakeEngramOnPATH(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake PATH binary is a shell script")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "engram"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func entryShapeEqual(got, want map[string]any) bool {
	if len(got) != len(want) {
		return false
	}
	for k, wv := range want {
		gv, ok := got[k]
		if !ok {
			return false
		}
		switch w := wv.(type) {
		case []any:
			g, ok := gv.([]any)
			if !ok || len(g) != len(w) {
				return false
			}
			for i := range w {
				if g[i] != w[i] {
					return false
				}
			}
		default:
			if gv != wv {
				return false
			}
		}
	}
	return true
}
