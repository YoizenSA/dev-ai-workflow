package plugins

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CodegraphNPMPackage is the npm package that ships the `codegraph` CLI.
// Upstream: https://github.com/colbymchenry/codegraph
const CodegraphNPMPackage = "@colbymchenry/codegraph"

// codegraphInstallURL is the official install.sh used by codegraph's primary
// (Node-free) installer. ywai pipes it through sh; the script writes versioned
// installs under ~/.codegraph/versions/ and symlinks the binary onto PATH.
const codegraphInstallURL = "https://raw.githubusercontent.com/colbymchenry/codegraph/main/install.sh"

// CodegraphInfo reports whether the `codegraph` CLI is on PATH and, if so, its
// version as printed by `codegraph version`. installed is decided by
// exec.LookPath; version is "" when the binary exists but does not report a
// parseable version (e.g. an older build without the version subcommand).
func CodegraphInfo() (version string, installed bool) {
	exe, err := exec.LookPath("codegraph")
	if err != nil {
		return "", false
	}
	v, _ := codegraphVersionFromBinary(exe)
	return v, true
}

// codegraphVersionFromBinary runs `<exe> version` and returns the trimmed
// first line of output. Extracted for testability (see codegraph_cli_test.go).
// It does not parse the "Update available" banner line that newer codegraph
// versions append after the bare version.
func codegraphVersionFromBinary(exe string) (string, error) {
	out, err := exec.Command(exe, "version").Output()
	if err != nil {
		return "", err
	}
	return parseCodegraphVersion(out)
}

// parseCodegraphVersion returns the first non-empty line of `codegraph version`
// output. Newer builds append an "Update available" banner that must not be
// parsed as the version. Kept separate from the exec call so it stays unit
// testable without a platform-specific mock binary.
func parseCodegraphVersion(out []byte) (string, error) {
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		return line, nil
	}
	return "", fmt.Errorf("no version line in codegraph version output")
}

// codegraphResolveBin returns the `codegraph` binary to invoke. It prefers the
// binary on PATH; when that fails (e.g. the curl-installer just ran but the new
// binary is not yet on the current process's PATH), it falls back to the
// well-known ~/.local/bin/codegraph symlink that the installer writes. Returns
// "" when nothing usable is found.
func codegraphResolveBin() string {
	if exe, err := exec.LookPath("codegraph"); err == nil {
		return exe
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidate := filepath.Join(home, ".local", "bin", "codegraph")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

// InstallCodegraphCLI installs the `codegraph` CLI. It is non-fatal: a missing
// tool or a failed install returns an error the caller should surface as a
// warning, pointing the user at the manual install step.
//
// Install order (mirrors the engram brew + release fallback):
//  1. Idempotent short-circuit: if `codegraph` is already on PATH, do nothing.
//  2. Primary: the official curl-installer (Node-free). Writes versioned
//     installs under ~/.codegraph/versions/.
//  3. Fallback: `npm i -g @colbymchenry/codegraph`. Requires Node/npm.
//
// If both paths fail, the returned error names both attempts so the caller can
// print a single actionable warning.
func InstallCodegraphCLI() error {
	// Idempotent short-circuit: if `codegraph` is already on PATH, do nothing.
	if _, err := exec.LookPath("codegraph"); err == nil {
		return nil
	}

	// Primary: curl-installer (no Node required).
	if exe, err := exec.LookPath("curl"); err == nil {
		_ = exe // curl present; run the pipe regardless of its exact path.
		cmd := exec.Command("sh", "-c", "curl -fsSL "+codegraphInstallURL+" | sh")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err == nil {
			if _, lookErr := exec.LookPath("codegraph"); lookErr == nil {
				return nil
			}
			// Installer ran but the binary is not on the current PATH (it
			// typically tells the user to open a new terminal). Try the
			// well-known symlink before declaring failure.
			if home, herr := os.UserHomeDir(); herr == nil {
				if _, serr := os.Stat(filepath.Join(home, ".local", "bin", "codegraph")); serr == nil {
					return nil
				}
			}
		}
	}

	// Fallback: npm global install.
	if _, err := exec.LookPath("npm"); err != nil {
		return fmt.Errorf("codegraph install failed: curl-installer did not put it on PATH and npm is not installed — install Node.js, then run `npm i -g %s`", CodegraphNPMPackage)
	}
	cmd := exec.Command("npm", "i", "-g", CodegraphNPMPackage)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm install %s failed: %w", CodegraphNPMPackage, err)
	}
	return nil
}

// WireCodegraphMCP wires the codegraph MCP server (and its AGENTS.md marker
// section) into an agent's config by delegating to codegraph's own installer:
//
//	codegraph install --target=opencode --yes --location=global
//
// ywai intentionally does NOT write the mcp.codegraph entry or the AGENTS.md
// section itself — codegraph owns its config shape and its instruction surface.
// Non-fatal: returns an error for the caller to surface as a warning.
//
// If the codegraph binary cannot be resolved, the error tells the user to run
// InstallCodegraphCLI first.
func WireCodegraphMCP() error {
	exe := codegraphResolveBin()
	if exe == "" {
		return fmt.Errorf("codegraph binary not found — install it first (it should have been installed in the previous step)")
	}
	cmd := exec.Command(exe, "install", "--target=opencode", "--yes", "--location=global")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("codegraph install --target=opencode failed: %w", err)
	}
	return nil
}
