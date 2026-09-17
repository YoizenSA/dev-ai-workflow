package plugins

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// JevKeyFileName is the file the jev-gate plugin reads its key from. The
// plugin looks in the project first, then ~/.config/opencode, then ~/.ywai;
// ywai writes the ~/.ywai one because that is the directory ywai owns.
const JevKeyFileName = "jev-gate.json"

// JevKeyPath is where ywai writes the key.
func JevKeyPath() string {
	return filepath.Join(config.DataDir(), JevKeyFileName)
}

// EnsureJevKey writes the key file when a key is available and none is
// stored yet, and reports what the user still has to do.
//
// The environment is the source on purpose: the plugin runs inside a
// long-lived OpenCode server that does NOT inherit the shell that starts it,
// so a key exported in a terminal never reaches the plugin. Copying it into a
// file at install time is what makes it usable at all.
//
// An existing file is never overwritten: re-running install must not clobber
// a key the user edited by hand.
func EnsureJevKey(key string) (path string, wrote bool, err error) {
	path = JevKeyPath()
	if _, statErr := os.Stat(path); statErr == nil {
		return path, false, nil
	}
	if key == "" {
		return path, false, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return path, false, fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	body, err := json.MarshalIndent(map[string]string{"apiKey": key}, "", "  ")
	if err != nil {
		return path, false, err
	}
	// 0600: this is a credential, not config.
	if err := os.WriteFile(path, append(body, '\n'), 0o600); err != nil {
		return path, false, fmt.Errorf("write %s: %w", path, err)
	}
	return path, true, nil
}

// JevKeyStatus is the line install prints about the key. It never prints the
// key itself.
func JevKeyStatus(path string, wrote bool) string {
	if wrote {
		return fmt.Sprintf("  jev-gate: API key written to %s (from TYPESAFE_API_KEY)", path)
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Sprintf("  jev-gate: using the API key already at %s", path)
	}
	return fmt.Sprintf(
		"  jev-gate: NO API KEY. The tools will refuse to run until you write\n"+
			"            {\"apiKey\": \"...\"} to %s\n"+
			"            (or export TYPESAFE_API_KEY and re-run install).", path)
}
