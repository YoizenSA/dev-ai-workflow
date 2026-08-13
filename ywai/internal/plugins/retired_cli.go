package plugins

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// codegraphNPMPackage is the retired CodeGraph indexer CLI. Graft replaced it,
// so ywai removes the leftover global install and per-repo index instead of
// leaving them to rot on disk.
const codegraphNPMPackage = "@colbymchenry/codegraph"

// npmRun is indirected so tests can stub the global uninstall.
var npmRun = func(ctx context.Context, args ...string) error {
	if _, err := exec.LookPath("npm"); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "npm", args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

// RemoveRetiredCLIs uninstalls the retired CodeGraph CLI globally and deletes
// its index directory under dir (the repo ywai runs in), when present. Both are
// best-effort cleanup: a missing npm, binary, or index is not an error. The
// returned names describe what was (or would be) removed.
func RemoveRetiredCLIs(dir string, dryRun bool) ([]string, error) {
	var removed []string

	if _, err := exec.LookPath("codegraph"); err == nil {
		removed = append(removed, codegraphNPMPackage+" (global npm)")
		if !dryRun {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if err := npmRun(ctx, "rm", "-g", codegraphNPMPackage); err != nil {
				return removed, fmt.Errorf("npm rm -g %s: %w", codegraphNPMPackage, err)
			}
		}
	}

	if dir != "" {
		index := filepath.Join(dir, ".codegraph")
		if _, err := os.Stat(index); err == nil {
			removed = append(removed, index)
			if !dryRun {
				if err := os.RemoveAll(index); err != nil {
					return removed, fmt.Errorf("remove %s: %w", index, err)
				}
			}
		}
	}

	return removed, nil
}
