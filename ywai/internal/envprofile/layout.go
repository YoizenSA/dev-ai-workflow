package envprofile

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// Dirs returns the resolved directories of a profile (all under
// ProfilesDir()/p.Name). Keys: config, data, state, cache, tmp, run, evals.
func Dirs(p Profile) map[string]string {
	root := profileDir(p.Name)
	out := make(map[string]string, len(subDirs)+1)
	out["root"] = root
	for _, d := range subDirs {
		out[d] = filepath.Join(root, d)
	}
	return out
}

// Ensure creates the full profile layout (0700) plus the config subtree so
// scoped writers never hit a missing parent on a fresh environment.
func Ensure(p Profile) error {
	for _, d := range Dirs(p) {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return fmt.Errorf("ensure profile dir %s: %w", d, err)
		}
	}
	cfgRoot := filepath.Join(profileDir(p.Name), "config", "opencode")
	for _, d := range configSubDirs {
		if err := os.MkdirAll(filepath.Join(cfgRoot, d), 0o755); err != nil {
			return fmt.Errorf("ensure profile config dir %s: %w", d, err)
		}
	}
	return EnsureServicePort(p)
}

// Env returns the environment delta that isolates one profile. Applied on
// top of os.Environ() by the runner (see sandbox.go). opencode appends
// "/opencode" to each XDG value, so the real locations become
// <p>/config/opencode, <p>/data/opencode and so on.
func Env(p Profile) map[string]string {
	dirs := Dirs(p)
	return map[string]string{
		"XDG_CONFIG_HOME":     dirs["config"],
		"XDG_DATA_HOME":       dirs["data"],
		"XDG_STATE_HOME":      dirs["state"],
		"XDG_CACHE_HOME":      dirs["cache"],
		"TMPDIR":              dirs["tmp"],
		"TEMP":                dirs["tmp"], // Windows: os.TempDir ignores TMPDIR
		"TMP":                 dirs["tmp"],
		"OPENCODE_CONFIG_DIR": filepath.Join(dirs["config"], "opencode"),
		"OPENCODE_DB":         filepath.Join(dirs["data"], "opencode", "opencode.db"),
		"OPENCODE_URL":        "http://127.0.0.1:" + strconv.Itoa(p.Port),
		"YWAI_PROFILE":        p.Name,
	}
}
