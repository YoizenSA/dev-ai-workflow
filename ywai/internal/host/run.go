package host

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// RunSpec is a non-interactive agent invocation for workflow runs.
type RunSpec struct {
	Host    ID
	Agent   string // e.g. myflow-orchestrator
	Model   string // optional provider/model
	Prompt  string
	WorkDir string
}

// Command builds the exec.Cmd for a one-shot run. The caller owns Start/Wait
// and PTY wrapping. Returns an error if the host binary is missing.
func Command(ctx context.Context, spec RunSpec) (*exec.Cmd, error) {
	bin := FindBinary(spec.Host)
	if bin == "" {
		return nil, fmt.Errorf("%s binary not found on PATH", spec.Host)
	}
	args, err := commandArgs(spec)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	if spec.WorkDir != "" {
		cmd.Dir = spec.WorkDir
	}
	return cmd, nil
}

// commandArgs builds the argv for a one-shot run, independent of the binary
// lookup so it can be unit-tested without a host CLI on PATH.
func commandArgs(spec RunSpec) ([]string, error) {
	prompt := strings.TrimSpace(spec.Prompt)
	if prompt == "" {
		prompt = "Run the workflow."
	}
	agent := strings.TrimSpace(spec.Agent)
	model := strings.TrimSpace(spec.Model)

	var args []string
	switch spec.Host {
	case OpenCode:
		args = []string{"run"}
		if agent != "" {
			args = append(args, "--agent", agent)
		}
		if model != "" {
			args = append(args, "--model", model)
		}
		args = append(args, prompt)

	case Pi:
		// pi has no --agent flag (it rejects unknown options). The orchestrator
		// markdown exported by ywai to ~/.pi/agent/agents/<agent>.md is loaded by
		// appending the file to the system prompt: --append-system-prompt reads
		// the file contents when the value is an existing path, and falls back to
		// treating it as literal text otherwise.
		if agent != "" {
			args = append(args, "--append-system-prompt", filepath.Join(AgentsDir(Pi), agent+".md"))
		}
		if model != "" {
			args = append(args, "--model", model)
		}
		args = append(args, "--print", prompt)

	case OMP:
		// omp mirrors pi: no --agent flag either, so the exported agent markdown
		// under ~/.omp/agent/agents/<agent>.md is appended via the same
		// --append-system-prompt file-content mechanism.
		if agent != "" {
			args = append(args, "--append-system-prompt", filepath.Join(AgentsDir(OMP), agent+".md"))
		}
		if model != "" {
			args = append(args, "--model", model)
		}
		args = append(args, "-p", prompt)

	default:
		return nil, fmt.Errorf("workflow run not supported on host %s", spec.Host)
	}

	return args, nil
}
