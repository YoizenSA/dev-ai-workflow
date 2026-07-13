package fastfs

import (
	"bytes"
	"io/fs"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"sync/atomic"
)

// SearchOptions controls content search.
type SearchOptions struct {
	Pattern     string // Go regexp
	Glob        string // optional file filter (same as Find pattern)
	MaxMatches  int    // default 50
	MaxFiles    int    // default 2000 scanned files with content
	CaseInsensitive bool
}

// SearchMatch is one line hit.
type SearchMatch struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Preview string `json:"preview"`
}

// SearchResult is the structured search response.
type SearchResult struct {
	Matches    []SearchMatch `json:"matches"`
	FilesScanned int         `json:"filesScanned"`
	Truncated  bool          `json:"truncated"`
	CacheHits  int64         `json:"cacheHits"`
	CacheMisses int64        `json:"cacheMisses"`
}

// Search runs a parallel regex content search through the mtime cache.
func (s *Service) Search(opts SearchOptions) (*SearchResult, error) {
	if opts.MaxMatches <= 0 {
		opts.MaxMatches = 50
	}
	if opts.MaxFiles <= 0 {
		opts.MaxFiles = 2000
	}
	pat := opts.Pattern
	if opts.CaseInsensitive {
		pat = "(?i)" + pat
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		return nil, err
	}

	type job struct {
		abs string
		rel string
	}
	jobs := make(chan job, 64)
	var (
		mu       sync.Mutex
		matches  []SearchMatch
		scanned  int32
		trunc    int32
		wg       sync.WaitGroup
	)

	workers := runtime.NumCPU()
	if workers < 2 {
		workers = 2
	}
	if workers > 8 {
		workers = 8
	}

	hitsBefore, missesBefore, _, _ := s.cache.Stats()

	worker := func() {
		defer wg.Done()
		for j := range jobs {
			if atomic.LoadInt32(&trunc) == 1 {
				continue
			}
			data, err := s.cache.Get(j.abs)
			if err != nil {
				continue
			}
			atomic.AddInt32(&scanned, 1)
			if isBinary(data) {
				continue
			}
			lines := bytes.Split(data, []byte{'\n'})
			for i, line := range lines {
				loc := re.FindIndex(line)
				if loc == nil {
					continue
				}
				preview := string(line)
				if len(preview) > 200 {
					preview = preview[:200] + "…"
				}
				mu.Lock()
				if len(matches) >= opts.MaxMatches {
					atomic.StoreInt32(&trunc, 1)
					mu.Unlock()
					return
				}
				matches = append(matches, SearchMatch{
					Path:    j.rel,
					Line:    i + 1,
					Column:  loc[0] + 1,
					Preview: preview,
				})
				if len(matches) >= opts.MaxMatches {
					atomic.StoreInt32(&trunc, 1)
				}
				mu.Unlock()
			}
		}
	}

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go worker()
	}

	filesEnqueued := 0
	_ = filepath.WalkDir(s.root.Abs, func(path string, d fs.DirEntry, err error) error {
		if err != nil || atomic.LoadInt32(&trunc) == 1 {
			if atomic.LoadInt32(&trunc) == 1 {
				return fs.SkipAll
			}
			return nil
		}
		rel := s.root.Rel(path)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			if s.ignore.SkipDir(d.Name()) || s.ignore.SkipFile(rel+"/") {
				return fs.SkipDir
			}
			return nil
		}
		if s.ignore.SkipFile(rel) {
			return nil
		}
		if opts.Glob != "" && !pathMatches(opts.Glob, rel, d.Name()) {
			return nil
		}
		if filesEnqueued >= opts.MaxFiles {
			atomic.StoreInt32(&trunc, 1)
			return fs.SkipAll
		}
		filesEnqueued++
		jobs <- job{abs: path, rel: rel}
		return nil
	})
	close(jobs)
	wg.Wait()

	hitsAfter, missesAfter, _, _ := s.cache.Stats()
	return &SearchResult{
		Matches:      matches,
		FilesScanned: int(atomic.LoadInt32(&scanned)),
		Truncated:    atomic.LoadInt32(&trunc) == 1,
		CacheHits:    hitsAfter - hitsBefore,
		CacheMisses:  missesAfter - missesBefore,
	}, nil
}

func isBinary(data []byte) bool {
	// Null byte in first 8KiB ⇒ binary.
	n := len(data)
	if n > 8192 {
		n = 8192
	}
	return bytes.IndexByte(data[:n], 0) >= 0
}
