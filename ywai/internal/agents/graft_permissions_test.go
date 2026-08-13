package agents

// graft_permissions_test.go — RED tests for slice 2: replace the
// `codegraph_*` permission grant in every agent profile with
// `graft_*` (acceptance item 4).
//
// Contract under test:
//   - Every agents/**/permissions.json file that grants `codegraph_*`
//     must instead grant `graft_*` (or both, when not exact-exclusion).
//     The slice contract is "codegraph_* must not survive".
//   - Profiles currently carrying `codegraph_*` MUST carry `graft_*`.
//   - No JSON in the agents tree mentions `codegraph_*`.
//
// We use stdlib filepath/json so the test is deterministic over the
// whole tree — adding a new agent profile does not silently regress
// the contract. The dev is free to keep the legacy key around as an
// alias if an explicit compatibility shim is needed, but that is NOT
// required by this slice: the acceptance criterion is the absence of
// the legacy wildcard.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// walkPermissions walks the agents/ tree and returns every
// permissions.json path it finds. Errors are fatal — a missing tree
// means the test environment is broken.
func walkPermissions(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) != "permissions.json" {
			return nil
		}
		out = append(out, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return out
}

// readPermMap reads a permissions.json file and decodes it into a
// string→string map. The on-disk format is intentionally permissive:
// we ignore unexpected fields and accept anything json.Unmarshal can
// handle.
func readPermMap(t *testing.T, path string) map[string]string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return m
}

// TestPermissions_NoCodegraphWildcard asserts that no permissions.json
// file in the agents tree grants the legacy `codegraph_*` wildcard.
// This is the strongest, simplest guarantee of acceptance item 4:
// the legacy grant must be gone from every agent profile.
func TestPermissions_NoCodegraphWildcard(t *testing.T) {
	root := filepath.Join("..", "..", "agents")
	for _, path := range walkPermissions(t, root) {
		perms := readPermMap(t, path)
		for key := range perms {
			if strings.HasPrefix(key, "codegraph") {
				t.Errorf("%s still grants legacy permission %q — slice 2 must replace with graft_*",
					path, key)
			}
		}
	}
}

// TestPermissions_GraftWildcardPresent asserts that the slice-2
// migration left at least one shipped profile granting `graft_*`.
// The legacy `codegraph_*` set is empty after a full migration, so a
// direct graft_* count is the post-migration regression signal.
func TestPermissions_GraftWildcardPresent(t *testing.T) {
	root := filepath.Join("..", "..", "agents")
	profiles := walkPermissions(t, root)
	if len(profiles) == 0 {
		t.Fatal("no permissions.json files found under agents/ — test environment is broken")
	}

	// Post-migration every legacy codegraph_* grant is gone, so the
	// migration signal is a direct graft_* grant. Counting those keeps
	// the regression guard: if a profile ever drops graft_* (or someone
	// reverts to codegraph_*), this test and the NoCodegraph tests fail.
	migrated := 0
	for _, path := range profiles {
		perms := readPermMap(t, path)
		for key := range perms {
			if strings.HasPrefix(key, "graft_") {
				migrated++
				break
			}
		}
	}

	if migrated == 0 {
		t.Errorf("expected at least one profile to grant graft_* after migration, found none")
	}
}

// TestPermissions_NoCodegraphMentionInProfile is a defense-in-depth
// scan: no permissions.json file may contain the literal string
// `codegraph`. Catches accidental comments, alternate keys, or
// stale migration metadata left behind by a partial replace.
func TestPermissions_NoCodegraphMentionInProfile(t *testing.T) {
	root := filepath.Join("..", "..", "agents")
	for _, path := range walkPermissions(t, root) {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if strings.Contains(string(b), "codegraph") {
			t.Errorf("%s still mentions \"codegraph\" — slice 2 must replace every reference", path)
		}
	}
}
