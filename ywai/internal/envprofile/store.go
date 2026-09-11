package envprofile

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// profilesRootOverride pins the profiles root in tests (config.DataDir has
// no env override, so hermetic tests need this seam).
var profilesRootOverride string

// SetProfilesRootForTest overrides ProfilesDir for the calling test.
func SetProfilesRootForTest(dir string) {
	profilesRootOverride = dir
}

// ProfilesDir is ~/.ywai/profiles (or the test override).
func ProfilesDir() string {
	if profilesRootOverride != "" {
		return profilesRootOverride
	}
	return filepath.Join(config.DataDir(), "profiles")
}

func profileDir(name string) string {
	return filepath.Join(ProfilesDir(), name)
}

// ManifestPath returns the manifest file for a profile name.
func ManifestPath(name string) string {
	return filepath.Join(profileDir(name), "manifest.json")
}

var validProfileName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// reservedProfileNames are taken by ywai root commands (plus env itself).
// Dynamic shortcuts (ywai <name>) must never shadow them.
var reservedProfileNames = map[string]bool{
	"install": true, "update": true, "agents": true, "skills": true,
	"doctor": true, "config": true, "status": true, "groups": true,
	"tokenbank": true, "mcp": true, "profile": true, "env": true,
	"serve": true, "stop": true, "ui": true, "eval": true,
	"session": true, "advisor": true, "uninstall": true, "clean": true,
	"completion": true, "help": true, "version": true,
}

// ValidateName rejects names that are not DNS-safe or collide with commands.
// The regex excludes slashes, dots and separators, so a validated name can
// never escape ProfilesDir when joined.
func ValidateName(name string) error {
	if !validProfileName.MatchString(name) {
		return fmt.Errorf("invalid profile name %q: use lowercase letters, digits and hyphens (max 32 chars)", name)
	}
	if reservedProfileNames[name] {
		return fmt.Errorf("profile name %q is reserved by a ywai command", name)
	}
	return nil
}

// Exists reports whether a valid manifest exists for name.
func Exists(name string) bool {
	st, err := os.Stat(ManifestPath(name))
	return err == nil && !st.IsDir()
}

// Get loads one profile by name.
func Get(name string) (Profile, error) {
	if err := ValidateName(name); err != nil {
		return Profile{}, err
	}
	return LoadManifest(profileDir(name))
}

// List returns all profiles with a readable manifest, sorted by name.
// Directories without a manifest are skipped (stale or foreign data).
func List() ([]Profile, error) {
	entries, err := os.ReadDir(ProfilesDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list profiles: %w", err)
	}
	var out []Profile
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p, err := LoadManifest(filepath.Join(ProfilesDir(), e.Name()))
		if err != nil {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// subDirs is the full per-profile layout (D3).
var subDirs = []string{"config", "data", "state", "cache", "tmp", "run", "evals"}

// configSubDirs are pre-created inside <profile>/config/opencode so the first
// scoped write (agent, command, skill, config file) never 500s on a missing
// parent: a fresh environment starts with an empty config tree, unlike the
// global install where the installer always created these.
var configSubDirs = []string{"agents", "commands", "skills"}

// Create validates, allocates a port, creates the layout and writes the manifest.
func Create(name, preset string) (Profile, error) {
	if err := ValidateName(name); err != nil {
		return Profile{}, err
	}
	if Exists(name) {
		return Profile{}, fmt.Errorf("profile %q already exists", name)
	}
	if preset == "" {
		preset = "dev"
	}
	port, err := allocatePort()
	if err != nil {
		return Profile{}, err
	}
	p := Profile{
		Name:      name,
		Preset:    preset,
		Port:      port,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	for _, d := range subDirs {
		if err := os.MkdirAll(filepath.Join(profileDir(name), d), 0o700); err != nil {
			return Profile{}, fmt.Errorf("create profile dir %s: %w", d, err)
		}
	}
	cfgRoot := filepath.Join(profileDir(name), "config", "opencode")
	for _, d := range configSubDirs {
		if err := os.MkdirAll(filepath.Join(cfgRoot, d), 0o755); err != nil {
			return Profile{}, fmt.Errorf("create profile config dir %s: %w", d, err)
		}
	}
	if err := p.SaveManifest(profileDir(name)); err != nil {
		return Profile{}, err
	}
	return p, nil
}

// allocatePort returns the first free port in 5800-5899 not claimed by any
// stored manifest. Ports persist in the manifest at create time.
func allocatePort() (int, error) {
	claimed := map[int]bool{}
	if profiles, err := List(); err == nil {
		for _, p := range profiles {
			if p.Port != 0 {
				claimed[p.Port] = true
			}
		}
	}
	for port := 5800; port <= 5899; port++ {
		if !claimed[port] {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no free profile port in 5800-5899")
}

// Delete removes exactly the profile directory, retrying locked files:
// a just-stopped server can hold the database/log briefly on Windows. The
// name was validated, so the joined path cannot escape ProfilesDir.
// Callers stop the profile service first.
func Delete(name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	if !Exists(name) {
		return fmt.Errorf("profile %q does not exist", name)
	}
	dir := profileDir(name)
	var err error
	for i := 0; i < 4; i++ {
		if err = os.RemoveAll(dir); err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("remove profile %q: %w (files still locked — retry in a few seconds)", name, err)
}
