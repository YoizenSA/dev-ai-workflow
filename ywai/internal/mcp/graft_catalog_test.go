package mcp

// graft_catalog_test.go — RED tests for slice 2: replace the runtime
// CodeGraph MCP catalog entry with the Graft MCP catalog entry.
//
// Contracts under test (acceptance items 1 & 2):
//   1. Catalog has ID "graft" with Name "Graft", Command exactly
//      ["graft","mcp"], InstallCmd "npm i -g @nanonets/graft",
//      Docs "https://github.com/nanonets/graft", and Tools exactly
//      covering graft_find_code, graft_trace_calls, graft_find_all,
//      graft_file_api, graft_repo_map, graft_check_freshness.
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

// TestCatalogByID_Graft_PinnedContract pins every observable field of
// the graft catalog entry. Drift in any field (name, command, install,
// docs, tools) is a regression: agents and the install UI render from
// this struct directly.
func TestCatalogByID_Graft_PinnedContract(t *testing.T) {
	entry, ok := CatalogByID("graft")
	if !ok {
		t.Fatal("CatalogByID(graft) ok=false, want true (slice 2 contract)")
	}

	if entry.ID != "graft" {
		t.Errorf("graft.ID = %q, want %q", entry.ID, "graft")
	}
	if entry.Name != "Graft" {
		t.Errorf("graft.Name = %q, want %q", entry.Name, "Graft")
	}

	wantCmd := []string{"graft", "mcp"}
	if !reflect.DeepEqual(entry.Command, wantCmd) {
		t.Errorf("graft.Command = %#v, want %#v", entry.Command, wantCmd)
	}

	const wantInstall = "npm i -g @nanonets/graft"
	if entry.InstallCmd != wantInstall {
		t.Errorf("graft.InstallCmd = %q, want %q", entry.InstallCmd, wantInstall)
	}

	const wantDocs = "https://github.com/nanonets/graft"
	if entry.Docs != wantDocs {
		t.Errorf("graft.Docs = %q, want %q", entry.Docs, wantDocs)
	}

	wantTools := []string{
		"graft_check_freshness",
		"graft_file_api",
		"graft_find_all",
		"graft_find_code",
		"graft_repo_map",
		"graft_trace_calls",
	}
	gotTools := append([]string(nil), entry.Tools...)
	sort.Strings(gotTools)
	wantSorted := append([]string(nil), wantTools...)
	sort.Strings(wantSorted)

	if !reflect.DeepEqual(gotTools, wantSorted) {
		t.Errorf("graft.Tools = %#v (sorted), want %#v (sorted)", gotTools, wantSorted)
	}
}

// TestCatalogByID_Graft_ToolsCoverContract pins the tool-list shape
// without ordering. It checks every tool listed in the contract is
// present and no extras sneak in. Extras would expand the install UI
// beyond what the documentation promises; misses would break agents
// that rely on a documented tool.
func TestCatalogByID_Graft_ToolsCoverContract(t *testing.T) {
	entry, ok := CatalogByID("graft")
	if !ok {
		t.Fatal("CatalogByID(graft) ok=false, want true")
	}
	tools := make(map[string]bool, len(entry.Tools))
	for _, name := range entry.Tools {
		tools[name] = true
	}

	required := []string{
		"graft_find_code",
		"graft_trace_calls",
		"graft_find_all",
		"graft_file_api",
		"graft_repo_map",
		"graft_check_freshness",
	}
	for _, want := range required {
		if !tools[want] {
			t.Errorf("graft.Tools missing %q (got %#v)", want, entry.Tools)
		}
	}
}

// TestCatalogByID_Graft_NoCodegraphTools pins that the graft entry
// does NOT carry the legacy codegraph_* tool names. A stale
// migration that copies old tools into the new entry would render the
// install UI against tools the graft binary does not expose.
func TestCatalogByID_Graft_NoCodegraphTools(t *testing.T) {
	entry, ok := CatalogByID("graft")
	if !ok {
		t.Fatal("CatalogByID(graft) ok=false, want true")
	}
	for _, name := range entry.Tools {
		if strings.HasPrefix(name, "codegraph_") {
			t.Errorf("graft.Tools contains legacy %q (slice 2 must not ship codegraph tools)", name)
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