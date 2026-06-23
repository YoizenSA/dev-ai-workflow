package mcp

// installer.go — end-to-end orchestrator for the "Real MCP Install" flow.
//
// Pipeline (each step gated by a Progress callback, monotonically
// increasing percent, ending at 100):
//
//  1. StepPrereq      — verify the tooling/runtime binary is on PATH.
//  2. StepInstalling  — run entry.InstallCmd via `sh -c` (skipped when
//                       InstallCmd is empty: nothing to install for
//                       already-installed or remote entries).
//  3. StepProbing     — run DiscoverStdio or DiscoverHTTP, confirm the
//                       server actually speaks MCP and returns tools.
//  4. StepConfiguring — persist the entry to the target agent's config.
//  5. StepFinalizing  — terminal step, percent=100.
//
// Every step that can fail returns a wrapped sentinel (ErrPrereqMissing,
// ErrInstallTimeout, …) so callers can dispatch on errors.Is without
// parsing the error message.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	StepPrereq      = "prereq_check"
	StepInstalling  = "installing"
	StepProbing     = "probing"
	StepConfiguring = "configuring"
	StepFinalizing  = "finalizing"
)

var (
	ErrPrereqMissing     = errors.New("prereq_missing")
	ErrInstallTimeout    = errors.New("install_timeout")
	ErrInstallNonZero    = errors.New("install_nonzero")
	ErrProbeTimeout      = errors.New("probe_timeout")
	ErrProbeNoTools      = errors.New("probe_no_tools")
	ErrProbeUnreachable  = errors.New("probe_unreachable")
	ErrConfigWriteFailed = errors.New("config_write_failed")
)

const installOutputTailLines = 20

// InstallOptions controls a single Install invocation.
type InstallOptions struct {
	// Credentials are injected as KEY=VALUE on top of os.Environ() for
	// the install subprocess and the probe subprocess. Optional.
	Credentials map[string]string
	// Progress is called at each pipeline boundary with a stable step
	// name, a monotonically non-decreasing percent, and a human-readable
	// message. nil means no progress reporting.
	Progress func(step string, percent int, message string)
	// Target is the agent config target ("opencode", "pi", "claude-code").
	// Required; an empty Target fails fast before the pipeline runs.
	Target string
	// EntryID is the catalog entry's ID, used as the key in the agent's
	// mcp/mcpServers section.
	EntryID string
}

// lockedBuf is a goroutine-safe bytes.Buffer for capturing concurrent
// stdout+stderr from a subprocess. bytes.Buffer is not safe for
// concurrent Write; exec.Cmd writes stdout and stderr from separate
// goroutines internally.
type lockedBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuf) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// Tail returns the last n lines of the captured output. Used to surface
// the install command's stderr in the error message when the install
// exits non-zero.
func (b *lockedBuf) Tail(n int) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := b.buf.String()
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

// Install runs the full install pipeline for a single catalog entry.
// Returns the discovered tool list on success.
//
// Pipeline order:
//
//	prereq_check → installing → probing → configuring → finalizing
//
// Each step is gated by a Progress callback (if opts.Progress is non-nil).
// Errors wrap one of the sentinel errors declared above; callers should
// use errors.Is to discriminate.
func Install(ctx context.Context, entry CatalogEntry, opts InstallOptions) ([]string, error) {
	if opts.Target == "" {
		return nil, fmt.Errorf("install: target is required")
	}

	progress := opts.Progress
	if progress == nil {
		progress = func(string, int, string) {}
	}

	// 1. Prereq check.
	binary := prereqBinary(entry)
	if binary != "" {
		progress(StepPrereq, 10, fmt.Sprintf("checking prereq for %s", binary))
		if _, err := exec.LookPath(binary); err != nil {
			return nil, fmt.Errorf("%s not found in PATH. Install %s from %s: %w",
				binary, entry.Name, entry.Docs, ErrPrereqMissing)
		}
	} else {
		progress(StepPrereq, 10, fmt.Sprintf("checking prereq for %s", entry.Name))
	}

	// 2. Install step (skipped when InstallCmd is empty).
	if entry.InstallCmd != "" {
		progress(StepInstalling, 20, "running InstallCmd")
		installCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
		defer cancel()
		var out lockedBuf
		cmd := exec.CommandContext(installCtx, "sh", "-c", entry.InstallCmd)
		cmd.Env, _, _ = MergeEnv(os.Environ(), opts.Credentials, secretsFromEntry(entry))
		cmd.Stdout = &out
		cmd.Stderr = &out

		// Poner el árbol en su propio process group para que SIGKILL al líder
		// propague al grupo completo y no queden nietos huérfanos sosteniendo
		// los write-ends de stdout/stderr (causa hang de cmd.Wait).
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Cancel = func() error {
			if p := cmd.Process; p != nil {
				_ = syscall.Kill(-p.Pid, syscall.SIGKILL)
			}
			return os.ErrProcessDone
		}
		// Red de seguridad: si por alguna razón el grupo no muere, forzar
		// que cmd.Wait retorne después de un corto período.
		cmd.WaitDelay = 2 * time.Second

		// Detach stdin from the parent so install commands that read
		// stdin (e.g. an MCP stdio server used as a test fixture) do
		// not block waiting for input. The install step does not need
		// stdin.
		if devNull, err := os.Open(os.DevNull); err == nil {
			cmd.Stdin = devNull
			defer devNull.Close()
		}
		if err := cmd.Run(); err != nil {
			if errors.Is(installCtx.Err(), context.DeadlineExceeded) {
				return nil, fmt.Errorf("install command timed out: %w", ErrInstallTimeout)
			}
			return nil, fmt.Errorf("install command failed: %s: %w",
				strings.TrimSpace(out.Tail(installOutputTailLines)), ErrInstallNonZero)
		}
	}

	// 3. Probe step.
	progress(StepProbing, 70, "verifying MCP responds")
	probeEnvSlice, _, _ := MergeEnv(os.Environ(), opts.Credentials, secretsFromEntry(entry))
	probeEnvMap := envSliceToMap(probeEnvSlice)
	var tools []string
	var probeErr error
	switch entry.Type {
	case "remote":
		tools, probeErr = DiscoverHTTP(ctx, entry.URL)
	case "local":
		tools, probeErr = DiscoverStdio(ctx, entry.Command, probeEnvMap)
	default:
		return nil, fmt.Errorf("install: unknown entry type %q", entry.Type)
	}
	if probeErr != nil {
		if errors.Is(probeErr, context.DeadlineExceeded) || errors.Is(probeErr, context.Canceled) {
			return nil, fmt.Errorf("probe timed out: %w", ErrProbeTimeout)
		}
		return nil, fmt.Errorf("probe failed: %w", ErrProbeUnreachable)
	}
	if len(tools) == 0 {
		return nil, fmt.Errorf("probe returned no tools: %w", ErrProbeNoTools)
	}

	// 4. Configure step.
	progress(StepConfiguring, 95, fmt.Sprintf("writing config to %s", opts.Target))
	shape := BuildEntryShape(opts.Target, entry, opts.Credentials)
	if _, err := WriteAgentConfig(opts.Target, opts.EntryID, shape); err != nil {
		return nil, fmt.Errorf("write agent config: %w", ErrConfigWriteFailed)
	}

	// 5. Finalize.
	progress(StepFinalizing, 100, "done")
	return tools, nil
}

// prereqBinary returns the binary the prereq check should verify is on
// PATH. If entry.InstallCmd is set, the first word of InstallCmd is
// returned (the tooling binary: npx, go, uvx, etc.) because the runtime
// binary the install step is producing (e.g. `engram`, the result of
// `go install ...`) does not exist yet. If InstallCmd is empty, the
// first element of entry.Command is returned (the already-installed
// runtime binary). Returns "" only when neither field is set (remote
// entries with no local binary).
func prereqBinary(entry CatalogEntry) string {
	if entry.InstallCmd != "" {
		if fields := strings.Fields(entry.InstallCmd); len(fields) > 0 {
			return fields[0]
		}
	}
	if len(entry.Command) > 0 {
		return entry.Command[0]
	}
	return ""
}

// secretsFromEntry returns the names of all env vars in entry.RequiredEnv
// that are marked as secrets. These are passed to MergeEnv so the merge
// correctly counts which keys are sensitive (for log redaction downstream).
func secretsFromEntry(entry CatalogEntry) []string {
	var names []string
	for _, e := range entry.RequiredEnv {
		if e.Secret {
			names = append(names, e.Name)
		}
	}
	return names
}

// envSliceToMap converts a slice of "KEY=VALUE" entries (as produced by
// MergeEnv) to a map[string]string suitable for DiscoverStdio's env
// parameter.
func envSliceToMap(env []string) map[string]string {
	m := make(map[string]string, len(env))
	for _, e := range env {
		if eq := strings.IndexByte(e, '='); eq >= 0 {
			m[e[:eq]] = e[eq+1:]
		}
	}
	return m
}
