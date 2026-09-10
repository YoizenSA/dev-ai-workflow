package testsandbox_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Any package that resolves an OpenCode config path itself can write into the
// developer's live config when its tests run. Isolating them one at a time has
// already failed twice — the sandbox went into internal/mcp, and internal/
// configapi went on rewriting the real file until someone noticed their MCP
// servers had changed. This finds the next one before a person does.
func TestEveryConfigResolvingPackageIsSandboxed(t *testing.T) {
	root := repoRoot(t)

	// Packages whose non-test code resolves a config path for itself.
	resolvers := map[string]bool{}
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(src), "EntryTargetPath(") {
			resolvers[filepath.Dir(path)] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(resolvers) == 0 {
		t.Fatal("found no package resolving a config path — this check is looking in the wrong place")
	}

	for dir := range resolvers {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		hasTests, sandboxed := false, false
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			hasTests = true
			src, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatalf("read %s: %v", e.Name(), err)
			}
			if strings.Contains(string(src), "testsandbox.Isolate(") {
				sandboxed = true
			}
		}
		if hasTests && !sandboxed {
			t.Errorf("%s resolves a real config path and has tests, but no TestMain calls testsandbox.Isolate — its tests can rewrite the developer's live OpenCode config",
				mustRel(t, root, dir))
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd() // .../ywai/internal/testsandbox
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Dir(filepath.Dir(wd))
}

func mustRel(t *testing.T, base, target string) string {
	t.Helper()
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}
	return rel
}
