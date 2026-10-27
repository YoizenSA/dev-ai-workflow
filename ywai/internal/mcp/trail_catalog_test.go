package mcp

// trail_catalog_test.go — RED tests for slice 2: replace the runtime
// CodeGraph MCP catalog entry with the Trail MCP catalog entry.
//
// Contracts under test (acceptance items 1 & 2):
//   1. Catalog has ID "trail" with Name "Trail", Command exactly
//      ["trail","mcp"], InstallCmd "npm i -g @trailhq/trail-code",
//      Docs "https://github.com/trailhq/Trail-Code", and Tools exactly
//      covering trail_find_code, trail_trace_calls, trail_find_all,
//      trail_file_api, trail_repo_map, trail_check_freshness.
//   2. Catalog no longer exposes ID "codegraph".
//
// These tests assert against the public Catalog() / CatalogByID()
// functions so the dev can move the entry around freely without
// breaking them. They will be RED until the catalog var is updated.

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

// TestCatalogByID_Trail_PinnedContract pins every observable field of
// the trail catalog entry. Drift in any field (name, command, install,
// docs, tools) is a regression: agents and the install UI render from
// this struct directly.
func TestCatalogByID_Trail_PinnedContract(t *testing.T) {
	entry, ok := CatalogByID("trail")
	if !ok {
		t.Fatal("CatalogByID(trail) ok=false, want true (slice 2 contract)")
	}

	if entry.ID != "trail" {
		t.Errorf("trail.ID = %q, want %q", entry.ID, "trail")
	}
	if entry.Name != "Trail" {
		t.Errorf("trail.Name = %q, want %q", entry.Name, "Trail")
	}

	wantCmd := []string{"trail", "mcp"}
	if !reflect.DeepEqual(entry.Command, wantCmd) {
		t.Errorf("trail.Command = %#v, want %#v", entry.Command, wantCmd)
	}

	const wantInstall = "npm i -g @trailhq/trail-code"
	if entry.InstallCmd != wantInstall {
		t.Errorf("trail.InstallCmd = %q, want %q", entry.InstallCmd, wantInstall)
	}

	const wantDocs = "https://github.com/trailhq/Trail-Code"
	if entry.Docs != wantDocs {
		t.Errorf("trail.Docs = %q, want %q", entry.Docs, wantDocs)
	}

	wantTools := []string{
		"trail_check_freshness",
		"trail_file_api",
		"trail_find_all",
		"trail_find_code",
		"trail_repo_map",
		"trail_trace_calls",
	}
	gotTools := append([]string(nil), entry.Tools...)
	sort.Strings(gotTools)
	wantSorted := append([]string(nil), wantTools...)
	sort.Strings(wantSorted)

	if !reflect.DeepEqual(gotTools, wantSorted) {
		t.Errorf("trail.Tools = %#v (sorted), want %#v (sorted)", gotTools, wantSorted)
	}
}

// TestCatalogByID_Trail_ToolsCoverContract pins the tool-list shape
// without ordering. It checks every tool listed in the contract is
// present and no extras sneak in. Extras would expand the install UI
// beyond what the documentation promises; misses would break agents
// that rely on a documented tool.
func TestCatalogByID_Trail_ToolsCoverContract(t *testing.T) {
	entry, ok := CatalogByID("trail")
	if !ok {
		t.Fatal("CatalogByID(trail) ok=false, want true")
	}
	tools := make(map[string]bool, len(entry.Tools))
	for _, name := range entry.Tools {
		tools[name] = true
	}

	required := []string{
		"trail_find_code",
		"trail_trace_calls",
		"trail_find_all",
		"trail_file_api",
		"trail_repo_map",
		"trail_check_freshness",
	}
	for _, want := range required {
		if !tools[want] {
			t.Errorf("trail.Tools missing %q (got %#v)", want, entry.Tools)
		}
	}
}

// TestCatalogByID_Trail_NoCodegraphTools pins that the trail entry
// does NOT carry the legacy codegraph_* tool names. A stale
// migration that copies old tools into the new entry would render the
// install UI against tools the trail binary does not expose.
func TestCatalogByID_Trail_NoCodegraphTools(t *testing.T) {
	entry, ok := CatalogByID("trail")
	if !ok {
		t.Fatal("CatalogByID(trail) ok=false, want true")
	}
	for _, name := range entry.Tools {
		if strings.HasPrefix(name, "codegraph_") {
			t.Errorf("trail.Tools contains legacy %q (slice 2 must not ship codegraph tools)", name)
		}
	}
}

// TestCatalogByID_Codegraph_Removed pins that the legacy codegraph ID
// is gone from the public catalog. CatalogByID must return ok=false;
// if any code under ywai still queries it, it falls through to the
// "not found" path instead of silently running a retired server.
func TestCatalogByID_Codegraph_Removed(t *testing.T) {
	if _, ok := CatalogByID("codegraph"); ok {
		t.Errorf("CatalogByID(codegraph) ok=true, want false (slice 2 removed codegraph)")
	}
}

// TestCatalog_NoCodegraphID walks the whole catalog and asserts no
// entry has ID "codegraph". Defense in depth: a future entry could
// reintroduce the ID without breaking CatalogByID's specific lookup.
func TestCatalog_NoCodegraphID(t *testing.T) {
	for i, e := range Catalog() {
		if e.ID == "codegraph" {
			t.Errorf("Catalog()[%d].ID = %q, want absence (slice 2 removed codegraph)", i, e.ID)
		}
	}
}
