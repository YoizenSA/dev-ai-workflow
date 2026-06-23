package mcp

// installer_test.go — TDD slice 4 of the "Real MCP Install" plan.
//
// These tests pin the contract of the new file ywai/internal/mcp/installer.go
// that will orchestrate the end-to-end MCP install flow:
//
//	prereq_check → installing → probing → configuring → finalizing
//
// RED: the file does not exist yet, so the test binary will not compile.
// The expected compile errors are:
//
//	undefined: Install
//	undefined: InstallOptions
//	undefined: StepPrereq
//	undefined: StepInstalling
//	undefined: StepProbing
//	undefined: StepConfiguring
//	undefined: StepFinalizing
//	undefined: ErrPrereqMissing
//	undefined: ErrInstallTimeout
//	undefined: ErrInstallNonZero
//	undefined: ErrProbeTimeout
//	undefined: ErrProbeNoTools
//	undefined: ErrProbeUnreachable
//	undefined: ErrConfigWriteFailed
//
// All symbols are pinned by the tests in this file. @dev's job is to add
// the implementation that makes them compile and pass.
//
// Test strategy:
//
//   - Install-side fakes (npx, go) are shell scripts written to t.TempDir().
//     This keeps the test fixtures cross-platform-friendly: the existing
//     fake_mcp_test.go uses a compiled Go binary for the probe side because
//     exec.Command cannot launch a bare .sh on Windows, but the install
//     side runs via `sh -c <InstallCmd>`, so the platform handling lives
//     in the implementation, not the test.
//   - Probe-side fake is the existing mcpfake binary (compiled Go program
//     from fake_mcp_test.go). The shell scripts delegate to it via the
//     MCPFAKE_SRC env var, so the same binary is reused across tests with
//     different per-test JSON spec files.
//   - Tests use stdlib only (no testify), live in `package mcp` so they
//     can reach unexported helpers @dev chooses to add, and follow the
//     conventions of the other *_test.go files in this package (per-test
//     banner comments, AAA structure, descriptive names).

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// ─── helpers ──────────────────────────────────────────────────────────────

// progressCall captures a single Progress callback invocation for later
// inspection. The test uses these to assert ordering, monotonicity, and
// the final percent==100 contract.
type progressCall struct {
	step    string
	percent int
	message string
}

// progressRecorder collects every Progress callback call behind a mutex.
// The implementer may choose to invoke Progress from a goroutine (e.g.
// while streaming subprocess output), so the recorder must be safe for
// concurrent use even if every test in this file today calls Install
// from a single goroutine.
type progressRecorder struct {
	mu    sync.Mutex
	calls []progressCall
}

func newProgressRecorder() *progressRecorder {
	return &progressRecorder{}
}

// callback returns the func(string,int,string) to pass as
// InstallOptions.Progress. It locks the mutex around the append.
func (r *progressRecorder) callback() func(string, int, string) {
	return func(step string, percent int, message string) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.calls = append(r.calls, progressCall{step, percent, message})
	}
}

// snapshot returns a copy of the recorded calls so the caller can iterate
// without holding the lock.
func (r *progressRecorder) snapshot() []progressCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]progressCall, len(r.calls))
	copy(out, r.calls)
	return out
}

// writeShellScript writes content to dir/name with mode 0o755 (executable).
// It is the single entry point for materializing install-side fakes: a
// shell stub that imitates the real binary it replaces (npx, go, etc.) for
// the duration of one test.
func writeShellScript(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	// #nosec G306 — test fixture must be executable on POSIX.
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write shell script %s: %v", path, err)
	}
	return path
}

// exeSuffix returns the platform-appropriate suffix for an executable file
// (".exe" on Windows, "" elsewhere). Mirrors the pattern in
// fake_mcp_test.go:fakeMCPExeName.
func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// installMcpfakeEnv sets MCPFAKE_SRC to the path of a freshly-built
// mcpfake binary whose behavior is described by spec. The install-side
// shell scripts read this env var to exec mcpfake for the probe-side
// MCP stdio behavior. Returns the temp dir the binary was written to.
//
// The mcpfake binary itself is NOT prepended to PATH here — only the
// shell scripts need to be in PATH. The implementation finds mcpfake
// via the full path stored in MCPFAKE_SRC.
func installMcpfakeEnv(t *testing.T, spec fakeMCPSpec) string {
	t.Helper()
	dir := writeFakeMCPBin(t, spec)
	t.Setenv("MCPFAKE_SRC", filepath.Join(dir, fakeMCPExeName()))
	return dir
}

// ─── shell-script templates ───────────────────────────────────────────────

// fakeNpxOK delegates every call to mcpfake. mcpfake performs the MCP
// stdio handshake and exits 0, so the install command ("npx -y @server")
// succeeds naturally. The probe then runs the same command and discovers
// the tools mcpfake was configured to advertise.
const fakeNpxOK = `#!/bin/sh
exec "$MCPFAKE_SRC" "$@"
`

// fakeNpxSleep blocks for 30 seconds. The install ctx (2s) fires first,
// surfacing ErrInstallTimeout.
const fakeNpxSleep = `#!/bin/sh
sleep 30
`

// fakeNpxExit1 exits with code 1 and writes a recognizable stderr message.
// The test pins that ErrInstallNonZero's Error() contains the stderr text
// so the install UI can show the user what went wrong.
const fakeNpxExit1 = `#!/bin/sh
echo "fake npx install failed: missing dependency" >&2
exit 1
`

// fakeNpxHang is the probe-side hang for the probe-fails test: it never
// responds to MCP stdio, so DiscoverStdio's ctx times out.
const fakeNpxHang = `#!/bin/sh
while true; do sleep 1; done
`

// fakeGoInstall imitates `go install <pkg>@<ver>` by copying mcpfake to
// $GOPATH/bin/<basename(pkg)>. The test uses this together with a
// CatalogEntry whose Command[0] is the basename, so the probe runs the
// created binary and gets MCP stdio.
const fakeGoInstall = `#!/bin/sh
pkg=""
for arg in "$@"; do
    pkg="$arg"
done
name=$(basename "${pkg%%@*}")
src_dir=$(dirname "$MCPFAKE_SRC")
mkdir -p "$GOPATH/bin"
cp "$MCPFAKE_SRC" "$GOPATH/bin/$name"
cp "$src_dir/mcp-fake-spec.json" "$GOPATH/bin/mcp-fake-spec.json"
chmod +x "$GOPATH/bin/$name"
exit 0
`

// fakeFakebinInstall is used by the probe-fails test. It distinguishes
// install-time (argv contains --install → exit 0) from probe-time (no
// args → hang forever). The CatalogEntry pins the install command as
// "fakebin --install" and the probe command as just "fakebin".
const fakeFakebinInstall = `#!/bin/sh
if [ "$1" = "--install" ]; then
    exit 0
fi
while true; do sleep 1; done
`

// ─── Test 1: TestInstall_LocalNpx_HappyPath ──────────────────────────────

// TestInstall_LocalNpx_HappyPath pins the happy path for a local entry
// whose install command is `npx -y <package>` and whose Command is
// `["npx", "-y", "<package>"]` (i.e. the catalog form used by playwright,
// git, github, etc.). The fake npx delegates to mcpfake, which does the
// MCP stdio handshake and exits 0. Install must return the tool list
// with no error.
func TestInstall_LocalNpx_HappyPath(t *testing.T) {
	installMcpfakeEnv(t, fakeMCPSpec{
		Mode:  "ok",
		Tools: []string{"tool1", "tool2"},
	})

	binDir := t.TempDir()
	writeShellScript(t, binDir, "npx", fakeNpxOK)
	prependFakeMCPPath(t, binDir)

	entry := CatalogEntry{
		ID:         "playwright",
		Name:       "Playwright",
		Type:       "local",
		Command:    []string{"npx", "-y", "fake-server"},
		InstallCmd: "npx -y fake-server",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tools, err := Install(ctx, entry, InstallOptions{
		Target:  "opencode",
		EntryID: "playwright",
	})
	if err != nil {
		t.Fatalf("Install(local npx happy path) unexpected error: %v", err)
	}
	slices.Sort(tools)
	want := []string{"tool1", "tool2"}
	if !slices.Equal(tools, want) {
		t.Errorf("Install(local npx) tools = %v, want %v", tools, want)
	}
}

// ─── Test 2: TestInstall_LocalGoInstall_HappyPath ────────────────────────

// TestInstall_LocalGoInstall_HappyPath pins the happy path for a local
// entry whose install command is `go install <pkg>@latest` (the catalog
// form used by engram and codegraph). The fake go copies mcpfake to
// $GOPATH/bin/<basename>, the install succeeds, the probe runs the
// created binary and discovers tools.
func TestInstall_LocalGoInstall_HappyPath(t *testing.T) {
	installMcpfakeEnv(t, fakeMCPSpec{
		Mode:  "ok",
		Tools: []string{"built-tool"},
	})

	gopath := t.TempDir()
	t.Setenv("GOPATH", gopath)

	binDir := t.TempDir()
	writeShellScript(t, binDir, "go", fakeGoInstall)
	// $GOPATH/bin must be in PATH so exec.LookPath("myserver") resolves
	// the binary that fake go install just created.
	t.Setenv("PATH",
		filepath.Join(gopath, "bin")+string(os.PathListSeparator)+
			binDir+string(os.PathListSeparator)+
			os.Getenv("PATH"))

	entry := CatalogEntry{
		ID:         "engram",
		Name:       "Engram",
		Type:       "local",
		Command:    []string{"myserver"},
		InstallCmd: "go install github.com/fake/myserver@latest",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tools, err := Install(ctx, entry, InstallOptions{
		Target:  "opencode",
		EntryID: "engram",
	})
	if err != nil {
		t.Fatalf("Install(go install) unexpected error: %v", err)
	}
	slices.Sort(tools)
	want := []string{"built-tool"}
	if !slices.Equal(tools, want) {
		t.Errorf("Install(go install) tools = %v, want %v", tools, want)
	}
}

// ─── Test 3: TestInstall_Remote_HappyPath ─────────────────────────────────

// TestInstall_Remote_HappyPath pins the happy path for a remote entry
// (Type="remote", URL set, InstallCmd empty). The install step is skipped
// (there is nothing to install for an HTTP endpoint). The probe hits the
// HTTP server and discovers tools.
func TestInstall_Remote_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"tools": []map[string]string{
					{"name": "remote-tool-1"},
					{"name": "remote-tool-2"},
				},
			},
		})
	}))
	defer srv.Close()

	entry := CatalogEntry{
		ID:         "context7",
		Name:       "Context7",
		Type:       "remote",
		URL:        srv.URL,
		InstallCmd: "",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tools, err := Install(ctx, entry, InstallOptions{
		Target:  "opencode",
		EntryID: "context7",
	})
	if err != nil {
		t.Fatalf("Install(remote happy path) unexpected error: %v", err)
	}
	slices.Sort(tools)
	want := []string{"remote-tool-1", "remote-tool-2"}
	if !slices.Equal(tools, want) {
		t.Errorf("Install(remote) tools = %v, want %v", tools, want)
	}
}

// ─── Test 4: TestInstall_PrereqMissing ────────────────────────────────────

// TestInstall_PrereqMissing pins the prereq-check failure path. When the
// required binary (npx, go, etc.) is not on PATH, Install must fail with
// ErrPrereqMissing *before* spawning any subprocess. We use an empty
// PATH (t.TempDir() with no executables) and a local entry whose
// Command starts with "npx". The prereq check is the first thing in
// the pipeline; the test asserts the sentinel error and a quick return.
func TestInstall_PrereqMissing(t *testing.T) {
	// Empty PATH: no npx, no go, nothing.
	t.Setenv("PATH", t.TempDir())

	entry := CatalogEntry{
		ID:         "playwright",
		Name:       "Playwright",
		Type:       "local",
		Command:    []string{"npx", "-y", "fake-server"},
		InstallCmd: "npx -y fake-server",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := Install(ctx, entry, InstallOptions{
		Target:  "opencode",
		EntryID: "playwright",
	})
	if !errors.Is(err, ErrPrereqMissing) {
		t.Errorf("Install(prereq missing) err = %v, want errors.Is(err, ErrPrereqMissing)", err)
	}
}

// ─── Test 5: TestInstall_InstallTimeout ───────────────────────────────────

// TestInstall_InstallTimeout pins the install-timeout failure path. The
// install command blocks for 30s; the ctx deadline is 2s. Install must
// fail with ErrInstallTimeout within a small grace period after the
// deadline. A 5s safety guard fails the test if Install does not
// respect the ctx.
func TestInstall_InstallTimeout(t *testing.T) {
	binDir := t.TempDir()
	writeShellScript(t, binDir, "npx", fakeNpxSleep)
	prependFakeMCPPath(t, binDir)

	entry := CatalogEntry{
		ID:         "playwright",
		Name:       "Playwright",
		Type:       "local",
		Command:    []string{"npx", "-y", "fake-server"},
		InstallCmd: "npx -y fake-server",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	type result struct {
		tools []string
		err   error
	}
	done := make(chan result, 1)
	go func() {
		tools, err := Install(ctx, entry, InstallOptions{
			Target:  "opencode",
			EntryID: "playwright",
		})
		done <- result{tools, err}
	}()

	select {
	case r := <-done:
		if !errors.Is(r.err, ErrInstallTimeout) {
			t.Errorf("Install(install timeout) err = %v, want errors.Is(err, ErrInstallTimeout)", r.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Install did not return within 5s after ctx timeout; install timeout is not being honored")
	}
}

// ─── Test 6: TestInstall_InstallNonZero ───────────────────────────────────

// TestInstall_InstallNonZero pins the install-nonzero-exit failure path.
// The install command exits 1 and writes a stderr message. Install must
// fail with ErrInstallNonZero, and the error message MUST include the
// stderr text — the install UI surfaces this directly to the user, so a
// silent "exit code 1" without context would be a UX regression.
func TestInstall_InstallNonZero(t *testing.T) {
	binDir := t.TempDir()
	writeShellScript(t, binDir, "npx", fakeNpxExit1)
	prependFakeMCPPath(t, binDir)

	entry := CatalogEntry{
		ID:         "playwright",
		Name:       "Playwright",
		Type:       "local",
		Command:    []string{"npx", "-y", "fake-server"},
		InstallCmd: "npx -y fake-server",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := Install(ctx, entry, InstallOptions{
		Target:  "opencode",
		EntryID: "playwright",
	})
	if !errors.Is(err, ErrInstallNonZero) {
		t.Errorf("Install(install nonzero) err = %v, want errors.Is(err, ErrInstallNonZero)", err)
	}
	if err != nil && !strings.Contains(err.Error(), "missing dependency") {
		t.Errorf("Install(install nonzero) err = %q, want it to contain the stderr "+
			"text %q so the install UI can show the user what went wrong",
			err.Error(), "missing dependency")
	}
}

// ─── Test 7: TestInstall_ProbeFails ───────────────────────────────────────

// TestInstall_ProbeFails pins the probe-failure path. The install
// succeeds (the install command exits 0), but the probe times out
// (the probe binary hangs and never responds to the MCP handshake).
// Install must fail with ErrProbeTimeout.
//
// To make the install and probe use different argv (so the same fake
// binary can exit 0 for the install and hang for the probe), the entry
// pins the install command as "fakebin --install" and the probe
// command as just "fakebin". The fake script branches on $1.
//
// Assumption: ErrProbeTimeout (not ErrProbeUnreachable) is the sentinel
// the implementer returns when the probe's ctx expires. ErrProbeUnreachable
// is reserved for network/transport failures (remote endpoints that
// refuse the connection). This matches the contract sketched in the
// slice 4 brief.
func TestInstall_ProbeFails(t *testing.T) {
	binDir := t.TempDir()
	writeShellScript(t, binDir, "fakebin", fakeFakebinInstall)
	prependFakeMCPPath(t, binDir)

	entry := CatalogEntry{
		ID:         "fakebin",
		Name:       "FakeBin",
		Type:       "local",
		Command:    []string{"fakebin"},
		InstallCmd: "fakebin --install",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	type result struct {
		tools []string
		err   error
	}
	done := make(chan result, 1)
	go func() {
		tools, err := Install(ctx, entry, InstallOptions{
			Target:  "opencode",
			EntryID: "fakebin",
		})
		done <- result{tools, err}
	}()

	select {
	case r := <-done:
		if !errors.Is(r.err, ErrProbeTimeout) {
			t.Errorf("Install(probe fails) err = %v, want errors.Is(err, ErrProbeTimeout)", r.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Install did not return within 5s after ctx timeout; probe timeout is not being honored")
	}
}

// ─── Test 8: TestInstall_SkipInstall ──────────────────────────────────────

// TestInstall_SkipInstall pins the skip-install path. When the entry has
// InstallCmd="" and the binary is already in PATH, the install step is
// skipped entirely. The probe runs the binary directly. The contract
// has two observable consequences:
//
//  1. Install returns the tools the binary advertises (no error).
//  2. Progress is NEVER called with StepInstalling — the install step
//     is fully bypassed, not just "the install command is empty".
//
// We use a copy of the mcpfake binary renamed to "somebin" as the
// already-installed binary. The platform-appropriate suffix is appended
// so the test works on Windows.
func TestInstall_SkipInstall(t *testing.T) {
	dir := writeFakeMCPBin(t, fakeMCPSpec{
		Mode:  "ok",
		Tools: []string{"already-installed"},
	})
	srcBin := filepath.Join(dir, fakeMCPExeName())

	binDir := t.TempDir()
	dstBin := filepath.Join(binDir, "somebin"+exeSuffix())
	if err := copyFile(srcBin, dstBin, 0o755); err != nil {
		t.Fatalf("copy mcpfake to somebin: %v", err)
	}
	srcSpec := filepath.Join(dir, "mcp-fake-spec.json")
	dstSpec := filepath.Join(binDir, "mcp-fake-spec.json")
	if err := copyFile(srcSpec, dstSpec, 0o644); err != nil {
		t.Fatalf("copy mcpfake spec: %v", err)
	}
	prependFakeMCPPath(t, binDir)

	rec := newProgressRecorder()

	entry := CatalogEntry{
		ID:         "somebin",
		Name:       "SomeBin",
		Type:       "local",
		Command:    []string{"somebin"},
		InstallCmd: "", // skip install
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tools, err := Install(ctx, entry, InstallOptions{
		Target:   "opencode",
		EntryID:  "somebin",
		Progress: rec.callback(),
	})
	if err != nil {
		t.Fatalf("Install(skip install) unexpected error: %v", err)
	}
	if !slices.Equal(tools, []string{"already-installed"}) {
		t.Errorf("Install(skip install) tools = %v, want [already-installed]", tools)
	}

	// Progress must NOT include StepInstalling. The install step is
	// fully skipped when InstallCmd is empty, not just "the command
	// is empty so we ran nothing".
	for _, call := range rec.snapshot() {
		if call.step == StepInstalling {
			t.Errorf("Install(skip install) called Progress with StepInstalling; " +
				"install step must be skipped when InstallCmd is empty")
		}
	}
}

// ─── Test 9: TestInstall_ProgressCallback ─────────────────────────────────

// TestInstall_ProgressCallback pins the Progress callback contract. The
// callback must:
//
//  1. Be called at least once.
//  2. Have percent values that are monotonic non-decreasing across calls.
//  3. End with percent == 100 on the final call.
//  4. Present steps in a sensible order: StepPrereq before StepProbing,
//     StepProbing before StepFinalizing. We don't pin exact positions
//     (the implementer is free to insert StepConfiguring between them),
//     only relative order.
//
// The exact set of steps is NOT pinned: the implementer may call
// StepConfiguring zero or one times, may call StepInstalling zero or
// more times within the install step, etc. What matters is the
// contract: ordered, monotonic, ends at 100.
func TestInstall_ProgressCallback(t *testing.T) {
	installMcpfakeEnv(t, fakeMCPSpec{
		Mode:  "ok",
		Tools: []string{"tool1"},
	})

	binDir := t.TempDir()
	writeShellScript(t, binDir, "npx", fakeNpxOK)
	prependFakeMCPPath(t, binDir)

	rec := newProgressRecorder()

	entry := CatalogEntry{
		ID:         "playwright",
		Name:       "Playwright",
		Type:       "local",
		Command:    []string{"npx", "-y", "fake-server"},
		InstallCmd: "npx -y fake-server",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := Install(ctx, entry, InstallOptions{
		Target:   "opencode",
		EntryID:  "playwright",
		Progress: rec.callback(),
	})
	if err != nil {
		t.Fatalf("Install(progress) unexpected error: %v", err)
	}

	calls := rec.snapshot()
	if len(calls) == 0 {
		t.Fatal("Install(progress) called Progress 0 times; want at least one call")
	}

	// Percent must be monotonic non-decreasing.
	for i := 1; i < len(calls); i++ {
		if calls[i].percent < calls[i-1].percent {
			t.Errorf("Install(progress) call[%d].percent = %d < call[%d].percent = %d "+
				"(percent must be monotonic non-decreasing)",
				i, calls[i].percent, i-1, calls[i-1].percent)
		}
	}

	// Last call must have percent == 100.
	last := calls[len(calls)-1]
	if last.percent != 100 {
		t.Errorf("Install(progress) last call percent = %d, want 100", last.percent)
	}

	// Steps must appear in order: prereq before probing, probing before
	// finalizing. We don't pin exact positions, only relative order.
	stepIndex := map[string]int{}
	for i, call := range calls {
		if _, seen := stepIndex[call.step]; !seen {
			stepIndex[call.step] = i
		}
	}
	// StepPrereq must appear before StepProbing.
	if pi, ok := stepIndex[StepPrereq]; ok {
		if pri, ok2 := stepIndex[StepProbing]; ok2 && pi > pri {
			t.Errorf("Install(progress) StepPrereq (call %d) appeared after "+
				"StepProbing (call %d); prereq must come first", pi, pri)
		}
	}
	// StepProbing must appear before StepFinalizing.
	if pi, ok := stepIndex[StepProbing]; ok {
		if fi, ok2 := stepIndex[StepFinalizing]; ok2 && pi > fi {
			t.Errorf("Install(progress) StepProbing (call %d) appeared after "+
				"StepFinalizing (call %d); probing must come before finalizing",
				pi, fi)
		}
	}
}

// ─── Test 10: TestInstall_EnvInjection ────────────────────────────────────

// TestInstall_EnvInjection pins that credentials passed via
// InstallOptions.Credentials are injected as environment variables into
// the child processes (install and probe). The mechanism is the same as
// TestDiscoverStdio_EnvInjection: the mcpfake binary is configured with
// EnvEcho:["MCP_TEST_TOKEN"]; if the env var reaches the child, its
// value appears as a tool with that name.
//
// This is a behavior pin: it does NOT assume the implementation
// strategy (os.Environ() + append, or a fresh env with overrides). It
// only asserts the externally visible consequence — the credential
// reaches the probe child and is observed as a tool.
func TestInstall_EnvInjection(t *testing.T) {
	installMcpfakeEnv(t, fakeMCPSpec{
		Mode:    "ok",
		Tools:   []string{"base-tool"},
		EnvEcho: []string{"MCP_TEST_TOKEN"},
	})

	binDir := t.TempDir()
	writeShellScript(t, binDir, "npx", fakeNpxOK)
	prependFakeMCPPath(t, binDir)

	entry := CatalogEntry{
		ID:         "playwright",
		Name:       "Playwright",
		Type:       "local",
		Command:    []string{"npx", "-y", "fake-server"},
		InstallCmd: "npx -y fake-server",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const secret = "abc123-token"
	tools, err := Install(ctx, entry, InstallOptions{
		Target:      "opencode",
		EntryID:     "playwright",
		Credentials: map[string]string{"MCP_TEST_TOKEN": secret},
	})
	if err != nil {
		t.Fatalf("Install(env injection) unexpected error: %v", err)
	}
	if !slices.Contains(tools, secret) {
		t.Errorf("Install(env injection) tools = %v, want to contain %q "+
			"(credentials must be injected into the child process env)",
			tools, secret)
	}
}

// ─── Test 11: TestInstall_TargetRequired ──────────────────────────────────

// TestInstall_TargetRequired pins the input-validation contract:
// opts.Target must be set. An empty target means the install doesn't
// know which agent config to write, so Install must fail with a non-nil
// error before running the pipeline (no subprocess spawn, no Progress
// callbacks). The exact error sentinel is NOT pinned — the test only
// asserts err != nil and that no tool list is returned.
func TestInstall_TargetRequired(t *testing.T) {
	installMcpfakeEnv(t, fakeMCPSpec{Mode: "ok", Tools: []string{"tool1"}})

	binDir := t.TempDir()
	writeShellScript(t, binDir, "npx", fakeNpxOK)
	prependFakeMCPPath(t, binDir)

	entry := CatalogEntry{
		ID:         "playwright",
		Name:       "Playwright",
		Type:       "local",
		Command:    []string{"npx", "-y", "fake-server"},
		InstallCmd: "npx -y fake-server",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tools, err := Install(ctx, entry, InstallOptions{
		Target:  "", // missing
		EntryID: "playwright",
	})
	if err == nil {
		t.Fatalf("Install(target=\"\") err = nil, tools = %v, want error (target is required)", tools)
	}
}

// ─── Test 12: TestInstall_ProbeUnreachable ─────────────────────────────────

// TestInstall_ProbeUnreachable pins the probe-failure path for transport
// errors that are NOT ctx timeouts. The contract distinguishes three
// probe-failure sentinels:
//
//   - ErrProbeTimeout     — ctx deadline exceeded (covered by TestInstall_ProbeFails).
//   - ErrProbeUnreachable — transport refused / DNS / network down (THIS test).
//   - ErrProbeNoTools     — probe succeeded but returned zero tools (Test 13).
//
// We trigger ErrProbeUnreachable by pointing entry.URL at a port that is
// guaranteed to be closed: bind a TCP listener on 127.0.0.1:0 to claim a
// random port, capture the address, then close the listener. The OS will
// refuse any subsequent connection to that address. The HTTP client returns
// a *url.Error whose cause is a net.OpError — neither context.DeadlineExceeded
// nor context.Canceled — so Install must wrap it as ErrProbeUnreachable, NOT
// ErrProbeTimeout. This pins the discriminant in the if-else inside Install.
func TestInstall_ProbeUnreachable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("open test listener: %v", err)
	}
	closedURL := "http://" + ln.Addr().String() + "/mcp"
	// Closing the listener guarantees the port is closed; subsequent TCP
	// connects to this address will get ECONNREFUSED.
	if err := ln.Close(); err != nil {
		t.Fatalf("close test listener: %v", err)
	}

	entry := CatalogEntry{
		ID:         "ghost",
		Name:       "Ghost",
		Type:       "remote",
		URL:        closedURL,
		InstallCmd: "",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tools, err := Install(ctx, entry, InstallOptions{
		Target:  "opencode",
		EntryID: "ghost",
	})
	if !errors.Is(err, ErrProbeUnreachable) {
		t.Errorf("Install(probe unreachable) err = %v, want errors.Is(err, ErrProbeUnreachable)", err)
	}
	if err != nil && errors.Is(err, ErrProbeTimeout) {
		t.Errorf("Install(probe unreachable) err = %v, must NOT be ErrProbeTimeout "+
			"(transport refused is distinct from ctx timeout)", err)
	}
	if tools != nil {
		t.Errorf("Install(probe unreachable) tools = %v, want nil on probe failure", tools)
	}
}

// ─── Test 13: TestInstall_ProbeNoTools ─────────────────────────────────────

// TestInstall_ProbeNoTools pins the empty-tool-list failure path. The
// existing TestInstall_ProbeFails covers the timeout case (probe never
// responds). This test covers the orthogonal case where the probe succeeds
// at the transport level — HTTP 200, valid JSON-RPC envelope — but the
// server advertises zero tools. The contract: Install must fail with
// ErrProbeNoTools so the install UI can tell the user "the server
// responded but doesn't expose any tools" (different from "the server
// is down"). The mcpfake binary's "ok" mode always returns at least one
// tool, so we use httptest to drive the response directly.
func TestInstall_ProbeNoTools(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Valid JSON-RPC 2.0 response with an empty tools array.
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"tools": []map[string]string{},
			},
		})
	}))
	defer srv.Close()

	entry := CatalogEntry{
		ID:         "empty",
		Name:       "Empty",
		Type:       "remote",
		URL:        srv.URL,
		InstallCmd: "",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tools, err := Install(ctx, entry, InstallOptions{
		Target:  "opencode",
		EntryID: "empty",
	})
	if !errors.Is(err, ErrProbeNoTools) {
		t.Errorf("Install(probe no tools) err = %v, want errors.Is(err, ErrProbeNoTools)", err)
	}
	if err != nil && errors.Is(err, ErrProbeUnreachable) {
		t.Errorf("Install(probe no tools) err = %v, must NOT be ErrProbeUnreachable "+
			"(server responded OK; tools were just empty)", err)
	}
	if tools != nil {
		t.Errorf("Install(probe no tools) tools = %v, want nil on no-tools failure", tools)
	}
}

// ─── Test 14: TestInstall_UnknownEntryType ─────────────────────────────────

// TestInstall_UnknownEntryType pins the defensive guard for malformed
// catalog entries. The contract: the Type field of a CatalogEntry is the
// only switch the probe step uses to decide which transport to invoke.
// Anything other than "local" or "remote" is undefined behavior from the
// catalog's point of view, and Install must fail fast with a clear error
// naming the offending value. The contract is intentionally simple — the
// error does NOT wrap a sentinel, because a malformed catalog entry is a
// caller bug, not a runtime condition. The test asserts:
//
//  1. err is non-nil.
//  2. The error message names the offending Type so the install UI can
//     surface it to the user / developer.
//
// The entry has no InstallCmd and an empty Command, so the prereq step
// goes through the "binary is empty" branch (no PATH lookup), the install
// step is skipped, and the probe step's switch hits `default` and returns
// the error. This is the only path that exercises that branch.
func TestInstall_UnknownEntryType(t *testing.T) {
	entry := CatalogEntry{
		ID:         "weird",
		Name:       "Weird",
		Type:       "weird", // not "local", not "remote"
		Command:    nil,     // empty → prereqBinary returns ""
		InstallCmd: "",      // skip install step
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tools, err := Install(ctx, entry, InstallOptions{
		Target:  "opencode",
		EntryID: "weird",
	})
	if err == nil {
		t.Fatalf("Install(type=weird) err = nil, tools = %v, want error (unknown entry type)", tools)
	}
	if !strings.Contains(err.Error(), "weird") {
		t.Errorf("Install(type=weird) err = %q, want it to contain the offending type "+
			"value %q so the install UI can surface it to the user", err.Error(), "weird")
	}
	if !strings.Contains(err.Error(), "unknown entry type") {
		t.Errorf("Install(type=weird) err = %q, want it to contain %q "+
			"so the failure mode is identifiable", err.Error(), "unknown entry type")
	}
}
