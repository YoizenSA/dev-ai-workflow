package mcp

// fake_mcp_test.go — compiled-binary fixture for the discovery tests.
//
// This file is a TDD test helper. It is intentionally in the same package
// (not `_test`) so the stub can be shared with discovery_test.go. The stub
// itself is a Go program compiled via `go build` and cached, then copied to
// a per-test tempdir. Its behavior is driven by a JSON spec file written
// next to the executable (same pattern as internal/missions/fake_opencode_test.go).
//
// Why a compiled binary and not a shell script: the stdio MCP probe in the
// real codebase must run identically on Unix and Windows. exec.Command
// cannot launch a bare .sh or .cmd script directly, which is exactly why
// the missions package switched to a compiled stub. We follow the same
// precedent so the discovery tests do not regress on Windows CI.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

// fakeMCPSpec declares the runtime behavior of the fake MCP stdio server.
// The behavior is encoded as JSON and written next to the compiled stub,
// so a single binary is reused across all tests with a per-test spec file.
type fakeMCPSpec struct {
	// Mode selects the high-level behavior:
	//   "ok"               — respond to initialize + tools/list normally.
	//   "hang"             — block forever (used for timeout/cancel tests).
	//   "close_stdin"      — close stdin without responding to anything.
	//   "no_init_response" — read the initialize request but never reply.
	Mode string `json:"mode"`
	// Tools is the list of tool names to return in the tools/list response
	// (only used when Mode == "ok").
	Tools []string `json:"tools"`
	// EnvEcho is a list of env var names whose values should appear as
	// additional tools in the response. This is how TestDiscoverStdio_EnvInjection
	// observes that the env map was actually applied to the child process:
	// if MCP_TEST_TOKEN=abc123 is injected, a tool named "abc123" appears.
	EnvEcho []string `json:"env_echo"`
	// DelayMs sleeps this many milliseconds before writing each response.
	// Used to slow the OK path enough to let ctx timeouts fire.
	DelayMs int `json:"delay_ms"`
}

// fakeMCPStubSource is the Go program compiled into the fake stdio MCP
// server. The whole source is a single Go raw string with no embedded
// backticks — JSON responses are written using normal double-quoted Go
// strings so the outer raw string is unambiguous.
const fakeMCPStubSource = `package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type tool struct {
	Name string ` + "`json:\"name\"`" + `
}

type spec struct {
	Mode    string   ` + "`json:\"mode\"`" + `
	Tools   []string ` + "`json:\"tools\"`" + `
	EnvEcho []string ` + "`json:\"env_echo\"`" + `
	DelayMs int      ` + "`json:\"delay_ms\"`" + `
}

// readLine skips blank lines and returns the first non-empty line read from
// in. Returns "" on EOF.
func readLine(in *bufio.Reader) string {
	for {
		line, err := in.ReadString('\n')
		if err != nil {
			return ""
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		return line
	}
}

func main() {
	s := spec{Mode: "ok", Tools: []string{"tool1", "tool2"}}
	exe, err := os.Executable()
	if err == nil {
		if data, rerr := os.ReadFile(filepath.Join(filepath.Dir(exe), "mcp-fake-spec.json")); rerr == nil {
			_ = json.Unmarshal(data, &s)
		}
	}

	switch s.Mode {
	case "hang":
		for {
			time.Sleep(time.Second)
		}
	case "close_stdin":
		_ = os.Stdin.Close()
		return
	case "no_init_response":
		// Consume the initialize request then exit without writing anything.
		// The parent will hang on readJSONRPCResponse until the child dies.
		readLine(bufio.NewReader(os.Stdin))
		// Give the parent a moment to actually issue the read.
		time.Sleep(50 * time.Millisecond)
		os.Exit(0)
	}

	if s.DelayMs > 0 {
		time.Sleep(time.Duration(s.DelayMs) * time.Millisecond)
	}

	in := bufio.NewReader(os.Stdin)

	// 1. Read initialize request, write initialize response.
	readLine(in)
	fmt.Fprintln(os.Stdout, "{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"protocolVersion\":\"2024-11-05\",\"capabilities\":{},\"serverInfo\":{\"name\":\"fake\",\"version\":\"1.0\"}}}")
	os.Stdout.Sync()

	// 2. Read the initialized notification. We don't reply to it.
	readLine(in)

	// 3. Read the tools/list request, write the tools/list response.
	readLine(in)

	tools := make([]tool, 0, len(s.Tools)+len(s.EnvEcho))
	for _, n := range s.Tools {
		tools = append(tools, tool{Name: n})
	}
	for _, n := range s.EnvEcho {
		if v := os.Getenv(n); v != "" {
			tools = append(tools, tool{Name: v})
		}
	}
	payload, _ := json.Marshal(tools)
	fmt.Fprintf(os.Stdout, "{\"jsonrpc\":\"2.0\",\"id\":2,\"result\":{\"tools\":%s}}\n", payload)
	os.Stdout.Sync()
}
`

var (
	fakeMCPOnce    sync.Once
	fakeMCPBinPath string
	fakeMCPBinErr  error
)

// buildFakeMCPStub compiles the fake MCP stub once per test binary run and
// caches the resulting executable path for reuse. Same pattern as
// internal/missions/buildFakeOpencodeStub.
func buildFakeMCPStub() (string, error) {
	fakeMCPOnce.Do(func() {
		buildDir, err := os.MkdirTemp("", "ywai-fake-mcp-build-*")
		if err != nil {
			fakeMCPBinErr = err
			return
		}
		if err := os.WriteFile(filepath.Join(buildDir, "main.go"), []byte(fakeMCPStubSource), 0o644); err != nil {
			fakeMCPBinErr = err
			return
		}
		if err := os.WriteFile(filepath.Join(buildDir, "go.mod"), []byte("module fakemcp\n\ngo 1.21\n"), 0o644); err != nil {
			fakeMCPBinErr = err
			return
		}
		exePath := filepath.Join(buildDir, fakeMCPExeName())
		cmd := exec.Command("go", "build", "-o", exePath, ".")
		cmd.Dir = buildDir
		if out, err := cmd.CombinedOutput(); err != nil {
			fakeMCPBinErr = fmt.Errorf("build fake mcp stub: %w: %s", err, out)
			return
		}
		fakeMCPBinPath = exePath
	})
	return fakeMCPBinPath, fakeMCPBinErr
}

// fakeMCPExeName returns the platform-appropriate executable name so
// exec.LookPath("mcpfake") resolves it via PATHEXT on Windows.
func fakeMCPExeName() string {
	if runtime.GOOS == "windows" {
		return "mcpfake.exe"
	}
	return "mcpfake"
}

// writeFakeMCPBin materializes a fake MCP stdio server implementing spec
// in a fresh temp dir and returns that dir. The caller is responsible for
// adding the dir to PATH (use prependFakeMCPPath).
func writeFakeMCPBin(t *testing.T, spec fakeMCPSpec) string {
	t.Helper()
	stub, err := buildFakeMCPStub()
	if err != nil {
		t.Fatalf("compile fake mcp stub: %v", err)
	}
	dir := t.TempDir()

	// #nosec G306 -- test fixture must be executable on POSIX.
	if err := copyFile(stub, filepath.Join(dir, fakeMCPExeName()), 0o755); err != nil {
		t.Fatalf("copy fake mcp: %v", err)
	}
	specData, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal fake mcp spec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mcp-fake-spec.json"), specData, 0o644); err != nil {
		t.Fatalf("write fake mcp spec: %v", err)
	}
	return dir
}

// prependFakeMCPPath puts dir at the front of PATH for the duration of the
// test, so `exec.LookPath("mcpfake")` resolves the fake binary. Mirrors
// the helper used in internal/missions/integration_test.go.
func prependFakeMCPPath(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// copyFile is a small helper for copying a binary blob (the compiled stub)
// to a fresh destination with the given mode.
func copyFile(src, dst string, mode os.FileMode) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, mode)
}
