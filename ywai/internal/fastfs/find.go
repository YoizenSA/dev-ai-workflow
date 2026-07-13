package fastfs

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// FindOptions controls path discovery.
type FindOptions struct {
	// Pattern is matched against the slash-relative path and/or basename
	// (e.g. "*.go", "**/*.ts", "internal/**").
	Pattern string
	// Max results (default 500).
	Max int
}

// FindResult is one path hit.
type FindResult struct {
	Path string `json:"path"` // workspace-relative slash path
}

// Find walks the workspace and returns matching paths (gitignore-aware).
func (s *Service) Find(opts FindOptions) ([]FindResult, error) {
	if opts.Max <= 0 {
		opts.Max = 500
	}
	if opts.Pattern == "" {
		opts.Pattern = "*"
	}
	var out []FindResult
	err := filepath.WalkDir(s.root.Abs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable
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
		if !pathMatches(opts.Pattern, rel, d.Name()) {
			return nil
		}
		out = append(out, FindResult{Path: rel})
		if len(out) >= opts.Max {
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func pathMatches(pattern, relSlash, base string) bool {
	pattern = filepath.ToSlash(pattern)
	if strings.HasPrefix(pattern, "**/") {
		sub := pattern[3:]
		if ok, _ := filepath.Match(sub, base); ok {
			return true
		}
		if ok, _ := filepath.Match(sub, relSlash); ok {
			return true
		}
		return strings.HasSuffix(relSlash, "/"+sub) || relSlash == sub
	}
	if strings.Contains(pattern, "/") {
		ok, _ := filepath.Match(pattern, relSlash)
		return ok
	}
	ok, _ := filepath.Match(pattern, base)
	return ok
}
