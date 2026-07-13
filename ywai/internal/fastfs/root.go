package fastfs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Root bounds all filesystem access to a single workspace directory.
type Root struct {
	// Abs is the cleaned absolute workspace path (no trailing separator).
	Abs string
}

// NewRoot resolves workspace to an absolute directory. Empty uses cwd.
func NewRoot(workspace string) (*Root, error) {
	if workspace == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		workspace = wd
	}
	abs, err := filepath.Abs(workspace)
	if err != nil {
		return nil, err
	}
	abs = filepath.Clean(abs)
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("workspace is not a directory: %s", abs)
	}
	// Resolve symlinks on the root so later checks are consistent.
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	return &Root{Abs: abs}, nil
}

// Resolve returns an absolute path under the workspace, or an error if the
// path escapes (including via symlink when evalSymlinks is true for the final path).
func (r *Root) Resolve(relOrAbs string) (string, error) {
	if relOrAbs == "" {
		return r.Abs, nil
	}
	var candidate string
	if filepath.IsAbs(relOrAbs) {
		candidate = filepath.Clean(relOrAbs)
	} else {
		candidate = filepath.Clean(filepath.Join(r.Abs, relOrAbs))
	}
	// Ensure prefix boundary with separator (not /tmp/foo vs /tmp/foobar).
	if !isUnder(r.Abs, candidate) {
		return "", fmt.Errorf("path escapes workspace: %s", relOrAbs)
	}
	// If the path exists, resolve symlinks and re-check.
	if resolved, err := filepath.EvalSymlinks(candidate); err == nil {
		if !isUnder(r.Abs, resolved) {
			return "", fmt.Errorf("path escapes workspace via symlink: %s", relOrAbs)
		}
		return resolved, nil
	}
	return candidate, nil
}

// Rel returns a workspace-relative slash path for display.
func (r *Root) Rel(abs string) string {
	rel, err := filepath.Rel(r.Abs, abs)
	if err != nil {
		return abs
	}
	return filepath.ToSlash(rel)
}

func isUnder(root, path string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if root == path {
		return true
	}
	sep := string(os.PathSeparator)
	return strings.HasPrefix(path, root+sep)
}
