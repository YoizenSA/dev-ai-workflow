package evals

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// The store keeps one JSON file per run plus a small index, so a control-server
// restart preserves runs that each cost minutes of real model time. Layout:
//
//	<dir>/<id>.json   one full Run per file
//	<dir>/index.json  {"runs": [RunHeader...], "baselines": {taskId: runId}}
//
// The index is a cache, not the truth: when it is missing or corrupt it is
// rebuilt by scanning the directory. Baseline pointers live only in the index,
// so they are the one thing a rebuild cannot recover.

const (
	indexFile      = "index.json"
	legacyRunsFile = "eval-runs.json" // pre-store layout: one array of Run
)

// defaultRunRetention caps how many runs the store keeps. Runs are small
// (attempt responses are truncated before they reach the store) but each is a
// real measurement, so the cap is generous and configurable.
const defaultRunRetention = 500

// evalRunsKeepEnv overrides defaultRunRetention.
const evalRunsKeepEnv = "EVAL_RUNS_KEEP"

// ErrRunNotFound is returned by GetRun for an unknown or purged run ID.
var ErrRunNotFound = errors.New("evals: run not found")

// RunHeader is the per-run summary kept in index.json so listing never has to
// read every run file.
type RunHeader struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"taskId"`
	TaskName  string    `json:"taskName"`
	Agent     string    `json:"agent"`
	Provider  string    `json:"provider"`
	Models    []string  `json:"models"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"startedAt"`
	EndedAt   time.Time `json:"endedAt,omitempty"`
}

// storeIndex is the on-disk shape of index.json.
type storeIndex struct {
	Runs      []RunHeader       `json:"runs"`
	Baselines map[string]string `json:"baselines"`
}

// Store persists benchmark runs under a directory. All state is guarded by mu:
// benchmark runs are upserted from a detached goroutine while HTTP handlers
// list and read concurrently.
type Store struct {
	mu    sync.RWMutex
	dir   string
	keep  int
	index storeIndex
}

// OpenStore opens (creating if needed) the run store rooted at dir. It runs the
// one-time legacy migration, loads or rebuilds the index, and applies
// retention, so the returned Store is immediately consistent.
func OpenStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("evals: create run dir: %w", err)
	}
	s := &Store{dir: dir, keep: retentionLimit()}
	// OpenStore runs before the store is shared, so the helpers below are
	// called without holding s.mu.
	if err := s.migrateLegacy(); err != nil {
		return nil, err
	}
	if err := s.loadIndex(); err != nil {
		return nil, err
	}
	if s.enforceRetention() {
		if err := s.persistIndex(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// ListRuns returns every stored run, newest first. A run whose file vanished
// after the index was built is skipped; an unreadable one is an error, because
// silently dropping runs would quietly skew the aggregate comparisons.
func (s *Store) ListRuns() ([]Run, error) {
	s.mu.RLock()
	headers := make([]RunHeader, len(s.index.Runs))
	copy(headers, s.index.Runs)
	s.mu.RUnlock()

	// Files are read outside the lock: upserts rewrite files atomically, so a
	// concurrent write only ever means a header whose file is gone by read
	// time, which the IsNotExist branch handles.
	runs := make([]Run, 0, len(headers))
	for _, h := range headers {
		run, err := readRunFile(s.runPath(h.ID))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("evals: read run %s: %w", h.ID, err)
		}
		runs = append(runs, run)
	}
	return runs, nil
}

// GetRun returns one full run, or ErrRunNotFound when no such run exists.
func (s *Store) GetRun(id string) (Run, error) {
	if !validRunID(id) {
		return Run{}, ErrRunNotFound
	}
	run, err := readRunFile(s.runPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return Run{}, ErrRunNotFound
		}
		return Run{}, fmt.Errorf("evals: read run %s: %w", id, err)
	}
	return run, nil
}

// UpsertRun persists run, replacing any earlier run with the same ID, then
// enforces retention. The index is written after the run file so a crash can
// leave an orphan file (harmless, re-purged later) but never an index entry
// without data.
func (s *Store) UpsertRun(r Run) error {
	if !validRunID(r.ID) {
		return fmt.Errorf("evals: invalid run id %q", r.ID)
	}
	if err := s.writeRunFile(r); err != nil {
		return fmt.Errorf("evals: persist run %s: %w", r.ID, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	replaced := false
	for i := range s.index.Runs {
		if s.index.Runs[i].ID == r.ID {
			s.index.Runs[i] = runHeaderOf(r)
			replaced = true
			break
		}
	}
	if !replaced {
		s.index.Runs = append(s.index.Runs, runHeaderOf(r))
	}
	sortRunHeaders(s.index.Runs)
	s.enforceRetention()
	return s.persistIndex()
}

// GetBaseline returns the run ID flagged as best for taskID, or "" when none is.
func (s *Store) GetBaseline(taskID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.index.Baselines[taskID], nil
}

// SetBaseline flags runID as the baseline for taskID.
func (s *Store) SetBaseline(taskID, runID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.index.Baselines == nil {
		s.index.Baselines = map[string]string{}
	}
	s.index.Baselines[taskID] = runID
	return s.persistIndex()
}

// ClearBaseline drops the baseline for taskID. Clearing an unknown task is a
// no-op, not an error.
func (s *Store) ClearBaseline(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.index.Baselines[taskID]; !ok {
		return nil
	}
	delete(s.index.Baselines, taskID)
	return s.persistIndex()
}

// migrateLegacy splits the pre-store single-file layout into per-run files and
// renames the original to eval-runs.json.imported. The original is archived,
// never deleted: it is the only copy of runs whose per-file write failed. An
// unreadable legacy file is left untouched so a later open can retry.
func (s *Store) migrateLegacy() error {
	legacyPath := filepath.Join(s.dir, legacyRunsFile)
	data, err := os.ReadFile(legacyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("evals: read legacy runs file: %w", err)
	}
	var runs []Run
	if err := json.Unmarshal(data, &runs); err != nil {
		return nil
	}
	for _, run := range runs {
		if !validRunID(run.ID) {
			continue // malformed entry; the archive keeps the original
		}
		if err := s.writeRunFile(run); err != nil {
			return fmt.Errorf("evals: migrate run %s: %w", run.ID, err)
		}
	}
	if err := os.Rename(legacyPath, legacyPath+".imported"); err != nil {
		return fmt.Errorf("evals: archive legacy runs file: %w", err)
	}
	return nil
}

// loadIndex reads index.json, falling back to a directory scan when it is
// missing or corrupt. Caller must run it before the store is shared.
func (s *Store) loadIndex() error {
	data, err := os.ReadFile(filepath.Join(s.dir, indexFile))
	switch {
	case err == nil:
		var idx storeIndex
		if json.Unmarshal(data, &idx) == nil {
			if idx.Runs == nil {
				idx.Runs = []RunHeader{}
			}
			if idx.Baselines == nil {
				idx.Baselines = map[string]string{}
			}
			s.index = idx
			return nil
		}
		// Corrupt index: fall through to the rebuild.
	case os.IsNotExist(err):
		// First open (or a deleted index): rebuild.
	default:
		return fmt.Errorf("evals: read run index: %w", err)
	}
	return s.rebuildIndex()
}

// rebuildIndex reconstructs the index by scanning the run directory. It runs
// when index.json is missing or corrupt, so recovered baselines start empty.
func (s *Store) rebuildIndex() error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return fmt.Errorf("evals: scan run dir: %w", err)
	}
	headers := []RunHeader{}
	for _, e := range entries {
		name := e.Name()
		switch {
		case e.IsDir(), !strings.HasSuffix(name, ".json"):
			continue
		case name == indexFile, name == legacyRunsFile, name == legacyRunsFile+".imported":
			continue
		case strings.HasSuffix(name, ".tmp"): // interrupted atomic write
			continue
		}
		run, err := readRunFile(filepath.Join(s.dir, name))
		if err != nil || !validRunID(run.ID) {
			continue // not a run file, or not a readable one
		}
		headers = append(headers, runHeaderOf(run))
	}
	sortRunHeaders(headers)
	s.index = storeIndex{Runs: headers, Baselines: map[string]string{}}
	return s.persistIndex()
}

// enforceRetention drops the oldest runs past the cap and reports whether
// anything was purged. Caller holds s.mu (or is OpenStore before sharing).
// Run-file deletion is best effort: a file left behind by a failed delete is
// re-discovered by an index rebuild and purged again on a later pass.
func (s *Store) enforceRetention() bool {
	purged := false
	for len(s.index.Runs) > s.keep {
		victim := s.index.Runs[len(s.index.Runs)-1] // sorted newest first
		s.index.Runs = s.index.Runs[:len(s.index.Runs)-1]
		_ = os.Remove(s.runPath(victim.ID))
		purged = true
	}
	return purged
}

// persistIndex writes index.json. Caller holds s.mu (or is OpenStore).
func (s *Store) persistIndex() error {
	data, err := json.MarshalIndent(s.index, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(s.dir, indexFile), data)
}

func (s *Store) runPath(id string) string {
	return filepath.Join(s.dir, id+".json")
}

func (s *Store) writeRunFile(run Run) error {
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(s.runPath(run.ID), data)
}

func runHeaderOf(r Run) RunHeader {
	return RunHeader{
		ID:        r.ID,
		TaskID:    r.TaskID,
		TaskName:  r.TaskName,
		Agent:     r.Agent,
		Provider:  r.Provider,
		Models:    r.Models,
		Status:    r.Status,
		StartedAt: r.StartedAt,
		EndedAt:   r.EndedAt,
	}
}

// sortRunHeaders orders headers newest first, so lists and retention (which
// evicts from the tail) agree on what "oldest" means.
func sortRunHeaders(headers []RunHeader) {
	sort.SliceStable(headers, func(i, j int) bool {
		return headers[i].StartedAt.After(headers[j].StartedAt)
	})
}

// validRunID rejects IDs that could escape the run directory: the ID becomes a
// file name verbatim, and it arrives from user-visible JSON.
func validRunID(id string) bool {
	if id == "" || id == "." || id == ".." {
		return false
	}
	return !strings.ContainsAny(id, `/\`)
}

func readRunFile(path string) (Run, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Run{}, err
	}
	var run Run
	if err := json.Unmarshal(data, &run); err != nil {
		return Run{}, fmt.Errorf("parse %s: %w", filepath.Base(path), err)
	}
	return run, nil
}

// writeFileAtomic writes via a temp file plus rename, so a crash mid-write
// leaves the previous file intact instead of a truncated one. os.Rename
// replaces an existing destination on every platform Go supports, Windows
// included.
func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// retentionLimit is the run cap: 500 by default, overridable with
// EVAL_RUNS_KEEP. A non-positive or unreadable value falls back to the default
// rather than letting a typo wipe history.
func retentionLimit() int {
	if v := strings.TrimSpace(os.Getenv(evalRunsKeepEnv)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return defaultRunRetention
}
