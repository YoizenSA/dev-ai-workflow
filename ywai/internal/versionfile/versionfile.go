// Package versionfile maintains ~/.ywai/version.json so external tools — notably
// the opencode TUI logo plugin, which cannot import Go or reach the control
// server — can show the installed ywai version and whether an update exists.
//
// The "latest" check reuses selfupdate.LatestVersion and mirrors the exact
// normalization used by the control UI's GET /api/version handler, so the logo
// and the settings page never disagree.
package versionfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/selfupdate"
)

// Info is the on-disk shape read by the logo plugin.
type Info struct {
	Installed       string `json:"installed"`
	Latest          string `json:"latest,omitempty"`
	UpdateAvailable bool   `json:"updateAvailable"`
	CheckedAt       int64  `json:"checkedAt"` // unix seconds of the last network check
}

// Path is the version.json location under the ywai data dir.
func Path() string {
	return filepath.Join(config.DataDir(), "version.json")
}

// latestFn is the network call, indirected for tests.
var latestFn = selfupdate.LatestVersion

// computeUpdate mirrors control.versionHandler: trim GitHub's leading "v" and
// never flag updates for dev builds.
func computeUpdate(installed, latest string) bool {
	if latest == "" {
		return false
	}
	norm := strings.TrimPrefix(latest, "v")
	return installed != norm && !strings.HasPrefix(installed, "dev")
}

// Touch records the installed version without any network call, recomputing
// UpdateAvailable from the cached latest. Cheap enough to run on every command.
func Touch(installed string) error {
	prev, _ := load(Path())
	info := Info{
		Installed:       installed,
		Latest:          prev.Latest,
		UpdateAvailable: computeUpdate(installed, prev.Latest),
		CheckedAt:       prev.CheckedAt,
	}
	return save(Path(), info)
}

// Refresh records the installed version and re-checks GitHub for the latest
// release at most once per ttl (reusing the cached value otherwise). Network
// failures are non-fatal: the file is still written with the cached latest.
func Refresh(installed string, ttl time.Duration) error {
	prev, _ := load(Path())

	latest := prev.Latest
	checkedAt := prev.CheckedAt
	if prev.CheckedAt == 0 || time.Since(time.Unix(prev.CheckedAt, 0)) >= ttl {
		if l, err := latestFn(); err == nil {
			latest = l
			checkedAt = time.Now().Unix()
		}
	}

	info := Info{
		Installed:       installed,
		Latest:          latest,
		UpdateAvailable: computeUpdate(installed, latest),
		CheckedAt:       checkedAt,
	}
	return save(Path(), info)
}

func load(path string) (Info, error) {
	var info Info
	data, err := os.ReadFile(path)
	if err != nil {
		return info, err
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return Info{}, err
	}
	return info, nil
}

func save(path string, info Info) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal version info: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
