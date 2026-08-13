package main

// graft_install_pipeline_test.go — RED test for slice 2:
//
// Acceptance item 3 says the install/update pipeline must not call
// the legacy CodeGraph entry points (InstallCodegraphCLI,
// WireCodegraphMCP, CodegraphInfo). Per the slice contract we
// observe the absence of an action rather than the presence of a
// new one: the slice-2 dev must remove every reference to those
// functions from the install/update path.
//
// This test reads the cmd/ywai source tree and asserts none of the
// install-path files reference the legacy entry points. The doctor
// command is intentionally OUT of scope — slice-2 keeps it intact
// because doctor is not the install/update pipeline.
//
// The test is RED on the current tree (root.go calls
// plugins.WireCodegraphMCP / plugins.InstallCodegraphCLI /
// plugins.CodegraphInfo). Once those references are removed, GREEN.

import (
	"os"
	"strings"
	"testing"
)

// installPipelineFiles returns the cmd/ywai source files that
// implement the install/update command. We intentionally scope the
// scan to root.go (the install executor) and the install/update
// subcommand handlers — not commands.go (doctor) or apply.go.
//
// Why hand-pick? cmd/ywai/commands.go holds `ywai doctor` which
// legitimately calls CodegraphInfo for the readiness report. The
// slice-2 contract is about the install/update pipeline, not the
// doctor diagnostic. Hard-coding the file list keeps the test
// honest: an accidental `CodegraphInfo()` re-add to the install
// path is caught, but a doctor refactor is not.
func installPipelineFiles(t *testing.T) []string {
	t.Helper()
	return []string{
		"root.go",
		"apply.go",
	}
}

// TestInstallPipeline_NoCodegraphEntryPoints scans the install/update
// pipeline source files and asserts none of them reference the
// legacy codegraph plugin entry points. A future regression that
// re-calls them — for example, when the dev is migrating the call
// sites — fails here before it ships.
func TestInstallPipeline_NoCodegraphEntryPoints(t *testing.T) {
	legacy := []string{
		"plugins.InstallCodegraphCLI",
		"plugins.WireCodegraphMCP",
		"plugins.CodegraphInfo",
	}

	// Find the cmd/ywai directory. The test runs with cwd = ywai/
	// because the package path is ./cmd/ywai/... — so the source
	// files sit at "./<file>" relative to this test's runtime cwd.
	for _, file := range installPipelineFiles(t) {
		path := file
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		body := string(data)
		for _, ref := range legacy {
			if strings.Contains(body, ref) {
				t.Errorf("%s still references %q — slice 2 must drop it from the install/update pipeline",
					path, ref)
			}
		}
	}
}
