package plugins

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// writePkgJSON writes a package.json with the given version into dir.
func writePkgJSON(t *testing.T, dir, version string) {
	t.Helper()
	// Embed a node_modules-ish layout so EvalSymlinks + walk still works.
	pkg := map[string]any{"name": "@cioffinahuel/opencode-ado", "version": version}
	b, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), append(b, '\n'), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
}

func TestAdoVersionFromBinary_FindsPackageJSON(t *testing.T) {
	// Layout: <tmp>/pkg/dist/bin/ado.js  →  package.json at <tmp>/pkg/
	root := t.TempDir()
	binDir := filepath.Join(root, "pkg", "dist", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	bin := filepath.Join(binDir, "ado.js")
	if err := os.WriteFile(bin, []byte("#!/usr/bin/env node\n"), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}
	writePkgJSON(t, filepath.Join(root, "pkg"), "0.8.10")

	got, err := adoVersionFromBinary(bin)
	if err != nil {
		t.Fatalf("adoVersionFromBinary error: %v", err)
	}
	if got != "0.8.10" {
		t.Fatalf("version = %q, want 0.8.10", got)
	}
}

func TestAdoVersionFromBinary_FollowsSymlink(t *testing.T) {
	// Real binary lives under the npm node_modules tree; the PATH entry is a
	// symlink (e.g. ~/.nvm/.../bin/ado → .../node_modules/@pkg/dist/bin/ado.js).
	root := t.TempDir()
	realDir := filepath.Join(root, "nvm", "lib", "node_modules", "@cioffinahuel", "opencode-ado", "dist", "bin")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatalf("mkdir real: %v", err)
	}
	realBin := filepath.Join(realDir, "opencode-ado.js")
	if err := os.WriteFile(realBin, []byte("#!/usr/bin/env node\n"), 0o755); err != nil {
		t.Fatalf("write real bin: %v", err)
	}
	writePkgJSON(t, filepath.Join(root, "nvm", "lib", "node_modules", "@cioffinahuel", "opencode-ado"), "0.8.10")

	linkDir := filepath.Join(root, "nvm", "bin")
	if err := os.MkdirAll(linkDir, 0o755); err != nil {
		t.Fatalf("mkdir link: %v", err)
	}
	link := filepath.Join(linkDir, "ado")
	if err := os.Symlink(realBin, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	got, err := adoVersionFromBinary(link)
	if err != nil {
		t.Fatalf("adoVersionFromBinary error: %v", err)
	}
	if got != "0.8.10" {
		t.Fatalf("version = %q, want 0.8.10", got)
	}
}

func TestAdoVersionFromBinary_NoPackageJSON(t *testing.T) {
	// A stray binary with no package.json above it (e.g. the old manual Go
	// build at /usr/local/bin/ado) — should error, not panic.
	root := t.TempDir()
	bin := filepath.Join(root, "ado")
	if err := os.WriteFile(bin, []byte("not a node script"), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}
	if _, err := adoVersionFromBinary(bin); err == nil {
		t.Fatal("expected error when no package.json is reachable, got nil")
	}
}

func TestAdoCLILatestVersion_UsesInjectedFn(t *testing.T) {
	// Stub the registry call so the test never hits the network.
	orig := adoLatestFn
	adoLatestFn = func() (string, error) { return "0.8.11", nil }
	defer func() { adoLatestFn = orig }()

	got, err := AdoCLILatestVersion()
	if err != nil {
		t.Fatalf("AdoCLILatestVersion error: %v", err)
	}
	if got != "0.8.11" {
		t.Fatalf("latest = %q, want 0.8.11", got)
	}
}

func TestAdoCLILatestVersion_PropagatesError(t *testing.T) {
	orig := adoLatestFn
	adoLatestFn = func() (string, error) { return "", fmt.Errorf("offline") }
	defer func() { adoLatestFn = orig }()

	if _, err := AdoCLILatestVersion(); err == nil {
		t.Fatal("expected error from injected registry failure, got nil")
	}
}

func TestReadVersionFromPackageJSON(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		dir := t.TempDir()
		writePkgJSON(t, dir, "1.2.3")
		got, err := readVersionFromPackageJSON(filepath.Join(dir, "package.json"))
		if err != nil || got != "1.2.3" {
			t.Fatalf("got (%q, %v), want (1.2.3, nil)", got, err)
		}
	})
	t.Run("missing version field", func(t *testing.T) {
		dir := t.TempDir()
		// Deliberately no "version" key.
		b, _ := json.Marshal(map[string]any{"name": "x"})
		path := filepath.Join(dir, "package.json")
		_ = os.WriteFile(path, b, 0o644)
		if _, err := readVersionFromPackageJSON(path); err == nil {
			t.Fatal("expected error for missing version field, got nil")
		}
	})
	t.Run("missing file", func(t *testing.T) {
		if _, err := readVersionFromPackageJSON(filepath.Join(t.TempDir(), "nope.json")); err == nil {
			t.Fatal("expected error for missing file, got nil")
		}
	})
}
