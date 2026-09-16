// Package versionfile maintains ~/.ywai/version.json so external tools — notably
// the opencode TUI logo plugin, which cannot import Go or reach the control
// server — can show the installed ywai version and whether an update exists.
//
// Channel matching reuses selfupdate.Offer so a beta install tracks the next
// beta (`ywai update --beta`) and a stable install tracks GitHub latest
// (`ywai update`). The control UI's GET /api/version handler uses the same
// Offer, so the logo and the settings page never disagree.
package versionfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/selfupdate"
)

// Info is the on-disk shape read by the logo plugin.
type Info struct {
	Installed       string `json:"installed"`
	Latest          string `json:"latest,omitempty"`
	LatestStable    string `json:"latestStable,omitempty"`
	LatestBeta      string `json:"latestBeta,omitempty"`
	Channel         string `json:"channel,omitempty"`
	UpdateCommand   string `json:"updateCommand,omitempty"`
	UpdateAvailable bool   `json:"updateAvailable"`
	StableNewer     bool   `json:"stableNewer,omitempty"`
	CheckedAt       int64  `json:"checkedAt"`     // unix seconds of the last stable check
	BetaCheckedAt   int64  `json:"betaCheckedAt"` // unix seconds of the last beta check (0 = never)
}

// Path is the version.json location under the ywai data dir.
func Path() string {
	return filepath.Join(config.DataDir(), "version.json")
}

// latestFn / latestBetaFn are the network calls, indirected for tests.
var latestFn = selfupdate.LatestVersion
var latestBetaFn = selfupdate.LatestPrereleaseVersion

// computeUpdate reports whether `latest` is a real upgrade for `installed`.
// Dev builds never flag; an older stable is not an update for a newer beta.
func computeUpdate(installed, latest string) bool {
	return selfupdate.Offer(installed, latest, latest).Available
}

func compose(installed, latestStable, latestBeta string, checkedAt, betaCheckedAt int64) Info {
	offer := selfupdate.Offer(installed, latestStable, latestBeta)
	return Info{
		Installed:       installed,
		Latest:          offer.Latest,
		LatestStable:    latestStable,
		LatestBeta:      latestBeta,
		Channel:         offer.Channel,
		UpdateCommand:   offer.Command,
		UpdateAvailable: offer.Available,
		StableNewer:     offer.StableNewer,
		CheckedAt:       checkedAt,
		BetaCheckedAt:   betaCheckedAt,
	}
}

// splitCached maps a previously written Info onto the two channel heads.
// Files written before LatestStable/LatestBeta existed stored one `latest`
// (always GitHub's stable); recover that so Touch does not drop the cache.
func splitCached(prev Info) (stable, beta string) {
	stable, beta = prev.LatestStable, prev.LatestBeta
	if stable == "" && prev.Latest != "" && !selfupdate.IsPrerelease(prev.Latest) {
		stable = prev.Latest
	}
	if beta == "" && prev.Latest != "" && selfupdate.IsPrerelease(prev.Latest) {
		beta = prev.Latest
	}
	return stable, beta
}

// Touch records the installed version without any network call, recomputing
// UpdateAvailable from the cached latest. Cheap enough to run on every command.
func Touch(installed string) error {
	prev, _ := load(Path())
	stable, beta := splitCached(prev)
	return save(Path(), compose(installed, stable, beta, prev.CheckedAt, prev.BetaCheckedAt))
}

func stale(unix int64, ttl time.Duration) bool {
	return unix == 0 || time.Since(time.Unix(unix, 0)) >= ttl
}

// Refresh records the installed version and re-checks GitHub for the latest
// stable and beta at most once per ttl (reusing the cached values otherwise).
// Network failures are non-fatal: the file is still written with the cache.
// A beta install whose file never recorded a beta check (legacy cache) fetches
// the prerelease head even if the stable check is still fresh.
func Refresh(installed string, ttl time.Duration) error {
	prev, _ := load(Path())
	stable, beta := splitCached(prev)
	checkedAt := prev.CheckedAt
	betaCheckedAt := prev.BetaCheckedAt
	now := time.Now().Unix()

	if stale(checkedAt, ttl) {
		if l, err := latestFn(); err == nil {
			stable = l
			checkedAt = now
		}
	}
	// Beta channel is only fetched for prerelease installs. Stamp
	// BetaCheckedAt even on failure so a missing GitHub prerelease does
	// not retry on every command.
	if selfupdate.IsPrerelease(installed) && (betaCheckedAt == 0 || stale(betaCheckedAt, ttl)) {
		if l, err := latestBetaFn(); err == nil {
			beta = l
		}
		betaCheckedAt = now
	}
	return save(Path(), compose(installed, stable, beta, checkedAt, betaCheckedAt))
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
