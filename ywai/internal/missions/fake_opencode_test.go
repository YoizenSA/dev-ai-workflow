package missions

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

// fakeOpencodeSpec declares the runtime behavior of a fake opencode binary used
// by mission tests. The fake is a real compiled executable (not a shell script)
// so it behaves identically on Unix and Windows — exec.Command cannot launch a
// bare .sh or .cmd script directly, which is why the original shell-script fakes
// failed the Windows CI with "opencode binary not found in PATH".
type fakeOpencodeSpec struct {
	Stdout   string `json:"stdout"`    // bytes written to stdout before exiting
	DelaySec int    `json:"delay_sec"` // seconds to sleep before producing output
	ExitCode int    `json:"exit_code"` // process exit code
	Hang     bool   `json:"hang"`      // block forever (for cancel/timeout tests)
}

// fakeOpencodeStubSource is the program compiled into the fake opencode binary.
// It reads its behavior from opencode-spec.json next to the executable, so a
// single compiled binary is reused across tests with a per-test spec file.
const fakeOpencodeStubSource = "package main\n" + `
import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type spec struct {
	Stdout   string ` + "`json:\"stdout\"`" + `
	DelaySec int    ` + "`json:\"delay_sec\"`" + `
	ExitCode int    ` + "`json:\"exit_code\"`" + `
	Hang     bool   ` + "`json:\"hang\"`" + `
}

func main() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "fake opencode: locate self:", err)
		os.Exit(0)
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(exe), "opencode-spec.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "fake opencode: read spec:", err)
		os.Exit(0)
	}
	var s spec
	if err := json.Unmarshal(data, &s); err != nil {
		fmt.Fprintln(os.Stderr, "fake opencode: parse spec:", err)
		os.Exit(0)
	}
	if s.Hang {
		for {
			time.Sleep(time.Second)
		}
	}
	if s.DelaySec > 0 {
		time.Sleep(time.Duration(s.DelaySec) * time.Second)
	}
	if s.Stdout != "" {
		fmt.Print(s.Stdout)
	}
	os.Exit(s.ExitCode)
}
`

var (
	fakeOpencodeOnce    sync.Once
	fakeOpencodeBinPath string
	fakeOpencodeBinErr  error
)

// buildFakeOpencodeStub compiles the stub program once per test binary run and
// caches the resulting executable path for reuse.
func buildFakeOpencodeStub() (string, error) {
	fakeOpencodeOnce.Do(func() {
		buildDir, err := os.MkdirTemp("", "ywai-fake-opencode-build-*")
		if err != nil {
			fakeOpencodeBinErr = err
			return
		}
		if err := os.WriteFile(filepath.Join(buildDir, "main.go"), []byte(fakeOpencodeStubSource), 0o644); err != nil {
			fakeOpencodeBinErr = err
			return
		}
		if err := os.WriteFile(filepath.Join(buildDir, "go.mod"), []byte("module fakeopencode\n\ngo 1.21\n"), 0o644); err != nil {
			fakeOpencodeBinErr = err
			return
		}
		exePath := filepath.Join(buildDir, fakeOpencodeExeName())
		cmd := exec.Command("go", "build", "-o", exePath, ".")
		cmd.Dir = buildDir
		if out, err := cmd.CombinedOutput(); err != nil {
			fakeOpencodeBinErr = fmt.Errorf("build fake opencode: %w: %s", err, out)
			return
		}
		fakeOpencodeBinPath = exePath
	})
	return fakeOpencodeBinPath, fakeOpencodeBinErr
}

// fakeOpencodeExeName returns the platform-appropriate executable name so
// exec.LookPath("opencode") resolves it via PATHEXT on Windows.
func fakeOpencodeExeName() string {
	if runtime.GOOS == "windows" {
		return "opencode.exe"
	}
	return "opencode"
}

// writeFakeOpencodeBin materializes a fake opencode executable implementing spec
// in a fresh temp dir and returns that dir. The caller is responsible for adding
// the dir to PATH (helpers differ on whether they set PATH themselves).
func writeFakeOpencodeBin(t *testing.T, spec fakeOpencodeSpec) string {
	t.Helper()
	stub, err := buildFakeOpencodeStub()
	if err != nil {
		t.Fatalf("compile fake opencode stub: %v", err)
	}
	dir := t.TempDir()

	// #nosec G306 -- test fixture must be executable
	if err := copyFile(stub, filepath.Join(dir, fakeOpencodeExeName()), 0o755); err != nil {
		t.Fatalf("copy fake opencode: %v", err)
	}
	specData, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal fake opencode spec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "opencode-spec.json"), specData, 0o644); err != nil {
		t.Fatalf("write fake opencode spec: %v", err)
	}
	return dir
}

func copyFile(src, dst string, mode os.FileMode) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, mode)
}
